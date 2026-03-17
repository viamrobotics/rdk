# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the **Viam CLI** (`viam`), a Go command-line tool for interacting with the Viam platform. It lives inside the larger `rdk` monorepo at `cli/`. The Go module path is `go.viam.com/rdk` (shared with the parent repo); the CLI package is `go.viam.com/rdk/cli` with entry point at `cli/viam/main.go`.

## Common Commands

All commands run from the **rdk repo root** (`/Users/allison.chiang/Documents/Code/rdk/`), not from `cli/`.

### Build the CLI
```sh
make cli
# Binary output: bin/<GOOS>-<GOARCH>/viam-cli

# Or for quick local dev (installs to go bin):
go build -o ~/go/bin/viam cli/viam/main.go
```

### Run Tests
```sh
# All tests with race detection (uses gotestsum):
make test-go

# All tests without race detection:
make test-go-no-race

# Single package:
TEST_TARGET=./cli/... make test-go

# Single test by name:
TEST_TARGET="-run TestName ./cli/..." make test-go-no-race

# Or directly with go test:
go test -v -run TestName ./cli/...
```

### Lint
```sh
make lint        # Full lint (go + actionlint)
make lint-go     # Go lint only (runs go mod tidy, then golangci-lint with --fix)
```
Lint config: `etc/.golangci.yaml`. Linter: `golangci-lint` v1.62.2.

### Generate
```sh
make generate-go
```

## Architecture

The CLI is a single Go package (`cli/`) using `urfave/cli/v2`. The command tree is built in `NewApp()` in `app.go`.

### Key patterns to follow (from STYLEGUIDE.md):

1. **Typed arguments with `createCommandWithT[T]()`**: Every command action must use a typed args struct instead of manually parsing flags from context.
```go
type fooArgs struct {
    Bar int
    Baz string
}
func fooAction(ctx cli.Context, args fooArgs) error { ... }
// In command definition:
Action: createCommandWithT[fooArgs](fooAction),
```

2. **Reuse existing flags** — don't create duplicate flag constants for the same CLI flag name.

3. **`HideHelpCommand: true`** on parent commands that only have subcommands.

4. **`createUsageText()`** for generating usage text — pass the fully qualified command name (without `viam`) and only required flags.

5. **`formatAcceptedValues()`** for flags with a discrete set of valid values.

6. **`DefaultText` field** for default values instead of inlining defaults in `Usage` strings.

### File organization by domain:

| Domain | Key files |
|--------|-----------|
| CLI app & command tree | `app.go` |
| Authentication | `auth.go` |
| Machine/robot control | `client.go` |
| Module generation | `module_generate.go`, `module_generate/` |
| Module building | `module_build.go` |
| Module registry | `module_registry.go` |
| Data management | `data.go` |
| Data pipelines | `datapipelines.go` |
| ML training/inference | `ml_training.go`, `ml_inference.go` |
| Utilities & config | `utils.go`, `config.go`, `defaults.go`, `flags.go` |

### Key helpers (in `app.go` and `utils.go`):
- `createCommandWithT[T]()` — wraps action with typed arg parsing
- `parseStructFromCtx[T]()` — parses CLI flags into a typed struct
- `getGlobalArgs()` — extracts org/location/machine context
- `viamClient` — core RPC client for Viam app/machine APIs

### Module generation templates:
`module_generate/` contains `//go:embed` templates for scaffolding new modules (Go, Python, C++). Templates live in `_templates/` and scripts in `scripts/`.

## Build Notes

- CLI builds use `CGO_ENABLED=0` with tags `osusergo,netgo,no_cgo` to avoid nlopt CGO dependency from the broader rdk module.
- CI builds for 5 targets: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64.
- Version info is injected via `-ldflags` from git revision, tag version, and compile date.
