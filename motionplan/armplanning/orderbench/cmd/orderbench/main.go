// Package main is the CLI for the motion order benchmark: it turns a `viam data export` tree into
// a replayable corpus, replays that corpus against the current revision, and diffs two runs.
//
//	orderbench ingest -export ./order-export -order <tag> -out ./corpus
//	orderbench run    -corpus ./corpus -mode warm -out head.ndjson
//	orderbench compare -base base.ndjson -head head.ndjson -out report.md
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/armplanning/orderbench"
)

func main() {
	if err := run(); err != nil {
		log.New(os.Stderr, "", 0).Printf("error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: orderbench <ingest|run|replay-one|compare> [flags]")
	}
	switch os.Args[1] {
	case "ingest":
		return runIngest(os.Args[2:])
	case "fetch":
		return runFetch(os.Args[2:])
	case "run":
		return runReplay(os.Args[2:])
	case "replay-one":
		return runReplayOne(os.Args[2:])
	case "compare":
		return runCompare(os.Args[2:])
	default:
		return fmt.Errorf("unknown command %q; want ingest, fetch, run, replay-one or compare", os.Args[1])
	}
}

// runFetch pulls a corpus out of the artifact store ahead of time, so that a replay's timings are
// not polluted by a first-use download.
func runFetch(args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)
	name := fs.String("name", orderbench.DefaultCorpus, "corpus manifest compiled into this binary")
	out := fs.String("out", "", "stage the corpus into this directory as symlinks (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	manifest, err := orderbench.LoadEmbeddedManifest(*name)
	if err != nil {
		return err
	}
	dir, err := orderbench.Resolve(manifest)
	if err != nil {
		return err
	}
	if *out != "" {
		if err := orderbench.StageCorpus(dir, manifest, *out); err != nil {
			return err
		}
		dir = *out
	}

	log.New(os.Stderr, "", 0).Printf("corpus %s: %d plans, %.1f MB, verified",
		manifest.Order, len(manifest.Entries), float64(manifest.TotalBytes())/(1024*1024))
	// The resolved path goes to stdout alone so a shell can capture it.
	log.New(os.Stdout, "", 0).Print(dir)
	return nil
}

func runIngest(args []string) error {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	exportDir := fs.String("export", "", "directory written by `viam data export binary filter --destination`")
	order := fs.String("order", "", "order tag the captures were filtered by")
	out := fs.String("out", "", "corpus directory to create")
	artifactPath := fs.String("artifact-path", "", "path this corpus will occupy in the artifact tree")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *exportDir == "" || *order == "" || *out == "" {
		return errors.New("ingest requires -export, -order and -out")
	}

	manifest, err := orderbench.Ingest(*exportDir, *order, *out, *artifactPath)
	if err != nil {
		return err
	}

	stdout := log.New(os.Stdout, "", 0)
	stdout.Printf("ingested %d plans (%.1f MB) into %s",
		len(manifest.Entries), float64(manifest.TotalBytes())/(1024*1024), *out)
	stdout.Printf("recorded span: %s .. %s",
		manifest.Entries[0].RecordedAt.Format(time.TimeOnly),
		manifest.Entries[len(manifest.Entries)-1].RecordedAt.Format(time.TimeOnly))
	return nil
}

type replayFlags struct {
	corpus      string
	corpusName  string
	mode        string
	repeat      int
	limit       int
	only        string
	roadmapDir  string
	gcPercent   int
	planTimeout time.Duration
	verbose     bool
}

func (f *replayFlags) bind(fs *flag.FlagSet) {
	fs.StringVar(&f.corpus, "corpus", "",
		"local corpus directory; when empty, -corpus-name is pulled from the artifact store")
	fs.StringVar(&f.corpusName, "corpus-name", orderbench.DefaultCorpus,
		"name of a compiled-in corpus manifest to resolve from the artifact store")
	fs.StringVar(&f.mode, "mode", string(orderbench.ModeWarm),
		"warm (one process, recorded order, caches accumulate) or cold (fresh process per plan)")
	fs.IntVar(&f.repeat, "repeat", 1, "replay the whole order this many times")
	fs.IntVar(&f.limit, "limit", 0, "replay only the first N plans (0 = all)")
	fs.StringVar(&f.only, "only", "", "comma-separated substrings; keep only matching plans")
	fs.StringVar(&f.roadmapDir, "roadmap-cache", "",
		"MOTION_ROADMAP_CACHE_DIR for the replay; empty disables the on-disk roadmap, matching viam-server")
	fs.IntVar(&f.gcPercent, "gc-percent", 0, "GC target to pin (0 = harness default)")
	fs.DurationVar(&f.planTimeout, "plan-timeout", 60*time.Second,
		"per-plan wall clock cap, overriding the timeout recorded in the request (0 = use recorded)")
	fs.BoolVar(&f.verbose, "v", false, "let planner logs through to stderr")
}

func (f *replayFlags) options() orderbench.Options {
	opts := orderbench.Options{
		CorpusDir:       f.corpus,
		CorpusName:      f.corpusName,
		Mode:            orderbench.Mode(f.mode),
		Repeat:          f.repeat,
		Limit:           f.limit,
		RoadmapCacheDir: f.roadmapDir,
		GCPercent:       f.gcPercent,
		PlanTimeout:     f.planTimeout,
	}
	if f.only != "" {
		opts.Only = strings.Split(f.only, ",")
	}
	if f.verbose {
		opts.Logger = logging.NewLogger("orderbench")
	}
	return opts.WithDefaults()
}

func runReplay(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	var flags replayFlags
	flags.bind(fs)
	out := fs.String("out", "", "write newline-delimited JSON records here (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("run requires -out")
	}

	opts := flags.options()
	orderbench.ApplyProcessControls(opts)

	source := opts.CorpusDir
	if source == "" {
		source = "artifact:" + opts.CorpusName
	}
	stderr := log.New(os.Stderr, "", 0)
	stderr.Printf("replaying %s in %s mode (repeat=%d, gc=%d, roadmap-cache=%q)",
		source, opts.Mode, opts.Repeat, opts.GCPercent, opts.RoadmapCacheDir)

	start := time.Now()
	records, err := orderbench.Run(context.Background(), opts)
	if err != nil {
		return err
	}
	if err := orderbench.WriteRecords(*out, records); err != nil {
		return err
	}

	stats := orderbench.Summarize(records)
	var total float64
	var failures int
	for _, s := range stats {
		total += s.PlanMS
		if s.Failures > 0 {
			failures++
		}
	}
	stderr.Printf("replayed %d plans in %s wall clock; planning total %.2fs, %d failed",
		len(stats), time.Since(start).Round(time.Millisecond), total/1000, failures)
	return nil
}

// runReplayOne plans a single request in this fresh process and writes its record out. Cold mode
// spawns this once per plan.
func runReplayOne(args []string) error {
	fs := flag.NewFlagSet("replay-one", flag.ExitOnError)
	var flags replayFlags
	flags.bind(fs)
	recordOut := fs.String("record-out", "", "write the single JSON record here (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("replay-one takes exactly one plan file")
	}
	if *recordOut == "" {
		return errors.New("replay-one requires -record-out")
	}

	entry, err := orderbench.EntryFromEnv()
	if err != nil {
		return err
	}

	opts := flags.options()
	opts.Mode = orderbench.ModeWarm // the child plans in-process; it *is* the fresh process
	orderbench.ApplyProcessControls(opts)

	record, err := orderbench.PlanOne(context.Background(), fs.Arg(0), entry, opts)
	if err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return os.WriteFile(*recordOut, data, 0o600)
}

func runCompare(args []string) error {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	basePath := fs.String("base", "", "records from the baseline revision")
	headPath := fs.String("head", "", "records from the revision under test")
	baseLabel := fs.String("base-label", "base", "label for the baseline in the report")
	headLabel := fs.String("head-label", "head", "label for the head revision in the report")
	out := fs.String("out", "", "write the markdown report here (default stdout)")
	jsonOut := fs.String("json-out", "", "also write the full report as JSON here")
	planThreshold := fs.Float64("plan-threshold", orderbench.DefaultRegressionThreshold,
		"per-plan slowdown ratio at which a plan is listed in the report")
	gateThreshold := fs.Float64("gate-threshold", orderbench.DefaultGateThreshold,
		"per-plan slowdown ratio that fails the build")
	totalThreshold := fs.Float64("total-threshold", orderbench.DefaultTotalThreshold,
		"whole-order slowdown ratio that fails the build")
	failOnRegression := fs.Bool("fail-on-regression", false, "exit non-zero when the verdict is a regression")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *basePath == "" || *headPath == "" {
		return errors.New("compare requires -base and -head")
	}

	base, err := orderbench.ReadRecords(*basePath)
	if err != nil {
		return err
	}
	head, err := orderbench.ReadRecords(*headPath)
	if err != nil {
		return err
	}

	report := orderbench.Compare(base, head, *baseLabel, *headLabel)
	report.PlanThreshold = *planThreshold
	report.GateThreshold = *gateThreshold
	report.TotalThreshold = *totalThreshold

	markdown := report.Markdown()
	if *out == "" {
		log.New(os.Stdout, "", 0).Print(markdown)
	} else if err := os.WriteFile(*out, []byte(markdown), 0o600); err != nil {
		return err
	}

	if *jsonOut != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*jsonOut, append(data, '\n'), 0o600); err != nil {
			return err
		}
	}

	if verdict := report.Verdict(); verdict != nil && *failOnRegression {
		return verdict
	}
	return nil
}
