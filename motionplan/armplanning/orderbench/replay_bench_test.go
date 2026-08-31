//go:build !386

package orderbench

import (
	"context"
	"testing"
	"time"

	"go.viam.com/rdk/motionplan/armplanning"
)

// BenchmarkOrderReplay replays the whole captured order. It is the local equivalent of what the
// Motion Order Benchmark workflow runs, and the number that matters is the reported order_ms rather
// than the ns/op Go prints -- one iteration is a whole order, not a single operation.
//
//	go test ./motionplan/armplanning/orderbench -run xxx -bench OrderReplay -benchtime 1x
//
// To compare two revisions, use the CLI rather than benchstat: it builds the harness against both
// libraries and joins plans by name.
func BenchmarkOrderReplay(b *testing.B) {
	benchmarkOrder(b, ModeWarm)
}

// BenchmarkOrderReplayCold plans each request in a fresh process, isolating the per-plan floor from
// anything that amortises across the order. It forks once per plan, so it is much slower than the
// warm replay.
func BenchmarkOrderReplayCold(b *testing.B) {
	benchmarkOrder(b, ModeCold)
}

func benchmarkOrder(b *testing.B, mode Mode) {
	if armplanning.IsTooSmallForCache() {
		b.Skip("machine is too small for the smart-seed cache; timings would not be comparable")
	}

	corpusDir, manifest, err := ResolveCorpusDir("", DefaultCorpus)
	if err != nil {
		// The corpus is a ~100MB artifact fetch. Skipping keeps `go test -bench .` usable on a
		// machine that cannot reach the artifact store.
		b.Skipf("corpus %q unavailable: %v", DefaultCorpus, err)
	}
	b.Logf("replaying %d plans from %s", len(manifest.Entries), corpusDir)

	opts := Options{
		CorpusDir:   corpusDir,
		Manifest:    manifest,
		Mode:        mode,
		PlanTimeout: 60 * time.Second,
	}

	var last []Record
	b.ResetTimer()
	for b.Loop() {
		last, err = Run(context.Background(), opts)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

	var totalMS float64
	var failures int
	for _, stat := range Summarize(last) {
		totalMS += stat.PlanMS
		if stat.Failures > 0 {
			failures++
		}
	}
	b.ReportMetric(totalMS, "order_ms")
	b.ReportMetric(float64(failures), "failed_plans")
}
