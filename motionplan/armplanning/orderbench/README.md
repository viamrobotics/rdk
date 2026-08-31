# orderbench — motion planning order replay

Replays a recorded sequence of production plan requests — one full robot order, in the order the
robot actually planned them — and compares planner latency and trajectory quality across two RDK
revisions.

## Why a sequence, and not a set of plans

`armplanning` carries state between calls inside a process: learned roadmaps, the smart-seed cache,
and a process-wide registry of static-scene distance fields. What a plan costs therefore depends on
which plans ran before it. A benchmark built from independent requests cannot see that, and it is
where real regressions have hidden — the same plan can be 0.07x or 5x depending on what the roadmap
learned earlier in the order.

So the corpus is ordered by the capture timestamp in each source filename, and the default replay
runs the whole order in one process.

## The two modes measure different things

| mode   | what it does                                            | what it tells you                                       |
| ------ | ------------------------------------------------------- | ------------------------------------------------------- |
| `warm` | one process, plans in recorded order, caches accumulate | what production experiences over a shift                |
| `cold` | a fresh forked process per plan                         | the per-plan floor — preprocessing that never amortises |

They routinely disagree, which is the point. On the current corpus, the first few plans run 10–24 ms
warm and 145–156 ms cold: that gap is scene preprocessing that a long-lived process pays once.

Cold mode forks because the state that has to be discarded is process-global; there is nothing to
reset from inside the process.

## Usage

Replay against the current checkout, pulling the corpus from the artifact store:

```bash
go run ./motionplan/armplanning/orderbench/cmd/orderbench fetch
go run ./motionplan/armplanning/orderbench/cmd/orderbench run -mode warm -out head.ndjson
```

Compare two revisions. The harness must be the *same* on both sides or you are comparing harnesses
as well as libraries, so build the head harness against the base library:

```bash
git worktree add /tmp/rdk-base <base-sha>
rm -rf /tmp/rdk-base/motionplan/armplanning/orderbench
cp -r motionplan/armplanning/orderbench /tmp/rdk-base/motionplan/armplanning/orderbench

go build -o /tmp/orderbench-head ./motionplan/armplanning/orderbench/cmd/orderbench
(cd /tmp/rdk-base && go build -o /tmp/orderbench-base ./motionplan/armplanning/orderbench/cmd/orderbench)

# Interleave the passes: a machine that drifts mid-run would otherwise charge it all to one side.
for pass in 1 2; do
  /tmp/orderbench-base run -corpus ./corpus -mode warm -out base-$pass.ndjson
  /tmp/orderbench-head run -corpus ./corpus -mode warm -out head-$pass.ndjson
done
cat base-*.ndjson > base.ndjson; cat head-*.ndjson > head.ndjson

/tmp/orderbench-head compare -base base.ndjson -head head.ndjson \
  -base-label base -head-label head
```

The solver-attribution fields are read reflectively precisely so that the head harness still builds
against an older `PlanMeta`.

There is also a Go benchmark, which skips when the corpus cannot be fetched:

```bash
go test ./motionplan/armplanning/orderbench -run xxx -bench OrderReplay -benchtime 1x
```

## Controls that must stay pinned

Each of these has silently produced a wrong answer at least once:

- **On-disk roadmap cache.** `MOTION_ROADMAP_CACHE_DIR` is set explicitly (empty by default).
  `viam-server` never sets it, so production runs cold; left enabled, the newer revision replays
  learned corridors the older one structurally cannot have.
- **`GOGC`.** Pinned to 300 by the harness and by the workflow. `cmd-plan` sets this only on newer
  revisions, so an unpinned run measures the CLI difference rather than the library.
- **Cold versus warm is a fork, not noise.** In-process repetitions reuse the in-memory roadmap, so
  `-repeat N` measures the *warm* path getting warmer. Independent samples need separate processes —
  which is why the workflow loops the binary instead of raising `-repeat`.
- **Records go to a file, never stdout.** `smart_seed.go` holds a package-level logger that writes
  to stdout and that no caller can silence, so anything parsing stdout eventually reads a log line
  as a result.

Only `PlanMotion` is timed. Parsing and `PrepSmartSeed` are recorded separately: they are roughly
version-neutral and would dilute the signal.

## Thresholds

The report and the gate use different thresholds on purpose. Replaying the *same* build twice puts
one or two of 98 plans past 1.25x, so a build cannot fail on that.

| threshold          | default                    | gates?                                         |
| ------------------ | -------------------------- | ---------------------------------------------- |
| order total        | 1.10x                      | yes — repeat runs of one build land within ~2% |
| per-plan, listed   | 1.25x                      | no                                             |
| per-plan, gated    | 2.0x **and** ≥250 ms added | yes                                            |
| newly failing plan | any                        | yes                                            |

The absolute-time condition on the per-plan gate matters. A change that adds a flat per-plan cost to
scene preprocessing shows up as a fleet of 7x regressions on the cheapest plans in the order — true
as ratios, nearly irrelevant to how long the order takes. Those belong in the order total, and the
tables are sorted by time added rather than by ratio for the same reason.

Trajectory length is compared against the plan the robot actually executed as well as against the
base revision, so a planner that buys latency with a worse path does not read as a win.

## Adding a corpus

Export the captures for one order:

```bash
viam data export binary filter --destination ./order-export --tags <order-tag> --timeout 120
```

Then build the corpus with `orderbench.Ingest`, which flattens the export's nested `tag=`
directories, orders the plans by capture timestamp and writes the manifest. This is a rare
maintenance task rather than part of running a benchmark, so it is a library call and not a CLI
subcommand — a throwaway `main` is enough:

```go
manifest, err := orderbench.Ingest(
    "./order-export", "<order-tag>", "./corpus", "motionplan/order-replay/<name>")
```

Commit the manifest, stage the payloads, and push:

```bash
cp ./corpus/manifest.json motionplan/armplanning/orderbench/corpora/<name>.json
mkdir -p .artifact/data/motionplan/order-replay/<name>
cp ./corpus/*.json .artifact/data/motionplan/order-replay/<name>/
rm .artifact/data/motionplan/order-replay/<name>/manifest.json
artifact push   # needs ARTIFACT_GOOGLE_APPLICATION_CREDENTIALS
```

The export mixes in unrelated captures that share the order tag — the current one carries 215 MB of
video alongside 107 MB of plans — which `Ingest` filters out. Payloads go to the artifact store;
only the manifest is committed, and it is what pins order, labels and content hashes. The replay
verifies every payload against it before running, because a corpus that has silently drifted would
invalidate every comparison made against it.
