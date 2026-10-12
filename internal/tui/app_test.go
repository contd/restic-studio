package tui

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/restic-studio/restic-studio/internal/config"
	"github.com/restic-studio/restic-studio/internal/runner"
)

func TestDashboardNavigationOpensLocations(t *testing.T) {
	cfg := &config.Config{
		Locations: map[string]config.Location{"home": {From: config.StringList{"/home"}}},
		Backends:  map[string]config.Backend{},
	}
	model := New(cfg, runner.New())
	model.width, model.height = 100, 40
	model.setMenu(model.dashboardItems(), "Restic Studio")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.screen != locationsScreen {
		t.Fatalf("screen = %v, want locations", got.screen)
	}
}

func TestSnapshotRestoreUsesSelectedIDAndDestination(t *testing.T) {
	cfg := &config.Config{
		Locations: map[string]config.Location{},
		Backends:  map[string]config.Backend{"archive": {Type: "local", Path: "/repo", Key: "password"}},
	}
	model := New(cfg, runner.New())
	model.backend = "archive"
	model.operation = "restore"
	model.startPrompt([]promptField{{label: "Restore destination", validate: required}}, "snapshot-id")
	model.input.SetValue("/restore-here")

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	want := []string{"-r", "/repo", "restore", "snapshot-id", "--target", "/restore-here"}
	if got.screen != confirmScreen || !reflect.DeepEqual(got.pending.Args, want) {
		t.Fatalf("restore confirmation args = %#v, screen = %v", got.pending.Args, got.screen)
	}
}

func TestPromptStartsWithConfiguredDefault(t *testing.T) {
	model := New(&config.Config{Locations: map[string]config.Location{}, Backends: map[string]config.Backend{}}, runner.New())
	model.startPrompt([]promptField{{label: "Snapshot ID", value: "latest", validate: required}}, "")
	if got := model.input.Value(); got != "latest" {
		t.Fatalf("prompt default = %q, want latest", got)
	}
}
