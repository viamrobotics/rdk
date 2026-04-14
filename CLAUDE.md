# Viam RDK

Viam RDK monorepo — `viam-server`, the Go SDK, and the Viam CLI all live here. Go module path: `go.viam.com/rdk`.

## Codebase Structure

```
cli/                              — Viam CLI (`viam` command). See cli/CLAUDE.md.
components/<name>/                — Hardware component types (arm, base, camera, etc.)
  <name>.go                       — Interface, Named type, registration
  client.go                       — gRPC client wrapper (Go SDK side)
  server.go                       — gRPC service handler (server side)
  fake/                           — Fake implementation for tests
services/<name>/                  — Service types (vision, motion, slam, navigation, etc.)
robot/                            — Robot lifecycle
  client/                         — RobotClient (Go SDK entry point)
  impl/                           — Server-side robot implementation
resource/                         — Resource registry, base interfaces (e.g., Shaped)
config/                           — Config parsing and loading
motionplan/                       — Motion planning and constraints
referenceframe/                   — Frame system and kinematics
spatialmath/                      — Geometries, poses, transforms
module/, modulegeninputs/         — Modular component support
web/cmd/server/                   — `viam-server` entrypoint
api/                              — Generated protobuf code (NEVER EDIT, regenerated from viamrobotics/api)
testutils/                        — Test helpers and mocks
```

All component/service clients follow the same pattern: `client.go` wraps the gRPC stub, `server.go` implements the gRPC service handler, `fake/` provides a test implementation.

## Subpackages with their own CLAUDE.md

Always check for a package-level `CLAUDE.md` in the directory you're working in. If one exists, its conventions take precedence over this file.

- `cli/CLAUDE.md` — Viam CLI

## Go Conventions

- **Formatting**: `gofmt`. Run via `make lint-go`.
- **Linting**: `golangci-lint` via `make lint-go`. Config at `etc/.golangci.yaml`.
- **Method signatures** for component/service/robot client methods follow this pattern:
  ```go
  func (c *client) MethodName(
      ctx context.Context,
      arg1 Type1,
      extra map[string]interface{},
  ) (ReturnType, error) {
      // build proto request, call stub, convert response, return Go type
  }
  ```
- **Context**: `ctx context.Context` is always the first parameter after the receiver. **Never store `context.Context` in a struct** — Go anti-pattern.
- **Errors**: idiomatic `(value, error)`. Return errors, don't panic.
- **Tests**: same package, `*_test.go` naming. Use `go.viam.com/test` for assertions.

## Protected Paths

Enforced at the CI settings level via `.claude/settings.ci.json`:

- `api/**` — generated proto stubs.
- `.github/**` — CI workflows.
- `Makefile`, `go.mod`, `go.sum` — build glue and dependency manifests.

## Verification Commands

- Build everything: `go build ./...`
- Build CLI only: `go build ./cli/...` or `make cli`
- Full test suite (with race detection): `make test-go`
- Faster test suite: `make test-go-no-race`
- Single package: `TEST_TARGET=./path/to/pkg/... make test-go-no-race`
- Lint: `make lint-go`
