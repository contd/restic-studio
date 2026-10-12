# Developing Restic Studio

## Requirements

- Go 1.24 or newer
- `restic` and `autorestic` installed and available on `PATH`
- An Autorestic v2-style YAML configuration
- A terminal with keyboard input (macOS, Linux, and Windows)

## Build and run

### Make targets

With Make installed, run these targets from the project directory:

```sh
make help
make test
make docs
make build
make run ARGS='--config /path/to/autorestic.yml'
make gen
```

`build` creates the optimized `restic-studio` executable, `test` runs all Go
tests, and `docs` prints package documentation with `go doc` without creating
files. `run` starts the application from source and accepts CLI arguments through
`ARGS`. Override the Go executable with `GO` if needed, for example
`make build GO=/path/to/go`.

`make` (or `make all`) runs `test`, `docs`, `gen`, and `build` in that order,
stopping on failure, including when invoked with `-j`. `gen` runs
`bash docs/capture.sh` and regenerates the README recordings and images using
an isolated demo repository. It requires Go, Restic, Autorestic, Asciinema 3,
agg with `--select` support, FFmpeg, Expect, and Node.js. For a build without
capture dependencies, use `make build`.

### Go commands

```sh
go run ./cmd/restic-studio
```

By default, Restic Studio reads `~/.autorestic.yml`. Select another file with
`--config`:

```sh
go run ./cmd/restic-studio --config /path/to/autorestic.yml
```

Build and run a local executable:

```sh
go build -o restic-studio ./cmd/restic-studio
./restic-studio --config /path/to/autorestic.yml
```

Build a smaller release executable:

```sh
go build -trimpath -ldflags="-s -w" -o restic-studio ./cmd/restic-studio
```

Run the tests with `go test ./...`; use `go test -race ./...` for race
detection.

## Configuration

The YAML parser supports top-level `locations` and `backends` mappings, location `from` and `to` values as scalars or lists, location `options`, and backend `type`, `path`, `key`, and `env` fields. Each location source is
required; a backend must have a repository `path`; and every backend named in a location's `to` list must exist. Configuration errors are reported before the TUI starts.

Example (`testdata/autorestic.example.yml`):

```yaml
version: 2

locations:
  home:
    from:
      - ~/Documents
      - ~/Pictures
    to: [encrypted-local]
    options:
      forget:
        keep-daily: 7

backends:
  encrypted-local:
    type: local
    path: /path/to/restic-repository
    key: REPLACE_WITH_A_REPOSITORY_PASSWORD
    env:
      AWS_ACCESS_KEY_ID: REPLACE_WITH_AWS_ACCESS_KEY_ID
      AWS_SECRET_ACCESS_KEY: REPLACE_WITH_AWS_SECRET_ACCESS_KEY
```

Replace every placeholder with your own values. Protect this file because it may contain repository passwords and storage credentials. Restic Studio reads configuration only; it does not edit it.

## Using the interface

- **Dashboard:** See the active config and dependency status, then open Locations or Backends. Reload re-reads and validates the configuration.
- **Locations:** Select a location to review the backup command and confirm before Autorestic starts the backup.
- **Backends:** Select a repository to open repository tools: check, snapshots, stats, repository object listing, snapshot file browsing, restore, and key management.
- **Snapshots:** Browse JSON snapshot metadata; press Enter to inspect a snapshot or `r` to start a restore.
- **Operations:** Restore, add key, change key password, and remove key all require explicit confirmation. New key passwords are entered with masked input and passed through a temporary mode-0600 file that is removed after the operation.
- **Output:** Read captured command output, completion status, and errors. Press Enter or Esc to return.

Keyboard controls: Up/Down (or `j`/`k`) to navigate; Enter to select; Esc or Backspace to go back; `y` to approve a confirmation; `n` or Esc to decline; `q` to quit from the dashboard; Ctrl+C or Esc to request cancellation while a command is running.

Long-running commands run asynchronously, so the interface can request cancellation without blocking. Cancelled processes are not retried automatically.

## Credentials and subprocesses

Restic Studio launches commands with explicit argument lists and never invokes
a shell. It preserves inherited environment variables and overlays the
selected backend's configured `env` entries for child processes; configured
values take precedence when names collide. For Autorestic backups, it supplies
environment entries only from backends selected by that location; when `to` is
omitted, it uses all configured backends. Conflicting values for the same
variable across selected backends are rejected with the variable name, never
the values.

For Restic commands, a backend `key` is supplied as `RESTIC_PASSWORD`. The
application does not print configured values, and redacts configured
credential values from captured command output and error output. Passwords are
not placed in command-line arguments. Use appropriately restrictive file
permissions for the Autorestic YAML file; credentials inherited from the
calling environment remain under the user's control.

The startup dashboard reports missing `restic` or `autorestic` binaries with
installation guidance. An operation that needs a missing binary reports its
error in the output view. Repository/authentication errors from Restic are
shown with the command's exit status; verify the backend path, password, and
storage environment values in the configuration without sharing their
contents.

## Development

Project packages:

- `internal/config`: YAML models, loading, and validation
- `internal/app`: subprocess argument and backend-environment construction
- `internal/runner`: context-aware process execution, environment merging, and
  output redaction
- `internal/tui`: Bubble Tea interface and asynchronous operation workflows
- `cmd/restic-studio`: command-line entry point

To run tests or build:

```sh
go test ./...
go build ./cmd/restic-studio
```
