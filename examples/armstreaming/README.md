# Arm Streaming Example

A client that drives an arm through the motion service's arm-streaming DoCommands. It
starts a session, pushes a sinusoidal joint-space path, polls the session status, and
flushes (or aborts) the session. See
[services/motion/builtin/streaming/README.md](../../services/motion/builtin/streaming/README.md)
for the full protocol reference.

## Running

Streaming requires a viam-server built with cgo and the `viam_rdk_cgo_have_cxx20_rt`
build tag (the client itself needs no special tags). From the repo root:

```
GO_BUILD_TAGS_EXTRA=viam_rdk_cgo_have_cxx20_rt make server
```

In one terminal, run the server with the bundled config, which defines a single fake
ur5e arm named `arm1`:

```
./bin/$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m)/viam-server -config examples/armstreaming/fake.json
```

In another terminal, run the client:

```
go run ./examples/armstreaming
```

Flags: `-host` (default `localhost:8080`), `-arm` (default `arm1`), and `-abort` to end
the session with `stream_abort` instead of `stream_flush`.

## Notes

- If the server was built **without** the `viam_rdk_cgo_have_cxx20_rt` tag, `stream_start`
  still succeeds but the session dies immediately in the background; the first
  `stream_push` (or a `stream_status` poll) then reports
  `arm streaming requires a cgo build with trajex support (build tag viam_rdk_cgo_have_cxx20_rt)`.
- The fake arm ignores per-point `Time` and `Constraints` — it snaps to each waypoint as
  it arrives — so its motion does not reflect the velocity/acceleration schedule that
  streaming computes. Timing behavior is only representative on an arm that executes the
  trajectory's time parameterization.
