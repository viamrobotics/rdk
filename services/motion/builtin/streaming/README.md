# Arm Trajectory Streaming

Arm streaming lets a client push coarse joint-space waypoints to the builtin motion
service and have them time-parameterized on the fly into a smooth trajectory that is
trickled into the arm, keeping a small, bounded "runway" of motion buffered inside the
arm at all times. Because time parameterization happens incrementally, a waypoint pushed
while the arm is already moving pivots the remaining trajectory instead of requiring a
stop-and-replan. Use it for live, low-latency following (teleoperation, visual servoing,
tracking a moving target); use `Move` or `DoPlan`/`DoExecute` when the whole motion is
known up front.

Using streaming from a client? Read [Part 1](#part-1-using-arm-streaming). Working on
this package? Read [Part 2](#part-2-architecture).

## Part 1: Using arm streaming

### Requirements

- The viam-server must be built with cgo and the `viam_rdk_cgo_have_cxx20_rt` build tag,
  which enables the [trajex](https://github.com/viam-modules/trajex) time-parameterization
  library. From the repo root: `GO_BUILD_TAGS_EXTRA=viam_rdk_cgo_have_cxx20_rt make server`.
- The target arm must implement `MoveThroughJointPositionsStreamed`.

On a server built without the tag, `stream_start` still succeeds — the failure happens in
the background when the session starts running. The next `stream_push`, or a
`stream_status` poll, reports:

```
arm streaming requires a cgo build with trajex support (build tag viam_rdk_cgo_have_cxx20_rt)
```

### The command protocol

Streaming is driven through five `DoCommand` keys on the builtin motion service. The doc
comment on `DoCommand` in `services/motion/builtin/builtin.go` is the canonical wire
spec; this table must be kept in sync with it. The keys are exported as Go constants
(`builtin.DoStreamStart` etc.) so Go clients need not hardcode them.

| Command | Request | Response | Blocks until |
|---|---|---|---|
| `stream_start` | `{"stream_start": {"arm": "myArm", "options": {...}}}` (`options` optional) | `{"ok": 1}` | session is started |
| `stream_push` | `{"stream_push": [[j0, j1, ...], [j0, j1, ...], ...]}` | `{"ok": 1}` | every target is accepted |
| `stream_flush` | `{"stream_flush": true}` | `{"running": false, "error": "..."}` | queued trajectory is drained **and** (by the runway estimate) executed, or ctx expires |
| `stream_abort` | `{"stream_abort": true}` | `{"running": false, "error": "..."}` | teardown finishes, or ctx expires |
| `stream_status` | `{"stream_status": true}` | `{"running": true, "arm": "myArm", "error": "..."}` | (immediate) |

Joint values are `referenceframe.Input`s: radians for revolute joints, millimeters for
prismatic ones. In flush/abort/status responses, `error` is present only once the session
has ended with an error, and `arm` (status only) once a session has started.

### Session lifecycle

```
              stream_start          stream_flush         drain + wait
   (idle) ----------------> running ------------> flushing ----------> ended
                               |                                         ^
                               |            stream_abort                 |
                               +-----------------------------------------+
                                        (or internal error)
```

- One session at a time. Starting while a session is running (or still shutting down)
  fails with `a stream is already running or still shutting down; call stream_flush or
  stream_abort first`.
- The trajectory is seeded from the arm's joint positions at `stream_start`, so the first
  pushed waypoints should be near the arm's current position.
- The session runs detached from any request context: it outlives the `DoCommand` calls
  that drive it, and keeps draining or tearing down even if a flush/abort caller gives up
  waiting. Closing the motion service aborts the session.

### Options

`stream_start` accepts an `options` object; omitted fields take their defaults.

| Option | Default | Meaning |
|---|---|---|
| `target_runway_in_arm_ms` | 100 | Duration of trajectory to keep buffered inside the arm. |
| `send_to_arm_interval_ms` | 10 | Interval at which the runway is topped up. Must be less than `target_runway_in_arm_ms`. |
| `vel_limit_deg_per_sec` | 10 | Per-joint velocity limit used by time parameterization. |
| `accel_limit_deg_per_sec2` | 10 | Per-joint acceleration limit used by time parameterization. |

All four must be positive. Invalid options fail `stream_start` synchronously. The
interval and the velocity/acceleration limits are currently caller-supplied; there are
TODOs in `options.go` to source them from the arm's properties API instead.

### Error handling and retries

- **Errors surface asynchronously.** A session that dies mid-stream (arm error,
  time-parameterization error) reports it on the *next* interaction: an in-flight or
  subsequent push fails with `streaming session ended: <cause>`, and flush/abort/status
  carry the cause in their `error` field.
- **An aborted session reports `context canceled`.** `stream_abort` ends the session by
  cancellation, so its final `error` field is `context canceled` — that is the normal
  signature of an abort, not a failure.
- **Flush and abort are retryable.** If the caller's context expires before the drain or
  teardown finishes, the call reports the session still running — as `{"running": true}`
  with no error, or as a `DeadlineExceeded` RPC error, depending on timing — and the
  session keeps winding down on its own. Repeat the command or poll `stream_status` until
  `running` is `false`.
- **Operations are sequential.** Pushes and flush must not overlap; a concurrent call
  fails with `overlapping stream operations: pushes and flush must be issued
  sequentially`. A push after flush fails with `streaming session is flushing or has
  ended; no further targets are accepted`.
- **Pushes block as backpressure.** A push returns only once the session has accepted
  every target in it. There is no queue between the client and the session — see
  [backpressure](#the-coordinator-loop) below for what happens when the client is faster
  or slower than the arm.

### Complete example

[`examples/armstreaming`](../../../../examples/armstreaming) is a runnable client that
performs the full lifecycle against a fake arm; its README has the two-terminal run
recipe. The core of it:

```go
// Start a session on the arm (options shown are the defaults).
resp, err := motionService.DoCommand(ctx, map[string]interface{}{
    builtin.DoStreamStart: map[string]interface{}{
        "arm": "arm1",
        "options": map[string]interface{}{
            "target_runway_in_arm_ms":  100,
            "send_to_arm_interval_ms":  10,
            "vel_limit_deg_per_sec":    10,
            "accel_limit_deg_per_sec2": 10,
        },
    },
})

// Push waypoints; each push blocks until all its targets are accepted.
_, err = motionService.DoCommand(ctx, map[string]interface{}{
    builtin.DoStreamPush: [][]float64{{0.1, 0, 0, 0, 0, 0}, {0.2, 0, 0, 0, 0, 0}},
})
```

Ending the session, demonstrating the retry semantics:

```go
for {
    callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
    resp, err = motionService.DoCommand(callCtx, map[string]interface{}{builtin.DoStreamFlush: true})
    cancel()
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
            continue // session is still draining; retry
        }
        return err
    }
    if running, _ := resp["running"].(bool); !running {
        break
    }
}
```

## Part 2: Architecture

### Data flow

```
 client                    services/motion/builtin              streaming package                          arm
--------  DoCommand RPCs  ------------------------  unbuffered  -----------------------------  bidi RPC  -----
 push  ----------------->  handlers (stream.go)  --- jpCh --->  Run select loop (coordinator.go)
                                                                   |            ^
                                                                extend        sample
                                                                   v            |
                                                                trajex TOTG session (trajex_session.go)
                                                                   |
                                                                PVAT batches
                                                                   v
                                                                armStream (arm_stream.go)  ------------->  MoveThroughJointPositionsStreamed
```

`stream.go` (package `builtin`) adapts the unary `DoCommand` interface into a session: it
parses requests, owns the session's lifecycle state, and passes waypoints to the
background goroutine over an unbuffered channel (`jpCh`). `streaming.Run`
(`coordinator.go`) is that goroutine: it feeds waypoints into a
[trajex](https://github.com/viam-modules/trajex) streaming TOTG (time-optimal trajectory
generation) session (`trajex_session.go`), samples out PVAT
(position/velocity/acceleration/time) points, and sends them in batches through
`armStream` (`arm_stream.go`), which owns the `MoveThroughJointPositionsStreamed` RPC to
the arm.

### The coordinator loop

`Run` is a single goroutine with one `select` over three cases: context cancellation
(abort), a new waypoint arriving on `jpCh`, and a ticker firing every
`send_to_arm_interval_ms`. A waypoint extends the trajex session's path; a tick computes
the runway deficit (`target_runway_in_arm_ms` minus the current estimate of what the arm
still has buffered) and, if positive, samples exactly that much trajectory out of trajex
and sends it to the arm. When `jpCh` closes (flush), no more waypoints can pivot the
remaining trajectory, so the loop drains all of it to the arm in runway-sized batches and
then waits out the runway estimate before returning.

The single-goroutine design means the in-arm runway is the only buffer in the pipeline:

- The **arm backpressures sampling** — the loop only samples out of trajex enough to keep
  the arm's runway topped up.
- **Trajex does not backpressure the client.** A client that pushes waypoints faster than
  the arm executes them accumulates trajectory inside the trajex session (unbounded). A
  client that pushes *slower* than the arm executes starves the loop of PVAT points to
  send, and the arm will typically fault, depending on the arm implementation.

Pushes block on the unbuffered `jpCh` until the loop accepts each target, and `opMu` in
`stream.go` serializes pushes and flush — the unary `DoCommand` interface is being used to
simulate a stream, and the mutex rejects the interleavings a real stream couldn't express.

### Runway management

The runway estimate (`armStream.currentEstimatedRunwayInArm`) is open-loop dead
reckoning: the trajectory-clock time of the last PVAT point sent, minus the wall-clock
time since the first batch was sent. It assumes the arm starts executing the moment the
first batch is accepted and plays the trajectory back in real time. There is no feedback
from the arm — `arm.Response` carries no fields today — so an arm that buffers without
executing, pauses, or replays at the wrong rate will make the estimate drift, causing the
loop to over- or under-fill the arm's buffer.

### Build gating

`coordinator.go` and `trajex_session.go` carry `//go:build !windows && !no_cgo &&
viam_rdk_cgo_have_cxx20_rt`; `coordinator_nocgo.go` provides a `Run` stub for every other
build that fails with the error quoted in Part 1. `options.go`, `types.go`, and
`arm_stream.go` are deliberately untagged so that the options and the arm-facing layer
build and test everywhere. Build with the tag via
`GO_BUILD_TAGS_EXTRA=viam_rdk_cgo_have_cxx20_rt` (see the Makefile); CI sets it in
`etc/test.sh`.

Because `stream_start` only spawns the session goroutine, the nocgo stub's error is not
returned by `stream_start` itself — it surfaces asynchronously, like any other session
error (see Part 1).

### Testing map

| File | Build tag | Covers |
|---|---|---|
| `../stream_test.go` | trajex tag | Full DoCommand paths: happy path, misuse errors, abort teardown, sequential-op enforcement, batch pushes. |
| `coordinator_test.go` | trajex tag | `Run` end-to-end against a mock arm: drain-on-close, context cancellation, arm errors. |
| `trajex_session_test.go` | trajex tag | Seeding, extending, sampling, and dedup behavior of the trajex wrapper. |
| `arm_stream_test.go` | none | Batch conversion/sending and the runway estimate. |
| `options_test.go` | none | Defaults and validation. |

("trajex tag" = `!windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt`.) Run the tagged
tests locally with:

```
go test -tags viam_rdk_cgo_have_cxx20_rt ./services/motion/builtin/...
```

### Known limitations and TODOs

- The runway estimate is open-loop; there is no execution feedback from the arm.
- Velocity/acceleration limits and the send interval are caller-supplied; they should
  come from the arm's properties API (TODOs in `options.go`).
- One streaming session at a time per motion service.
- The fake and sim arms implement the RPC but ignore per-point `Time` and `Constraints`,
  so timing behavior on them is not representative.
- No Windows or `no_cgo` support.
- `Reconfigure` does not abort a running session; only `Close` does.
