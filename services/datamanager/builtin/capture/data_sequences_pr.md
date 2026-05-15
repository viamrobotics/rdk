# APP-16294: Capture control sensor creates sequences

## Summary

Extends the capture control sensor to publish "sequences" — operator-defined time
windows that group capture data from one or more resources into a single record in
the cloud. Designed for the VLA training workflow where an operator demos a robot
and wants the resulting camera/joint data grouped as a unit.

The sensor emits a list of currently-active sequences in its readings; the data
manager tracks open/closed transitions, persists them locally, and uploads via a
new `CreateSequence` RPC.

## What's added

### Sensor-facing protocol

Two new keys read from the capture control sensor's `Readings` map:

```json
{
  "capture_configs": [...],
  "sequences": [
    {
      "sequence_tags": ["walking-demo"],
      "resources": [
        {"resource_name": "camera-1", "method": "GetImages"},
        {"resource_name": "arm-1",    "method": "JointPositions"}
      ]
    }
  ]
}
```

A sequence is **open** while its entry appears in the array and **ends** when it
disappears. Multiple concurrent sequences are supported — each entry is tracked
independently by content identity (resources + tags).

### On-disk lifecycle

Sequence files live in `<captureDir>/sequences/`:

- `<id>.progseq` — written when a sequence opens; in-progress, owned by the running process
- `<id>.seq` — written when a sequence closes; ready for upload
- `failed/<id>.{progseq,seq}` — terminal failures or orphans, for operator inspection

Mirrors the `.prog` → `.capture` pattern used for data capture files.

### Cloud upload

Sync's existing worker pool now recognizes `.seq` files and dispatches them to
`CreateSequence` instead of `DataCaptureUpload`. Reuses the existing
`exponentialRetry` for transient errors. Terminal errors (`InvalidArgument`,
`PermissionDenied`, `NotFound`, `Unauthenticated`, `FailedPrecondition`) move the
file to `failed/`.

### Crash recovery

On the first call to `sync.Reconfigure`, `handleOrphanedOpenSequences` scans for
`.progseq` files left from a previous crashed process and moves them to `failed/`.
Doesn't fabricate `end_at` — operator inspection only.

### Clean shutdown

`Capture.Close` calls `flushOpenSequences`, which closes in-flight sequences and
writes them as `.seq` files. They upload on the next process start.

## File layout

```
data/sequence_file.go                                             ← schema + constants
services/datamanager/data_manager.go                              ← ResourceMethod, SequenceReading, SequencesKey
services/datamanager/builtin/builtin.go                           ← poller parses sequences and calls SetActiveSequences
services/datamanager/builtin/capture/sequence_capture_control.go  ← in-memory state + transition logic
services/datamanager/builtin/capture/sequence_files.go            ← disk writes (.progseq, .seq)
services/datamanager/builtin/sync/upload_sequence.go              ← reads .seq, calls CreateSequence, orphan recovery
services/datamanager/builtin/sync/sync.go                         ← walker dispatch + `isSequenceFile` predicates
```

Layering:

- `data` owns the file format
- `capture` owns the producer side (writes `.progseq`/`.seq`)
- `sync` owns the consumer side (reads `.seq`, uploads, cleans orphans)
- `builtin` orchestrates without knowing sequence file mechanics

## Lifecycle scenarios

| Scenario | Outcome |
|---|---|
| Graceful shutdown mid-recording | ✅ Flushed, uploaded |
| Crash mid-recording | ❌ Lost; orphan moved to `failed/` |
| Reconfigure mid-recording (sensor still present) | ✅ Continues normally |
| Sensor removed from config | ⚠️ Survives clean shutdown; lost on crash |
| Capture disabled in config | ✅ Flushed, uploaded |
| Crash mid-close (between writing `.seq` and removing `.progseq`) | ✅ Deduped on recovery |
| Crash during `CreateSequence` | ⚠️ Possible duplicate (at-least-once) |
| Network outage during upload | ✅ Retried |
| Permanent server error | ⚠️ Quarantined to `failed/` |

## Trade-offs accepted

- **`CreateSequence` is not idempotent server-side.** Rare crash-during-response
  could create a duplicate sequence record. Acceptable for v1 since duplicates
  are harmless (same content, same query results) and the failure mode is rare.
- **No `end_at` fabrication on crash recovery.** An orphan `.progseq` is moved to
  `failed/` rather than finalized with `mtime` — we prefer honest data over
  invented timestamps.
- **Reconfigure with sensor removed leaves in-memory sequences as zombies until
  the next `Close`.** Operator-correctable.

## Test plan

- [ ] Unit tests for `SetActiveSequences`: open/close transitions, same-tick
  grouping, content-identity dedup
- [ ] Unit tests for `handleOrphanedOpenSequences`: orphan vs. duplicate-`.seq` dedup
- [ ] Integration test: sensor publishes sequence → file flows through to
  `CreateSequence` call
- [ ] Manual: kill viam-server mid-recording; verify `.progseq` moves to `failed/`
  on next start
- [ ] Manual: graceful shutdown mid-recording; verify `.seq` written and uploaded
- [ ] Manual: confirm capture-disable triggers flush + upload
