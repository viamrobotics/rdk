# RDK — Robot Development Kit

This repository is `go.viam.com/rdk` and ships three primary surfaces:

- **viam-server** — the robot agent. Entrypoint: `web/cmd/server/`. Core robot implementation lives in `robot/`; the gRPC layer is in `web/server/`.
- **Go SDK** — covers two clients:
  - Robot client (`robot/client/`) — connects to a viam-server to drive components and services.
  - App client (`app/`) — talks to Viam cloud APIs (`app_client.go`, `data_client.go`, `billing_client.go`, `mltraining_client.go`, `provisioning_client.go`); `viam_client.go` is the umbrella entrypoint.
- **Viam CLI** — lives in `cli/`. See `cli/STYLEGUIDE.md` and `cli/CONTRIBUTING.md` before changing CLI code.

## Layout

- `components/`, `services/`, `module/` — resource implementations and the module SDK. The bulk of the code follows this pattern; before adding a new component or service, read 1–2 existing ones in the same directory.
- `config/`, `resource/`, `motionplan/`, `referenceframe/`, `data/`, `logging/` — shared infrastructure used across the surfaces above.
- `examples/customresources/` — sample modules. The only `.pb.go` files in the tree live here; protobuf for the main APIs comes from `go.viam.com/api`.

## Conventions

- Verify changes with `make lint-go` and `make test-go`. The `rdk-devenv` container has Go tooling pre-installed; locally, run `make tool-install` first.
- Generated protobuf (`*.pb.go`), build files (`Makefile`, `*.make`), `go.sum`, and workflow files are deny-listed in `.claude/settings.ci.json`. Update `go.sum` via `go mod tidy`.
