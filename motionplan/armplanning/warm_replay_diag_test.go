package armplanning

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

// TestDiagWarmReplay replays the captured order's 98 plan requests in
// chronological order inside one process (the shape viam-server runs in) and
// prints per-plan wall time. Diagnostic only; skipped unless ORDER_EXPORT_DIR
// is set. WARM_PASSES controls how many passes run (default 1).
func TestDiagWarmReplay(t *testing.T) {
	dir := os.Getenv("ORDER_EXPORT_DIR")
	if dir == "" {
		t.Skip("set ORDER_EXPORT_DIR to the captured order's data directory")
	}
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(path) == ".json" {
			files = append(files, path)
		}
		return nil
	})
	test.That(t, err, test.ShouldBeNil)
	sort.Slice(files, func(i, j int) bool { return filepath.Base(files[i]) < filepath.Base(files[j]) })

	passes := 1
	if v := os.Getenv("WARM_PASSES"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &passes); err != nil {
			t.Fatalf("bad WARM_PASSES %q: %v", v, err)
		}
	}

	logger := logging.NewBlankLogger("replay")
	ctx := context.Background()

	for pass := 0; pass < passes; pass++ {
		var total time.Duration
		fails := 0
		for _, f := range files {
			req, _, err := ReadRequestAndResponseFromFile(f)
			test.That(t, err, test.ShouldBeNil)
			// The capture path saves planner_options without re-serializing
			// defaults; a missing collision_buffer_mm must not decode to an
			// exact 0 for this experiment (matches the prior benchmarks, which
			// kept the saved value).
			start := time.Now()
			_, _, err = PlanMotion(ctx, logger, req)
			el := time.Since(start)
			total += el
			status := "ok"
			if err != nil {
				fails++
				status = "FAIL: " + err.Error()
			}
			name := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(f)))) + "/" + filepath.Base(f)
			fmt.Printf("pass=%d %8.1fms %s %s\n", pass, float64(el.Microseconds())/1000.0, name, status)
		}
		fmt.Printf("PASS %d TOTAL: %.2fs (%d plans, %d failures)\n", pass, total.Seconds(), len(files), fails)
		roadmapRegistry.Range(func(k, v any) bool {
			rm := v.(*roadmap)
			verdicts := 0
			rm.sceneVerdicts.Range(func(_, _ any) bool { verdicts++; return true })
			scenes := map[uint64]bool{}
			rm.sceneVerdicts.Range(func(vk, _ any) bool {
				scenes[vk.(roadmapVerdictKey).scene] = true
				return true
			})
			rm.mu.RLock()
			fmt.Printf("  roadmap %q: %d nodes (600 sampled), %d scene verdicts across %d scenes\n",
				k.(string), len(rm.flat), verdicts, len(scenes))
			rm.mu.RUnlock()
			return true
		})
	}
}
