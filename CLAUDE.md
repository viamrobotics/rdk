# rdk

This is the Viam RDK monorepo — the core server (`viam-server`), the Go SDK, and the Viam CLI all live here.

## Subpackages with their own CLAUDE.md

Always check for a package-level `CLAUDE.md` in the directory you're working in before starting. If one exists, its conventions take precedence over this file.

- `cli/CLAUDE.md` — Viam CLI (`viam` command, module generator, CLI subcommands)

## General conventions

- **Go version**: see `go.mod` `go` directive.
- **Full test suite**: `make test-go` (with race detection) or `make test-go-no-race` (faster, no race).
- **Lint**: `make lint-go`.
- **Single package test**: `TEST_TARGET=./path/to/pkg/... make test-go-no-race`.

## Protected paths (do not edit)

These are enforced at the CI settings level via `.claude/settings.ci.json`:

- `api/**` — generated proto stubs (regenerated from the `viamrobotics/api` repo).
- `.github/**` — CI workflows.
- `Makefile` — build glue.
- `go.mod`, `go.sum` — dependency manifests (changes require human review).

## Where things live

| Area | Paths |
|---|---|
| Server entrypoint | `web/cmd/server/main.go` |
| CLI | `cli/` (see `cli/CLAUDE.md`) |
| Component implementations | `components/<type>/` — server, client, fake, real drivers |
| Service implementations | `services/<type>/` |
| Motion planning | `motionplan/`, `referenceframe/`, `spatialmath/` |
| Robot lifecycle | `robot/`, `robot/client/`, `robot/impl/` |
| Module system | `module/`, `modulegeninputs/` |
