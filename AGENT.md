# Restic Studio

Restic Studio is a **Go-based terminal user interface (TUI)** for visually managing Autorestic configurations and the Restic repositories they control. It provides an interactive, menu-driven experience for browsing backup locations, managing backends, inspecting snapshots, and performing common Restic operations without memorizing command-line syntax.

The application reads `~/.autorestic.yml` by default, presents configured locations and backends in interactive menus, and requests confirmation before destructive or write-like operations. It delegates actual backup and repository operations to the installed `autorestic` and `restic` command-line binaries rather than reimplementing their functionality.

Build the application in Go using a maintainable project structure, idiomatic Go patterns, and a terminal UI library such as [Bubble Tea](https://github.com/charmbracelet/bubbletea), with [Lip Gloss](https://github.com/charmbracelet/lipgloss) for styling and [Bubbles](https://github.com/charmbracelet/bubbles) for reusable UI components.

## Features

### 1. Configuration Management

- Load Autorestic configuration from `~/.autorestic.yml` by default.
- Allow users to specify an alternative configuration file using a `--config` command-line flag.
- Parse an Autorestic v2-style YAML configuration, including top-level `locations` and `backends`.
- Display configured backup locations and backends in interactive menus.
- Show relevant configuration details, such as location paths, associated backends, backup options, and repository identifiers, when available.
- Support scalar fields, inline lists, block lists, and backend `env` maps.
- Handle missing configuration files, malformed YAML, missing fields, and invalid configuration references with clear, actionable error messages.
- Never display passwords, access keys, secret environment variables, or other sensitive configuration values in the UI or logs.

Use a standard Go YAML library, such as `gopkg.in/yaml.v3`, rather than implementing a custom YAML parser. Model only the Autorestic configuration fields the application needs, while preserving compatibility with the supported configuration shapes.

### 2. Backup Operations

- Browse configured Autorestic locations.
- Display each location's associated backend or backends.
- Run backups for a selected location through the installed `autorestic` binary.
- Support selecting individual locations and, where practical, running backups for multiple locations.
- Display command output, progress information, exit status, and errors in a dedicated terminal output view.
- Allow users to cancel long-running operations when supported by the underlying process.
- Ask for confirmation before starting a backup.
- Prevent accidental duplicate operations and communicate when a backup is already running.

The application must invoke external commands using Go's `os/exec` package with explicit argument lists. Do not construct shell command strings or invoke an unnecessary shell.

### 3. Restic Repository Operations

Allow users to select a configured backend and execute the following Restic operations against its repository.

**Repository verification**
- `check` — Verify repository consistency and integrity.

**Repository keys**
- `key list` — List repository keys.
- `key add` — Add a new repository key.
- `key passwd` — Change a repository key's password.
- `key remove` — Remove a repository key, with explicit confirmation.

**Browsing and inspection**
- `list` — List files and directories in a snapshot.
- `ls` — Browse snapshot contents.
- `snapshots` — List available snapshots.
- `stats` — Display repository and snapshot statistics.

**Restore operations**
- `restore` — Restore files or directories from a selected snapshot to a specified destination.

Provide context-sensitive menus, argument prompts, and validation for each operation. Show the selected backend, repository, snapshot, and destination where applicable before execution.

Require confirmation before restoring files or directories, changing repository keys, or performing any other potentially destructive operation.

### 4. Credentials and Environment Variables

- Use a configured backend's `key` value as the Restic repository password by setting `RESTIC_PASSWORD` in the child process environment.
- Pass backend `env` values to the relevant subprocesses so Restic and Autorestic can access required credentials, including S3-compatible storage credentials.
- Preserve the user's existing environment variables unless the application explicitly needs to override them.
- Apply configuration-derived environment variables only to the subprocesses that require them.
- Never print secret values to the terminal, diagnostic output, logs, error messages, or debug traces.
- Avoid exposing credentials through command-line arguments.
- Keep password prompts masked when interactive input is required.
- Validate missing credentials and report which configuration field needs attention without revealing its value.

When building subprocess environments, merge configured values with the inherited process environment deliberately and document how conflicting variable names are handled.

### 5. Interactive Terminal UI

Build a polished, keyboard-driven TUI that works in common terminal emulators on macOS, Linux, and Windows.

Include the following screens or views:

- **Dashboard:** Overview of configured locations, backends, and available actions.
- **Locations:** Browse locations and initiate backups.
- **Backends:** Select repositories and access Restic operations.
- **Snapshots:** Browse available snapshots and inspect their metadata.
- **Repository tools:** Access checks, keys, statistics, listing, and restore workflows.
- **Command output:** Display subprocess output, operation status, and errors.
- **Confirmation dialogs:** Require explicit approval for backups, restores, key modifications, and other write-like operations.
- **Configuration status:** Show the active configuration path and any detected configuration or dependency issues.

Use consistent visual styling, readable status indicators, clear navigation hints, and useful empty states.

Support common keyboard interactions, including navigation, selection, cancellation, confirmation, and returning to the previous screen. Keep long-running subprocess execution responsive so the UI remains interactive.

### 6. Process Execution and Error Handling

- Detect whether `restic` and `autorestic` are available on `PATH`.
- Check command availability at startup and provide helpful installation guidance when dependencies are missing.
- Execute commands with `os/exec`, explicit arguments, and carefully constructed environment variables.
- Capture standard output and standard error without exposing secrets.
- Display command exit codes and actionable error messages.
- Distinguish configuration errors, missing dependencies, authentication failures, repository errors, and user cancellation where possible.
- Handle interrupted processes and terminal shutdown gracefully.
- Ensure subprocesses do not leave the terminal in a broken state.
- Avoid blocking the main UI event loop while commands are running.
- Never silently retry a potentially destructive operation.

Use Go contexts and process cancellation where appropriate. Long-running operations should run asynchronously, with their results communicated to the TUI through Bubble Tea messages or an equivalent concurrency-safe mechanism.

### 7. Configuration Parsing and Validation

Use a maintained YAML library such as `gopkg.in/yaml.v3` to parse the configuration.

Support the following configuration constructs needed by the application:

- Top-level `locations` and `backends` mappings.
- Scalar fields, including names, paths, and repository settings.
- Inline lists and YAML block lists.
- Backend `env` maps.
- Location-to-backend references.
- Optional fields and reasonable defaults.

The parser should not attempt to implement YAML syntax itself. Instead, define typed Go structures for the supported fields, validate required values, and retain enough configuration information to construct correct commands.

Validate configuration references before execution and report missing or incompatible settings clearly. Where appropriate, allow the user to open a configuration file in their preferred editor rather than attempting to modify configuration automatically.

## Technical Requirements

- **Language:** Go.
- **Minimum Go version:** Go 1.24 or newer, using a currently supported stable Go release compatible with the project's dependencies.
- **TUI framework:** Bubble Tea.
- **Terminal styling:** Lip Gloss.
- **Reusable UI components:** Bubbles.
- **YAML parsing:** `gopkg.in/yaml.v3`.
- **Subprocess execution:** Go standard library `os/exec`.
- **Process lifecycle and cancellation:** Go standard library `context`.
- **Configuration location:** `~/.autorestic.yml` by default.
- **External dependencies:** `restic` and `autorestic` must be installed and available on `PATH`.
- **Supported configuration:** Autorestic v2-style YAML.
- **Supported platforms:** macOS, Linux, and Windows, subject to the availability of compatible Restic and Autorestic binaries.

Use Go modules for dependency management. Pin or constrain dependencies appropriately, keep the module file reproducible, and include a `go.sum` file.

Prefer small, focused packages with clear responsibilities. Avoid unnecessary abstractions, global mutable state, and a monolithic `main.go`.

## Suggested Project Structure

Organize the project using a structure similar to the following:

```text
restic-studio/
├── cmd/
│   └── restic-studio/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   ├── parser.go
│   │   └── validation.go
│   ├── runner/
│   │   ├── executor.go
│   │   ├── environment.go
│   │   └── process.go
│   ├── restic/
│   │   ├── client.go
│   │   ├── snapshots.go
│   │   ├── repository.go
│   │   └── restore.go
│   ├── autorestic/
│   │   └── client.go
│   └── tui/
│       ├── app.go
│       ├── dashboard.go
│       ├── locations.go
│       ├── backends.go
│       ├── snapshots.go
│       ├── operations.go
│       └── styles.go
├── testdata/
│   └── autorestic.example.yml
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── LICENSE
```

Treat this as a recommended starting point, not a requirement to create empty packages or files that do not provide useful functionality. Keep the implementation straightforward and idiomatic.

## Command-Line Interface

The application should support these commands and options:

Start using the default configuration:

```sh
go run ./cmd/restic-studio
```

Start with an alternative configuration:

```sh
go run ./cmd/restic-studio --config /path/to/autorestic.yml
```

Build a local executable:

```sh
go build -o restic-studio ./cmd/restic-studio
```

Run the compiled application:

```sh
./restic-studio
```

Build an optimized release binary:

```sh
go build -trimpath -ldflags="-s -w" -o restic-studio ./cmd/restic-studio
```

Use standard Go flag parsing or a lightweight CLI library if additional commands and options are needed. The `--config` flag should work consistently for both `go run` and compiled executables.

## Testing Requirements

Include unit tests and integration-oriented tests for the core functionality.

- Test parsing of valid Autorestic configuration files.
- Test missing files, invalid YAML, malformed entries, and missing backend references.
- Test inline lists, block lists, and backend environment mappings.
- Test environment merging and verify that sensitive values are never included in diagnostic output.
- Test subprocess argument construction without executing destructive commands.
- Test missing external binaries and nonzero exit codes.
- Test context cancellation and subprocess cleanup.
- Test TUI navigation and confirmation workflows using Bubble Tea's testable model/update architecture where practical.
- Use temporary configuration files and fake executables for subprocess tests.
- Do not require access to a real backup repository for routine unit tests.

Provide clear instructions for running the tests:

```sh
go test ./...
```

Optionally include race detection:

```sh
go test -race ./...
```

## Documentation

Provide a README covering:

- What Restic Studio does.
- Prerequisites and installation.
- Go version and dependency requirements.
- Building and running the application.
- Configuration file discovery and the `--config` flag.
- Supported Autorestic configuration fields.
- Available backup and repository operations.
- Keyboard controls.
- Credential handling and security considerations.
- Troubleshooting missing binaries, invalid configuration, authentication failures, and repository errors.
- Running tests and building release binaries.

Include a sanitized example Autorestic configuration. Use placeholder values for credentials and clearly indicate that they must be replaced with actual values.

## Implementation Priorities

Implement the application incrementally, keeping each stage runnable and testable:

1. **Foundation:** Create the Go module, CLI entry point, typed configuration structures, YAML parsing, validation, and dependency checks.
2. **Interactive shell:** Implement the Bubble Tea application, dashboard, navigation, location selection, and backend selection.
3. **Backup execution:** Integrate Autorestic subprocess execution, progress output, cancellation, and confirmation dialogs.
4. **Restic integration:** Implement repository checks, snapshots, listing, statistics, key management, and restore workflows.
5. **Security and reliability:** Harden environment handling, redact secrets, validate arguments, and improve error handling and process cleanup.
6. **Testing and documentation:** Add comprehensive tests, a sanitized sample configuration, build instructions, and user documentation.

Deliver a functional application rather than a mockup. Core menus must invoke real commands, display actual results, handle errors, and preserve a responsive terminal interface.

## Important Design Constraints

- Do not reimplement Restic or Autorestic backup functionality.
- Do not execute user-supplied command strings through a shell.
- Do not expose passwords, storage credentials, or secret environment values.
- Require explicit confirmation before write-like or destructive operations.
- Do not perform restores or modify repository keys without clear user approval.
- Keep configuration parsing scoped to the application's needs while relying on a proper YAML parser.
- Use Go's standard concurrency and process-management facilities where practical.
- Keep the TUI maintainable, modular, accessible, and easy to extend with additional Restic and Autorestic features.

The finished product should feel like a practical terminal-based backup management studio: fast to navigate, safe to operate, visually clear, and fully integrated with the user's existing Autorestic configuration and Restic repositories.