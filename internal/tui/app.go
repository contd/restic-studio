package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/restic-studio/restic-studio/internal/app"
	"github.com/restic-studio/restic-studio/internal/config"
	"github.com/restic-studio/restic-studio/internal/runner"
)

type screen uint8

const (
	dashboardScreen screen = iota
	locationsScreen
	backendsScreen
	backendOpsScreen
	snapshotScreen
	promptScreen
	confirmScreen
	runningScreen
	outputScreen
)

type menuItem struct {
	id, title, description string
}

func (i menuItem) FilterValue() string { return i.title + " " + i.description }
func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.description }

type promptField struct {
	label, value string
	secret       bool
	validate     func(string) error
}

type progressMessage string

type snapshot struct {
	ID       string    `json:"id"`
	ShortID  string    `json:"short_id"`
	Time     time.Time `json:"time"`
	Hostname string    `json:"hostname"`
	Paths    []string  `json:"paths"`
	Tags     []string  `json:"tags"`
}

type Model struct {
	config           *config.Config
	executor         *runner.Executor
	parentCtx        context.Context
	screen           screen
	menu             list.Model
	width            int
	height           int
	location         string
	backend          string
	operation        string
	cleanupPath      string
	selectedSnapshot string
	fields           []promptField
	fieldIndex       int
	input            textinput.Model
	pending          app.Command
	pendingName      string
	cancel           context.CancelFunc
	spinner          spinner.Model
	output           string
	outputErr        string
	outputStatus     string
	progress         func(string)
	status           string
	snapshots        []snapshot
	dependencies     []string
}

func New(cfg *config.Config, executor *runner.Executor) Model {
	if executor == nil {
		executor = runner.New()
	}
	model := Model{
		config:    cfg,
		executor:  executor,
		parentCtx: context.Background(),
		screen:    dashboardScreen,
		spinner:   spinner.New(),
		input:     textinput.New(),
	}
	model.spinner.Spinner = spinner.Dot
	for _, binary := range []string{"restic", "autorestic"} {
		if err := executor.Check(binary); err != nil {
			model.dependencies = append(model.dependencies, err.Error())
		}
	}
	model.setMenu(model.dashboardItems(), "Restic Studio")
	return model
}

func (m Model) Init() tea.Cmd { return m.spinner.Tick }

func (m *Model) setMenu(items []menuItem, title string) {
	menuItems := make([]list.Item, 0, len(items))
	for _, item := range items {
		menuItems = append(menuItems, item)
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	menu := list.New(menuItems, delegate, max(m.width-8, 50), max(m.height-12, 12))
	menu.Title = title
	menu.SetShowStatusBar(false)
	menu.SetFilteringEnabled(false)
	menu.SetShowHelp(false)
	menu.SetShowPagination(false)
	m.menu = menu
}

func (m Model) dashboardItems() []menuItem {
	return []menuItem{
		{id: "locations", title: "Locations", description: fmt.Sprintf("%d configured backup locations", len(m.config.Locations))},
		{id: "backends", title: "Backends", description: fmt.Sprintf("%d configured Restic repositories", len(m.config.Backends))},
		{id: "reload", title: "Reload configuration", description: "Re-read and validate the active YAML file"},
		{id: "quit", title: "Quit", description: "Exit Restic Studio"},
	}
}

func (m *Model) openLocations() {
	m.screen = locationsScreen
	names := m.config.LocationNames()
	sort.Strings(names)
	items := make([]menuItem, 0, len(names))
	for _, name := range names {
		location := m.config.Locations[name]
		detail := strings.Join(location.From, ", ")
		if len(location.To) > 0 {
			detail += " → " + strings.Join(location.To, ", ")
		} else {
			detail += " → default backends"
		}
		items = append(items, menuItem{id: name, title: name, description: detail})
	}
	m.setMenu(items, "Backup locations")
}

func (m *Model) openBackends() {
	m.screen = backendsScreen
	names := m.config.BackendNames()
	sort.Strings(names)
	items := make([]menuItem, 0, len(names))
	for _, name := range names {
		backend := m.config.Backends[name]
		repository := backend.Path
		if backend.Type != "" && backend.Type != "local" && !strings.Contains(repository, ":") {
			repository = backend.Type + ":" + repository
		}
		items = append(items, menuItem{id: name, title: name, description: repository})
	}
	m.setMenu(items, "Restic backends")
}

func (m *Model) openBackendOperations() {
	m.screen = backendOpsScreen
	backend := m.config.Backends[m.backend]
	items := []menuItem{
		{id: "check", title: "Check repository", description: "Verify repository consistency"},
		{id: "snapshots", title: "Browse snapshots", description: "Show available snapshots and metadata"},
		{id: "stats", title: "Statistics", description: "Display repository and snapshot statistics"},
		{id: "list", title: "List repository objects", description: "List blobs, packs, snapshots, index, keys, or locks"},
		{id: "ls", title: "Browse snapshot files", description: "List files and directories in a snapshot"},
		{id: "restore", title: "Restore snapshot", description: "Restore data to a destination with confirmation"},
		{id: "key-list", title: "List repository keys", description: "Show key IDs without changing the repository"},
		{id: "key-add", title: "Add repository key", description: "Create a new key (confirmation required)"},
		{id: "key-passwd", title: "Change key password", description: "Update a key password (confirmation required)"},
		{id: "key-remove", title: "Remove repository key", description: "Remove a key ID (confirmation required)"},
		{id: "back", title: "Back", description: "Return to backend selection"},
	}
	if strings.TrimSpace(backend.Path) == "" {
		m.status = "This backend has no repository path; edit the configuration before using it."
	}
	m.setMenu(items, "Repository tools · "+m.backend)
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.screen <= snapshotScreen {
			m.menu.SetSize(max(msg.Width-8, 50), max(msg.Height-12, 12))
		}
	case progressMessage:
		if m.screen == runningScreen {
			m.output = appendProgress(m.output, string(msg))
		}
	case operationDone:
		m.cancel = nil
		m.cleanupPath = ""
		m.output = msg.result.Output
		m.outputStatus = fmt.Sprintf("Exit code %d · %s", msg.result.ExitCode, msg.result.Duration.Round(time.Millisecond))
		m.outputErr = ""
		if msg.result.Err != nil {
			m.outputErr = msg.result.Err.Error()
		}
		if msg.name == "Browse snapshots" && msg.result.Err == nil {
			if err := json.Unmarshal([]byte(msg.result.Output), &m.snapshots); err == nil {
				m.screen = snapshotScreen
				items := make([]menuItem, 0, len(m.snapshots))
				for index, item := range m.snapshots {
					id := item.ShortID
					if id == "" {
						id = item.ID
					}
					when := item.Time.Format("2006-01-02 15:04")
					description := strings.Join(item.Paths, ", ")
					items = append(items, menuItem{id: fmt.Sprint(index), title: id + " · " + item.Hostname + " · " + when, description: description})
				}
				if len(items) == 0 {
					m.output = "No snapshots found in this repository."
					m.screen = outputScreen
				} else {
					m.setMenu(items, "Snapshots · "+m.backend+" · r restores selected")
				}
				return m, nil
			}
		}
		m.screen = outputScreen
		return m, nil
	case spinner.TickMsg:
		if m.screen == runningScreen {
			var command tea.Cmd
			m.spinner, command = m.spinner.Update(msg)
			return m, command
		}
	case tea.KeyMsg:
		if m.screen == runningScreen {
			if msg.String() == "ctrl+c" || msg.String() == "esc" {
				if m.cancel != nil {
					m.cancel()
					m.status = "Cancellation requested…"
				}
			}
			return m, nil
		}
		if msg.String() == "ctrl+c" {
			if m.cleanupPath != "" {
				_ = os.Remove(m.cleanupPath)
				m.cleanupPath = ""
			}
			return m, tea.Quit
		}
		switch m.screen {
		case dashboardScreen:
			switch msg.String() {
			case "q", "esc":
				return m, tea.Quit
			case "enter":
				if selected, ok := m.selectedItem(); ok {
					switch selected.id {
					case "locations":
						m.openLocations()
					case "backends":
						m.openBackends()
					case "reload":
						if err := m.reloadConfig(); err != nil {
							m.outputErr = err.Error()
						} else {
							m.outputErr = ""
							m.status = "Configuration reloaded."
						}
						m.setMenu(m.dashboardItems(), "Restic Studio")
					case "quit":
						return m, tea.Quit
					}
				}
				return m, nil
			}
		case locationsScreen:
			switch msg.String() {
			case "esc", "backspace":
				m.screen = dashboardScreen
				m.setMenu(m.dashboardItems(), "Restic Studio")
				return m, nil
			case "enter":
				if selected, ok := m.selectedItem(); ok {
					m.location = selected.id
					command, err := app.BackupCommand(m.config, m.location)
					if err != nil {
						m.outputErr = err.Error()
						m.screen = outputScreen
						return m, nil
					}
					m.prepareOperation("Backup location "+m.location, command, true)
				}
				return m, nil
			}
		case backendsScreen:
			switch msg.String() {
			case "esc", "backspace":
				m.screen = dashboardScreen
				m.setMenu(m.dashboardItems(), "Restic Studio")
				return m, nil
			case "enter":
				if selected, ok := m.selectedItem(); ok {
					m.backend = selected.id
					m.status = ""
					m.openBackendOperations()
				}
				return m, nil
			}
		case backendOpsScreen:
			switch msg.String() {
			case "esc", "backspace":
				m.screen = backendsScreen
				return m, nil
			case "enter":
				if selected, ok := m.selectedItem(); ok {
					return m, m.selectOperation(selected.id)
				}
				return m, nil
			}
		case snapshotScreen:
			switch msg.String() {
			case "esc", "backspace":
				m.openBackendOperations()
				return m, nil
			case "enter":
				if selected, ok := m.selectedItem(); ok {
					index := 0
					_, _ = fmt.Sscan(selected.id, &index)
					if index >= 0 && index < len(m.snapshots) {
						snapshot := m.snapshots[index]
						m.output = fmt.Sprintf("Snapshot: %s\nID: %s\nHost: %s\nTime: %s\nPaths: %s\nTags: %s",
							snapshot.ShortID, snapshot.ID, snapshot.Hostname, snapshot.Time.Format(time.RFC3339),
							strings.Join(snapshot.Paths, ", "), strings.Join(snapshot.Tags, ", "))
						m.screen = outputScreen
					}
				}
				return m, nil
			case "r":
				if selected, ok := m.selectedItem(); ok {
					index := 0
					_, _ = fmt.Sscan(selected.id, &index)
					if index >= 0 && index < len(m.snapshots) {
						m.operation = "restore"
						id := m.snapshots[index].ShortID
						if id == "" {
							id = m.snapshots[index].ID
						}
						m.startPrompt([]promptField{{label: "Restore destination", validate: required}}, id)
						return m, nil
					}
				}
				return m, nil
			}
		case promptScreen:
			switch msg.String() {
			case "esc":
				m.screen = backendOpsScreen
				return m, nil
			case "enter":
				field := m.fields[m.fieldIndex]
				value := strings.TrimSpace(m.input.Value())
				if field.secret {
					value = m.input.Value()
				}
				if field.validate != nil {
					if err := field.validate(strings.TrimSpace(value)); err != nil {
						m.status = err.Error()
						return m, nil
					}
				}
				m.fields[m.fieldIndex].value = value
				m.fieldIndex++
				if m.fieldIndex < len(m.fields) {
					m.input.SetValue(m.fields[m.fieldIndex].value)
					m.input.Placeholder = m.fields[m.fieldIndex].label
					m.input.EchoMode = textinput.EchoNormal
					if m.fields[m.fieldIndex].secret {
						m.input.EchoMode = textinput.EchoPassword
					}
					m.input.Focus()
					m.status = ""
					return m, textinput.Blink
				}
				return m, m.finishPrompts()
			}
		case confirmScreen:
			switch msg.String() {
			case "esc", "n":
				m.status = "Operation cancelled."
				if m.cleanupPath != "" {
					_ = os.Remove(m.cleanupPath)
					m.cleanupPath = ""
				}
				m.screen = backendOpsScreen
				if m.pendingName == "Backup location "+m.location {
					m.screen = locationsScreen
				}
				return m, nil
			case "y":
				return m, m.beginOperation()
			}
		case outputScreen:
			switch msg.String() {
			case "esc", "backspace", "enter":
				m.output, m.outputErr, m.outputStatus = "", "", ""
				m.screen = backendOpsScreen
				return m, nil
			case "q":
				return m, tea.Quit
			}
		}
	}
	if m.screen == promptScreen {
		var command tea.Cmd
		m.input, command = m.input.Update(message)
		return m, command
	}
	if m.screen == dashboardScreen || m.screen == locationsScreen || m.screen == backendsScreen || m.screen == backendOpsScreen || m.screen == snapshotScreen {
		var command tea.Cmd
		m.menu, command = m.menu.Update(message)
		return m, command
	}
	return m, nil
}

func (m *Model) selectedItem() (menuItem, bool) {
	selected, ok := m.menu.SelectedItem().(menuItem)
	return selected, ok
}

func (m *Model) selectOperation(operation string) tea.Cmd {
	if operation == "back" {
		m.screen = backendsScreen
		return nil
	}
	m.operation = operation
	switch operation {
	case "check":
		return m.prepareRestic(nil, []string{"check"}, "Check repository", false)
	case "snapshots":
		return m.prepareRestic(nil, []string{"snapshots", "--json"}, "Browse snapshots", false)
	case "stats":
		return m.prepareRestic(nil, []string{"stats"}, "Repository statistics", false)
	case "key-list":
		return m.prepareRestic(nil, []string{"key", "list"}, "List repository keys", false)
	case "list":
		m.startPrompt([]promptField{{label: "Object type (blobs, packs, snapshots, index, keys, locks)", value: "snapshots", validate: validObjectType}}, "")
	case "ls":
		m.startPrompt([]promptField{{label: "Snapshot ID", value: "latest", validate: required}, {label: "Path in snapshot (optional)"}}, "")
	case "restore":
		m.startPrompt([]promptField{{label: "Snapshot ID", value: "latest", validate: required}, {label: "Restore destination", validate: required}}, "")
	case "key-add":
		m.startPrompt([]promptField{{label: "New repository password", secret: true, validate: required}}, "")
	case "key-passwd":
		m.startPrompt([]promptField{{label: "New repository password", secret: true, validate: required}}, "")
	case "key-remove":
		m.startPrompt([]promptField{{label: "Key ID to remove", validate: required}}, "")
	}
	return nil
}

func (m *Model) prepareRestic(extraEnv map[string]string, args []string, name string, confirm bool) tea.Cmd {
	backend := m.config.Backends[m.backend]
	command, err := app.ResticCommand(m.backend, backend, args...)
	if err != nil {
		m.outputErr = err.Error()
		m.screen = outputScreen
		return nil
	}
	if extraEnv != nil {
		for key, value := range extraEnv {
			command.Env[key] = value
			command.Secrets = append(command.Secrets, value)
		}
	}
	m.prepareOperation(name, command, confirm)
	if !confirm {
		return m.beginOperation()
	}
	return nil
}

func (m *Model) startPrompt(fields []promptField, firstValue string) {
	m.fields = fields
	m.fieldIndex = 0
	if firstValue != "" {
		m.selectedSnapshot = firstValue
	} else {
		m.selectedSnapshot = ""
	}
	m.input = textinput.New()
	m.input.Placeholder = m.fields[0].label
	m.input.Prompt = "> "
	m.input.CharLimit = 512
	m.input.Width = max(m.width-16, 40)
	m.input.SetValue(m.fields[0].value)
	if m.fields[0].secret {
		m.input.EchoMode = textinput.EchoPassword
	}
	m.input.Focus()
	m.screen = promptScreen
	m.status = ""
}

func (m *Model) finishPrompts() tea.Cmd {
	fields := m.fields
	m.fields = nil
	m.status = ""
	var args []string
	switch m.operation {
	case "list":
		args = []string{"list", fields[0].value}
	case "ls":
		args = []string{"ls", fields[0].value}
		if fields[1].value != "" {
			args = append(args, fields[1].value)
		}
	case "restore":
		if m.selectedSnapshot != "" {
			args = []string{"restore", m.selectedSnapshot, "--target", fields[0].value}
		} else {
			args = []string{"restore", fields[0].value, "--target", fields[1].value}
		}
		m.selectedSnapshot = ""
	case "key-add":
		return m.prepareKeyAdd(fields[0].value)
	case "key-passwd":
		return m.prepareKeyPassword(fields[0].value)
	case "key-remove":
		args = []string{"key", "remove", fields[0].value}
	}
	name := "Restic " + m.operation
	confirm := m.operation == "restore" || m.operation == "key-remove"
	return m.prepareRestic(nil, args, name, confirm)
}

func (m *Model) prepareKeyAdd(password string) tea.Cmd {
	command, err := app.ResticCommand(m.backend, m.config.Backends[m.backend], "key", "add")
	if err != nil {
		m.outputErr = err.Error()
		m.screen = outputScreen
		return nil
	}
	passwordFile, err := writePasswordFile(password)
	if err != nil {
		m.outputErr = err.Error()
		m.screen = outputScreen
		return nil
	}
	command.Args = append(command.Args, "--new-password-file", passwordFile)
	command.Secrets = append(command.Secrets, password)
	m.prepareOperationWithCleanup("Add repository key", command, true, passwordFile)
	return nil
}

func (m *Model) prepareKeyPassword(password string) tea.Cmd {
	backend := m.config.Backends[m.backend]
	command, err := app.ResticCommand(m.backend, backend, "key", "passwd")
	if err != nil {
		m.outputErr = err.Error()
		m.screen = outputScreen
		return nil
	}
	passwordFile, err := writePasswordFile(password)
	if err != nil {
		m.outputErr = err.Error()
		m.screen = outputScreen
		return nil
	}
	command.Args = append(command.Args, "--new-password-file", passwordFile)
	command.Secrets = append(command.Secrets, password)
	m.prepareOperationWithCleanup("Change repository key password", command, true, passwordFile)
	return nil
}

func writePasswordFile(password string) (string, error) {
	file, err := os.CreateTemp("", "restic-studio-password-*")
	if err != nil {
		return "", fmt.Errorf("create temporary password file: %w", err)
	}
	path := file.Name()
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("secure temporary password file: %w", err)
	}
	if _, err := file.WriteString(password + "\n"); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write temporary password file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close temporary password file: %w", err)
	}
	return path, nil
}

func (m *Model) prepareOperation(name string, command app.Command, confirm bool) {
	m.pending = command
	m.pendingName = name
	if confirm {
		m.screen = confirmScreen
	} else {
		m.screen = runningScreen
	}
}

func (m *Model) prepareOperationWithCleanup(name string, command app.Command, confirm bool, cleanup string) {
	m.pending = command
	m.pendingName = name
	m.cleanupPath = cleanup
	if confirm {
		m.screen = confirmScreen
	} else {
		m.screen = runningScreen
	}
}

func (m *Model) beginOperation() tea.Cmd {
	ctx, cancel := context.WithCancel(m.parentCtx)
	m.cancel = cancel
	m.output = ""
	command := m.pending
	name := m.pendingName
	cleanup := m.cleanupPath
	progress := m.progress
	m.screen = runningScreen
	runCommand := func() tea.Msg {
		defer cancel()
		if cleanup != "" {
			defer os.Remove(cleanup)
		}
		onOutput := func(chunk string) {
			if progress != nil {
				progress(chunk)
			}
		}
		result, err := m.executor.RunWithProgress(ctx, command.Program, command.Args, command.Env, command.Secrets, onOutput)
		return operationDone{result: runnerResult{Output: result.Output, Err: err, ExitCode: result.ExitCode, Duration: result.Duration}, name: name}
	}
	return tea.Batch(m.spinner.Tick, runCommand)
}

type operationDone struct {
	result runnerResult
	name   string
}

type runnerResult struct {
	Output   string
	Err      error
	ExitCode int
	Duration time.Duration
}

func (m *Model) reloadConfig() error {
	cfg, err := config.Load(m.config.Path)
	if err != nil {
		return err
	}
	m.config = cfg
	return nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "Restic Studio\nLoading…"
	}
	header := titleStyle.Render("Restic Studio")
	configLine := mutedStyle.Render("Config: " + m.config.Path)
	var body string
	switch m.screen {
	case dashboardScreen, locationsScreen, backendsScreen, backendOpsScreen, snapshotScreen:
		body = m.menu.View()
		if m.screen == dashboardScreen && len(m.dependencies) > 0 {
			body += "\n" + errorStyle.Render("Dependency status")
			for _, issue := range m.dependencies {
				body += "\n" + errorStyle.Render("• "+issue)
			}
			if m.screen == dashboardScreen && m.outputErr != "" {
				body += "\n" + errorStyle.Render(m.outputErr)
			}
		}
		if m.status != "" {
			body += "\n" + mutedStyle.Render(m.status)
		}
	case promptScreen:
		body = titleStyle.Render(m.fields[m.fieldIndex].label) + "\n" + m.input.View()
		if m.status != "" {
			body += "\n" + errorStyle.Render(m.status)
		}
	case confirmScreen:
		var context string
		if m.pending.Program == "autorestic" {
			location := m.config.Locations[m.location]
			targets := strings.Join(location.To, ", ")
			if targets == "" {
				targets = "default backends"
			}
			context = fmt.Sprintf("Location: %s\nSources: %s\nBackends: %s", m.location, strings.Join(location.From, ", "), targets)
		} else {
			context = fmt.Sprintf("Backend: %s\nRepository: %s", m.backend, m.backendRepository())
		}
		body = boxStyle.Render(fmt.Sprintf("%s\n\n%s\nCommand: %s %s\n\nProceed? [y/N]",
			m.pendingName, context, m.pending.Program, strings.Join(m.pending.Args, " ")))
	case runningScreen:
		body = fmt.Sprintf("%s %s\n%s\n\nPress Esc or Ctrl+C to cancel.", m.spinner.View(), m.pendingName, m.status)
		if m.output != "" {
			lines := strings.Split(strings.TrimSpace(m.output), "\n")
			if len(lines) > 10 {
				lines = lines[len(lines)-10:]
			}
			body += "\n\n" + boxStyle.Width(max(m.width-8, 40)).Render(strings.Join(lines, "\n"))
		}
	case outputScreen:
		body = titleStyle.Render(m.pendingName)
		if m.outputStatus != "" {
			body += "\n" + mutedStyle.Render(m.outputStatus)
		}
		if m.outputErr != "" {
			body += "\n" + errorStyle.Render(m.outputErr)
		}
		if m.output != "" {
			body += "\n" + boxStyle.Width(max(m.width-8, 40)).Render(m.output)
		}
		if m.output == "" && m.outputErr == "" {
			body += "\n" + okStyle.Render("Command completed successfully.")
		}
		body += "\n\n" + mutedStyle.Render("Press Enter or Esc to return.")
	}
	return header + "\n" + configLine + "\n\n" + body + "\n\n" + mutedStyle.Render(helpText)
}

func (m Model) backendRepository() string {
	backend := m.config.Backends[m.backend]
	repo := backend.Path
	if backend.Type != "" && backend.Type != "local" && !strings.Contains(repo, ":") {
		repo = backend.Type + ":" + repo
	}
	return repo
}

func required(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("a value is required")
	}
	return nil
}

func validObjectType(value string) error {
	switch value {
	case "blobs", "packs", "snapshots", "index", "keys", "locks":
		return nil
	default:
		return errors.New("choose blobs, packs, snapshots, index, keys, or locks")
	}
}

func InitProgram(cfg *config.Config) error {
	model := New(cfg, runner.New())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model.parentCtx = ctx
	var program *tea.Program
	model.progress = func(output string) {
		program.Send(progressMessage(output))
	}
	program = tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithContext(ctx))
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run terminal UI: %w", err)
	}
	return nil
}

func appendProgress(existing, incoming string) string {
	const maxProgressRunes = 16_000
	combined := []rune(existing + incoming)
	if len(combined) <= maxProgressRunes {
		return string(combined)
	}
	return "[earlier progress omitted]\n" + string(combined[len(combined)-maxProgressRunes:])
}

func ResolveConfigPath(path string) (string, error) {
	if path != "" {
		return filepath.Abs(path)
	}
	return config.DefaultPath()
}

func IsCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

var _ tea.Model = Model{}
