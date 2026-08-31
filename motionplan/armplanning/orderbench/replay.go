package orderbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
)

// defaultGCPercent matches what cmd-plan (and therefore the planner's tuned behaviour) uses. It is
// set explicitly rather than inherited so that a GOGC difference between two revisions' CLIs
// cannot masquerade as a planner change.
const defaultGCPercent = 300

// Options configures a replay. Every field that can move a timing number is explicit: a
// cross-revision comparison is only meaningful if both sides ran with an identical configuration.
//
// The whole order is replayed in a single process, in recorded order, so learned roadmaps, the
// smart-seed cache and the static-scene SDF registry accumulate exactly as they do during a real
// shift. That is deliberate rather than incidental: it is the shape production runs, and a plan's
// cost depends on which plans preceded it.
type Options struct {
	// CorpusDir holds the payloads and the manifest. Leave empty to resolve CorpusName from the
	// artifact store instead.
	CorpusDir string
	// CorpusName selects one of the manifests compiled into the binary. Ignored when CorpusDir is
	// set.
	CorpusName string
	// Manifest, when set, is used as-is instead of being loaded from CorpusDir. A directory
	// resolved from the artifact store deliberately holds no manifest of its own -- writing one
	// there would put a stray file in the artifact cache -- so a caller that has already resolved
	// passes what it got back through here.
	Manifest *Manifest
	// Repeat runs the whole order this many times. Passes are recorded separately so the
	// comparator can take a median and shrug off a noisy runner.
	Repeat int
	// Limit truncates the order to its first N plans. Zero replays all of them.
	Limit int
	// Only, when non-empty, keeps just the plans whose name contains one of these substrings.
	Only []string
	// RoadmapCacheDir is the on-disk roadmap cache. Empty disables it, which is what viam-server
	// does; the disk cache would otherwise let one side of a comparison replay corridors the other
	// structurally cannot have.
	RoadmapCacheDir string
	// GCPercent is the GC target applied before planning. Zero means defaultGCPercent.
	GCPercent int
	// PlanTimeout caps each individual plan. Captured requests carry a 300s planner timeout, which
	// is far too long to let a pathological regression burn a CI budget. Zero leaves the recorded
	// timeout alone.
	PlanTimeout time.Duration
	Logger      logging.Logger
}

// WithDefaults fills in the fields that were left zero. Callers that report their configuration
// should normalize through this first, so what they print is what actually ran.
func (o Options) WithDefaults() Options {
	if o.Repeat <= 0 {
		o.Repeat = 1
	}
	if o.GCPercent == 0 {
		o.GCPercent = defaultGCPercent
	}
	if o.Logger == nil {
		o.Logger = logging.NewBlankLogger("orderbench")
	}
	return o
}

// Record is one plan's result. Field names are the join keys between two runs, so they are part of
// the file format and should only be added to.
type Record struct {
	Pass   int    `json:"pass"`
	Index  int    `json:"index"`
	Name   string `json:"name"`
	Step   string `json:"step"`
	Motion string `json:"motion"`

	// PlanMS times armplanning.PlanMotion alone. Parsing and smart-seed preparation are recorded
	// separately because they are not what a planner change is expected to move.
	PlanMS  float64 `json:"plan_ms"`
	ParseMS float64 `json:"parse_ms"`
	PrepMS  float64 `json:"prep_ms"`

	TrajectoryLen int     `json:"trajectory_len"`
	PathLen       int     `json:"path_len"`
	TotalL2       float64 `json:"total_l2"`

	// RecordedTrajectoryLen and RecordedTotalL2 come from the plan the robot actually executed,
	// giving every plan a quality baseline that does not depend on either revision under test.
	RecordedTrajectoryLen int     `json:"recorded_trajectory_len"`
	RecordedTotalL2       float64 `json:"recorded_total_l2"`

	// Solver attribution, straight off PlanMeta. A latency change with no matching shift here is a
	// different story from one where work moved between the roadmap and the search.
	GoalsProcessed     int `json:"goals_processed"`
	GoalsCBIRRTSolved  int `json:"goals_cbirrt_solved"`
	GoalsNudgeSolved   int `json:"goals_nudge_solved"`
	GoalsRoadmapSolved int `json:"goals_roadmap_solved"`

	HeapAllocMB float64 `json:"heap_alloc_mb"`
	Partial     bool    `json:"partial"`
	Err         string  `json:"err,omitempty"`
}

// Failed reports whether the plan did not produce a usable trajectory.
func (r Record) Failed() bool {
	return r.Err != ""
}

// Run replays the corpus and returns one Record per plan per pass.
func Run(ctx context.Context, opts Options) ([]Record, error) {
	o := opts.WithDefaults()

	corpusDir, manifest := o.CorpusDir, o.Manifest
	if manifest == nil {
		var err error
		if corpusDir, manifest, err = ResolveCorpusDir(o.CorpusDir, o.CorpusName); err != nil {
			return nil, err
		}
	} else if err := manifest.Verify(corpusDir); err != nil {
		return nil, err
	}
	o.CorpusDir = corpusDir

	entries := selectEntries(manifest.Entries, o)
	if len(entries) == 0 {
		return nil, errors.New("no corpus entries selected")
	}

	records := make([]Record, 0, len(entries)*o.Repeat)
	for pass := range o.Repeat {
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return records, err
			}
			rec, err := planOne(ctx, filepath.Join(o.CorpusDir, entry.File), entry, o)
			if err != nil {
				return records, err
			}
			rec.Pass = pass
			records = append(records, rec)
		}
	}
	return records, nil
}

func selectEntries(all []Entry, o Options) []Entry {
	var out []Entry
	for _, e := range all {
		if len(o.Only) > 0 && !matchesAny(e.Name(), o.Only) {
			continue
		}
		out = append(out, e)
		if o.Limit > 0 && len(out) == o.Limit {
			break
		}
	}
	return out
}

func matchesAny(name string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(name, p) {
			return true
		}
	}
	return false
}

func planOne(ctx context.Context, path string, entry Entry, o Options) (Record, error) {
	rec := Record{
		Index:  entry.Index,
		Name:   entry.Name(),
		Step:   entry.Step,
		Motion: entry.Motion,
	}

	parseStart := time.Now()
	req, recorded, err := armplanning.ReadRequestAndResponseFromFile(path)
	rec.ParseMS = msSince(parseStart)
	if err != nil {
		return rec, fmt.Errorf("reading %q: %w", path, err)
	}
	if recorded != nil {
		rec.RecordedTrajectoryLen = len(recorded.Trajectory())
		rec.RecordedTotalL2 = trajectoryL2(recorded.Trajectory())
	}
	if o.PlanTimeout > 0 {
		req.PlannerOptions.Timeout = o.PlanTimeout.Seconds()
	}

	prepStart := time.Now()
	if err := armplanning.PrepSmartSeed(req.FrameSystem, o.Logger); err != nil {
		return rec, fmt.Errorf("preparing smart seed for %q: %w", path, err)
	}
	rec.PrepMS = msSince(prepStart)

	planCtx := ctx
	if o.PlanTimeout > 0 {
		var cancel context.CancelFunc
		// The planner's own timeout is advisory; a hard deadline is what keeps one pathological
		// plan from consuming the entire run.
		planCtx, cancel = context.WithTimeout(ctx, o.PlanTimeout+planTimeoutGrace)
		defer cancel()
	}

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	planStart := time.Now()
	plan, meta, err := armplanning.PlanMotion(planCtx, o.Logger, req)
	rec.PlanMS = msSince(planStart)

	runtime.ReadMemStats(&after)
	rec.HeapAllocMB = float64(after.TotalAlloc-before.TotalAlloc) / (1024 * 1024)

	if meta != nil {
		rec.Partial = meta.Partial
		rec.GoalsProcessed = meta.GoalsProcessed
		rec.GoalsCBIRRTSolved = metaCounter(meta, "GoalsCBIRRTSolved")
		rec.GoalsNudgeSolved = metaCounter(meta, "GoalsNudgeSolved")
		rec.GoalsRoadmapSolved = metaCounter(meta, "GoalsRoadmapSolved")
	}
	if err != nil {
		// A planner failure is a result, not a harness error: the whole point is to notice when a
		// revision starts failing plans the other one solved.
		rec.Err = err.Error()
		return rec, nil
	}

	rec.TrajectoryLen = len(plan.Trajectory())
	rec.PathLen = len(plan.Path())
	rec.TotalL2 = trajectoryL2(plan.Trajectory())
	return rec, nil
}

// planTimeoutGrace lets the planner hit its own timeout and unwind before the context deadline
// fires, so a timed-out plan reports the planner's error rather than a context cancellation.
const planTimeoutGrace = 15 * time.Second

// metaCounter reads an integer counter off PlanMeta by name, yielding zero when the field does not
// exist on this revision.
//
// This harness is built to be compiled against an older RDK than the one it ships with -- that is
// how a base-vs-head comparison isolates a library change from a harness change -- and PlanMeta's
// solver attribution has grown over time (GoalsRoadmapSolved arrived with the lazy roadmap). A
// direct field reference would make the harness refuse to build against exactly the revisions it
// most needs to measure.
func metaCounter(meta *armplanning.PlanMeta, field string) int {
	value := reflect.ValueOf(meta).Elem().FieldByName(field)
	if !value.IsValid() || value.Kind() != reflect.Int {
		return 0
	}
	return int(value.Int())
}

func trajectoryL2(traj motionplan.Trajectory) float64 {
	total := 0.0
	for i := 1; i < len(traj); i++ {
		for frame, inputs := range traj[i] {
			total += referenceframe.InputsL2Distance(traj[i-1][frame], inputs)
		}
	}
	return total
}

func msSince(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000.0
}

// ApplyProcessControls pins the process-wide settings that would otherwise silently differ between
// two revisions. It must run before any planning.
func ApplyProcessControls(o Options) {
	opts := o.WithDefaults()
	debug.SetGCPercent(opts.GCPercent)
	// Set rather than defaulted: an unset value means "whatever this revision's code decides",
	// which is exactly the ambiguity a comparison cannot tolerate.
	os.Setenv("MOTION_ROADMAP_CACHE_DIR", opts.RoadmapCacheDir) //nolint:errcheck
}

// WriteRecords writes records as one JSON object per line, which lets a run be appended to and
// compared without loading the whole thing as a single document.
func WriteRecords(path string, records []Record) error {
	file, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck

	enc := json.NewEncoder(file)
	for _, rec := range records {
		if err := enc.Encode(rec); err != nil {
			return err
		}
	}
	return file.Close()
}

// ReadRecords reads a file written by WriteRecords.
func ReadRecords(path string) ([]Record, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck

	var records []Record
	dec := json.NewDecoder(file)
	for dec.More() {
		var rec Record
		if err := dec.Decode(&rec); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%q contains no records", path)
	}
	return records, nil
}
