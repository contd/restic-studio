# Developing Restic Studio

## Requirements

- Go 1.24 or newer
- Git and Bash for Make's version-tagging command and its tests
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
make api-docs
make build
make run ARGS='--config /path/to/autorestic.yml'
make gen
```

`build` creates the optimized `restic-studio` executable, `test` runs all Go
tests and isolated Git version-tagging tests, and `docs` generates the static
documentation site in `_site`.
`api-docs` prints package documentation with `go doc` without creating files.
`run` starts the application from source and accepts CLI arguments through
`ARGS`. Override the Go executable with `GO` if needed, for example
`make build GO=/path/to/go`.

`make` (or `make all`) runs `test`, `docs`, `gen`, and `build` in that order,
stopping on failure, including when invoked with `-j`. `gen` runs
`bash docs/capture.sh` and regenerates the README recordings and images using
an isolated demo repository. It requires Go, Restic, Autorestic, Asciinema 3,
agg with `--select` support, FFmpeg, Expect, and Node.js. For a build without
capture dependencies, use `make build`.

### Documentation site

Run `make docs` to generate the GitHub Pages site. The home page is rendered
from [README.md](README.md), and `development.html` is rendered from this file.
Both pages include navigation, responsive styling, Markdown tables, and
heading anchors. Existing images, GIFs, local Asciinema recordings, and example
configuration files are copied into the site without regenerating captures.
Mermaid diagrams are rendered in the browser using Mermaid 11.4.1 from jsDelivr;
rendering requires internet access. If it fails, the page reports the failure
and retains diagram source.

The generator requires only Go and its module dependencies. Override the
destination with `make docs SITE_DIR=/path/to/site`. The default `_site`
directory is ignored by Git; do not use a source directory as the destination.
Serve `_site` with a local static HTTP server to preview it.

### GitHub Actions, releases, and Pages

The workflow in `.github/workflows/ci.yml` runs on pushes, pull requests, and
manual dispatches:

1. Run `go vet` and uncached race-enabled tests on Linux, macOS, and Windows
   with stable Go, plus Linux tests on the minimum supported Go 1.24.
2. Only after every test job succeeds, build CGO-free Linux, macOS, and Windows
   binaries for both amd64 and arm64. Each archive includes the binary,
   README, development guide, and example configuration.
3. Only after every binary build succeeds, generate the site using `make docs`.

All runs upload binary archives and the generated site as workflow artifacts.
Cross-compilation does not establish runtime compatibility on every target
architecture. Tests use helper processes and do not need installed Restic or
Autorestic; end users still need both executables.

Push a version tag such as `v1.0.0` to publish a GitHub Release with the six
archives, SHA-256 checksums in `SHA256SUMS`, and generated release notes. The
release waits for both binary builds and site generation. Re-running a workflow
for an already-published tag does not replace its release; create a new version
tag instead.

For Pages, select **Settings -> Pages -> Build and deployment -> Source:
GitHub Actions** in the GitHub repository. Successful default-branch runs
deploy the generated site to the `github-pages` environment. Pull requests,
other branches, and tags generate site artifacts but do not deploy Pages.
Any configured environment approvals must be granted before deployment.

```sh
make test TEST_FLAGS='-race -count=1'
make tag minor
# Push the exact tag printed by the command:
git push origin v0.1.0
```

#### Bump version tags

Use `make tag major`, `make tag minor`, or `make tag patch` from a clean
worktree with a committed HEAD and a configured Git identity. The command
creates an annotated local tag at HEAD; it does not commit changes, fetch
tags, or push anything.

It selects the highest local stable version matching `vMAJOR.MINOR.PATCH`,
not the most recently created tag. Prerelease tags, build-metadata tags,
malformed versions, and leading-zero components are ignored. Fetch remote
tags with `git fetch --tags` first if needed to ensure the local tag list is
current.

| Command | Starting at `v0.1.1` | Reset components |
| --- | --- | --- |
| `make tag major` | `v1.0.0` | Minor and patch |
| `make tag minor` | `v0.2.0` | Patch |
| `make tag patch` | `v0.1.2` | None |

Without matching tags, the starting version is `v0.0.0`: major creates
`v1.0.0`, minor creates `v0.1.0`, and patch creates `v0.0.1`. Tracked changes
and non-ignored untracked files cause an explicit failure. Existing tags
are never overwritten. Push the exact new tag printed by the command to
trigger the GitHub Release workflow. Run tagging separately from other Make
targets; only one bump argument is accepted.

`make test` also runs `scripts/tag_test.sh`, which exercises tagging in
disposable Git repositories without modifying this project's tags.

GitHub workflows require the project files to be committed and pushed to a
GitHub repository. Generated screenshots and recordings must be committed too;
CI consumes those assets rather than running `make gen`.

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
