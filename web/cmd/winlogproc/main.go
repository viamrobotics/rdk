// Command winlogproc collects and/or processes Windows log dumps from
// viam-server's two appenders (classic Event Log and ETW), producing
// compact time<TAB>level<TAB>message TSVs.
//
// Three invocations:
//
//	winlogproc
//	    "One-button" mode. Flushes the ETW session at the default
//	    location, dumps the .etl via tracerpt, dumps Get-EventLog,
//	    and processes both. Writes to ./winlogs-<timestamp>/.
//	    Windows-only.
//
//	winlogproc <artifacts-dir>
//	    Process existing dumps. Expects <dir>/eventlog.txt and
//	    <dir>/trace.csv; writes <dir>/processed/{eventlog,trace}.tsv.
//	    Cross-platform.
//
//	winlogproc -eventlog FILE -trace FILE [-out DIR]
//	    Process specific files. Cross-platform.
//
// Override flags for collect mode: -etl, -source, -session, -out,
// -after, -before.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.viam.com/rdk/web/server/winlogproc"
)

func main() {
	// Collect-mode overrides.
	etlDir := flag.String("etl-dir", "", "directory containing per-session .etl files (default: $VIAM_HOME/logs)")
	source := flag.String("source", "", "Application Event Log source (default: viam-server)")
	session := flag.String("session", "", "ETW session name to flush before reading (default: viam-server-trace)")
	out := flag.String("out", "", "output directory (default: ./winlogs-<timestamp>)")
	afterStr := flag.String("after", "", "eventlog filter: only events at or after this RFC3339 time")
	beforeStr := flag.String("before", "", "eventlog filter: only events at or before this RFC3339 time")
	sinceStr := flag.String("since", "", "eventlog filter: only events from the last duration (e.g. 10s, 5m, 1h)")
	lastStr := flag.String("last", "", "filter: only the last duration of events anchored at the most recent log timestamp (offline-friendly version of -since); mutually exclusive with -since/-after/-before")

	// Process-only inputs.
	eventlogIn := flag.String("eventlog", "", "process this existing eventlog.txt instead of collecting")
	traceIn := flag.String("trace", "", "process this existing trace.csv instead of collecting")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s                              (collect + process from defaults)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       %s <artifacts-dir>              (process existing dumps in dir)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       %s -eventlog F -trace F [-out D]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	switch {
	case flag.NArg() == 1 && *eventlogIn == "" && *traceIn == "":
		// Process existing dumps from a directory.
		dir := flag.Arg(0)
		dst := *out
		if dst == "" {
			dst = filepath.Join(dir, "processed")
		}
		runProcess(filepath.Join(dir, "eventlog.txt"), filepath.Join(dir, "trace.csv"), dst)

	case *eventlogIn != "" || *traceIn != "":
		// Process explicit files.
		if *eventlogIn == "" || *traceIn == "" {
			fmt.Fprintln(os.Stderr, "-eventlog and -trace must be set together")
			os.Exit(2)
		}
		dst := *out
		if dst == "" {
			dst = "."
		}
		runProcess(*eventlogIn, *traceIn, dst)

	default:
		// Collect mode (default).
		opts := winlogproc.CollectOpts{
			ETLDir:         *etlDir,
			SessionName:    *session,
			EventlogSource: *source,
			OutDir:         *out,
		}

		// -last is mutually exclusive with -since/-after/-before and is
		// applied as a post-Collect filter pass (we don't know the
		// anchor until events are dumped).
		var lastDur time.Duration
		if *lastStr != "" {
			if *sinceStr != "" || *afterStr != "" || *beforeStr != "" {
				fmt.Fprintln(os.Stderr, "-last is mutually exclusive with -since/-after/-before")
				os.Exit(2)
			}
			d, derr := time.ParseDuration(*lastStr)
			if derr != nil {
				fmt.Fprintln(os.Stderr, "parse -last:", derr)
				os.Exit(2)
			}
			lastDur = d
		}

		var err error
		if *sinceStr != "" {
			if *afterStr != "" {
				fmt.Fprintln(os.Stderr, "-since and -after are mutually exclusive")
				os.Exit(2)
			}
			d, derr := time.ParseDuration(*sinceStr)
			if derr != nil {
				fmt.Fprintln(os.Stderr, "parse -since:", derr)
				os.Exit(2)
			}
			opts.After = time.Now().Add(-d)
		} else if *afterStr != "" {
			opts.After, err = time.Parse(time.RFC3339, *afterStr)
			if err != nil {
				fmt.Fprintln(os.Stderr, "parse -after:", err)
				os.Exit(2)
			}
		}
		if *beforeStr != "" {
			opts.Before, err = time.Parse(time.RFC3339, *beforeStr)
			if err != nil {
				fmt.Fprintln(os.Stderr, "parse -before:", err)
				os.Exit(2)
			}
		}
		dir, err := winlogproc.Collect(opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, "collect:", err)
			os.Exit(1)
		}

		if lastDur > 0 {
			if err := applyLastFilter(dir, lastDur); err != nil {
				fmt.Fprintln(os.Stderr, "-last filter:", err)
				os.Exit(1)
			}
		}

		fmt.Println(dir)
	}
}

// applyLastFilter implements the -last flag by anchoring each stream
// at its own latest timestamp, then rewriting each TSV in place to
// drop anything older than (that stream's max - duration). Per-stream
// anchors prevent one stream's tail from gutting the other.
func applyLastFilter(dir string, dur time.Duration) error {
	for _, name := range []string{"eventlog.tsv", "trace.tsv"} {
		path := filepath.Join(dir, "processed", name)
		max, err := winlogproc.MaxTimestampInTSV(path)
		if err != nil {
			return fmt.Errorf("max ts in %s: %w", name, err)
		}
		if max.IsZero() {
			fmt.Fprintf(os.Stderr, "-last: %s has no parseable timestamps; skipping\n", name)
			continue
		}
		cutoff := max.Add(-dur)
		fmt.Fprintf(os.Stderr, "-last %s: anchor=%s cutoff=%s\n",
			name, max.Format(time.RFC3339Nano), cutoff.Format(time.RFC3339Nano))
		if err := winlogproc.FilterProcessedTSV(path, cutoff, time.Time{}); err != nil {
			return fmt.Errorf("filter %s: %w", name, err)
		}
	}
	return nil
}

func runProcess(eventlogIn, traceIn, outDir string) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir out:", err)
		os.Exit(1)
	}
	exit := 0
	if err := winlogproc.Eventlog(eventlogIn, filepath.Join(outDir, "eventlog.tsv")); err != nil {
		fmt.Fprintln(os.Stderr, "eventlog:", err)
		exit = 1
	}
	if err := winlogproc.Trace(traceIn, filepath.Join(outDir, "trace.tsv"), time.Time{}, time.Time{}); err != nil {
		fmt.Fprintln(os.Stderr, "trace:", err)
		exit = 1
	}
	fmt.Println(outDir)
	os.Exit(exit)
}
