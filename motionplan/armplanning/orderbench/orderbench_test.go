package orderbench

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestTagLabels(t *testing.T) {
	labels := tagLabels("tag=step_opening_fridge/tag=motion_open_door/tag=planning_success/20260828_153733.046_carry.json")
	test.That(t, labels["step"], test.ShouldEqual, "opening_fridge")
	test.That(t, labels["motion"], test.ShouldEqual, "open_door")
	test.That(t, labels["planning"], test.ShouldEqual, "success")
}

func TestIngestOrdersByRecordedTime(t *testing.T) {
	exportDir := t.TempDir()
	order := "test-order"

	// Written in an order that is neither recorded order nor alphabetical by step, so a walk that
	// forgot to sort would not accidentally pass.
	writeCapture(t, exportDir, order, "serving", "carry", "20260828_153900.100")
	writeCapture(t, exportDir, order, "grinding", "move", "20260828_153126.083")
	writeCapture(t, exportDir, order, "tamping", "move", "20260828_153143.958")

	// A video capture shares the order tag but is not a plan; ingest must not pick it up.
	videoDir := filepath.Join(exportDir, "data", "video-upload")
	test.That(t, os.MkdirAll(videoDir, 0o750), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(videoDir, "20260828_153200.000_clip.mp4"), []byte("not json"), 0o600), test.ShouldBeNil)

	dstDir := filepath.Join(t.TempDir(), "corpus")
	manifest, err := Ingest(exportDir, order, dstDir, "motionplan/order-replay/test")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(manifest.Entries), test.ShouldEqual, 3)

	test.That(t, manifest.Entries[0].Step, test.ShouldEqual, "grinding")
	test.That(t, manifest.Entries[1].Step, test.ShouldEqual, "tamping")
	test.That(t, manifest.Entries[2].Step, test.ShouldEqual, "serving")
	for i, entry := range manifest.Entries {
		test.That(t, entry.Index, test.ShouldEqual, i)
		test.That(t, strings.HasPrefix(entry.Name(), entry.File[:3]), test.ShouldBeTrue)
	}

	// The manifest must round-trip and validate against what was written.
	reloaded, err := LoadManifest(dstDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reloaded.Order, test.ShouldEqual, order)
	test.That(t, reloaded.Verify(dstDir), test.ShouldBeNil)
}

func TestVerifyRejectsDriftedCorpus(t *testing.T) {
	exportDir := t.TempDir()
	writeCapture(t, exportDir, "o", "grinding", "move", "20260828_153126.083")

	dstDir := filepath.Join(t.TempDir(), "corpus")
	manifest, err := Ingest(exportDir, "o", dstDir, "")
	test.That(t, err, test.ShouldBeNil)

	payload := filepath.Join(dstDir, manifest.Entries[0].File)
	test.That(t, os.WriteFile(payload, []byte(`{"changed": true}`), 0o600), test.ShouldBeNil)

	err = manifest.Verify(dstDir)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not match manifest")
}

func TestRunAcceptsAManifestForADirectoryWithoutOne(t *testing.T) {
	exportDir := t.TempDir()
	writeCapture(t, exportDir, "o", "grinding", "move", "20260828_153126.083")

	dstDir := filepath.Join(t.TempDir(), "corpus")
	manifest, err := Ingest(exportDir, "o", dstDir, "")
	test.That(t, err, test.ShouldBeNil)

	// A corpus resolved from the artifact store has no manifest of its own, by design. Passing the
	// manifest explicitly has to be enough, or every caller that already resolved one would have to
	// write a stray file into the artifact cache to use it.
	test.That(t, os.Remove(filepath.Join(dstDir, ManifestName)), test.ShouldBeNil)

	_, err = Run(context.Background(), Options{CorpusDir: dstDir, Manifest: manifest, Limit: 1})
	// The capture is a stub, so planning cannot succeed -- but resolution must get far enough to
	// try, rather than failing on the missing manifest.
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldNotContainSubstring, ManifestName)
}

func TestSummarizeTakesMedianAcrossPasses(t *testing.T) {
	records := []Record{
		{Name: "000/a/move", PlanMS: 100, TrajectoryLen: 10, TotalL2: 1},
		{Name: "000/a/move", PlanMS: 900, TrajectoryLen: 10, TotalL2: 1},
		{Name: "000/a/move", PlanMS: 110, TrajectoryLen: 10, TotalL2: 1},
	}
	stats := Summarize(records)
	test.That(t, stats["000/a/move"].Passes, test.ShouldEqual, 3)
	// The 900ms outlier must not drag the estimate; that is the whole point of a median.
	test.That(t, stats["000/a/move"].PlanMS, test.ShouldEqual, 110)
}

func TestSummarizeExcludesFailedPlansFromQuality(t *testing.T) {
	records := []Record{
		{Name: "000/a/move", PlanMS: 100, TrajectoryLen: 40, TotalL2: 2},
		{Name: "000/a/move", PlanMS: 120, Err: "timed out"},
	}
	stats := Summarize(records)
	test.That(t, stats["000/a/move"].Failures, test.ShouldEqual, 1)
	test.That(t, stats["000/a/move"].FirstErr, test.ShouldEqual, "timed out")
	// A failure contributes no trajectory, so the quality figures come from the successful pass.
	test.That(t, stats["000/a/move"].TrajectoryLen, test.ShouldEqual, 40)
}

func TestCompareFlagsRegressionsAndFailures(t *testing.T) {
	base := []Record{
		{Mode: "warm", Index: 0, Name: "000/a/move", PlanMS: 1000, TrajectoryLen: 100, TotalL2: 3},
		{Mode: "warm", Index: 1, Name: "001/b/move", PlanMS: 500, TrajectoryLen: 50, TotalL2: 2},
		{Mode: "warm", Index: 2, Name: "002/c/move", PlanMS: 200, TrajectoryLen: 20, TotalL2: 1},
	}
	head := []Record{
		{Mode: "warm", Index: 0, Name: "000/a/move", PlanMS: 6000, TrajectoryLen: 140, TotalL2: 4},
		{Mode: "warm", Index: 1, Name: "001/b/move", PlanMS: 480, TrajectoryLen: 50, TotalL2: 2},
		{Mode: "warm", Index: 2, Name: "002/c/move", PlanMS: 210, Err: "no path found"},
	}

	report := Compare(base, head, "v1.5.0", "main")
	test.That(t, report.Mode, test.ShouldEqual, "warm")

	regressions := report.Regressions()
	test.That(t, len(regressions), test.ShouldEqual, 1)
	test.That(t, regressions[0].Name, test.ShouldEqual, "000/a/move")
	test.That(t, regressions[0].Ratio, test.ShouldEqual, 6.0)
	test.That(t, regressions[0].TrajectoryRatio, test.ShouldEqual, 1.4)

	failures := report.NewFailures()
	test.That(t, len(failures), test.ShouldEqual, 1)
	test.That(t, failures[0].Name, test.ShouldEqual, "002/c/move")

	verdict := report.Verdict()
	test.That(t, verdict, test.ShouldNotBeNil)
	test.That(t, verdict.Error(), test.ShouldContainSubstring, "newly failing")
	test.That(t, verdict.Error(), test.ShouldContainSubstring, "order total")

	markdown := report.Markdown()
	test.That(t, markdown, test.ShouldContainSubstring, "v1.5.0")
	test.That(t, markdown, test.ShouldContainSubstring, "000/a/move")
	test.That(t, markdown, test.ShouldContainSubstring, "no path found")
}

func TestCompareIgnoresRatiosOnTrivialPlans(t *testing.T) {
	// A 3ms plan going to 9ms is a 3x that says nothing about the planner.
	base := []Record{{Mode: "warm", Name: "000/a/move", PlanMS: 3, TrajectoryLen: 2}}
	head := []Record{{Mode: "warm", Name: "000/a/move", PlanMS: 9, TrajectoryLen: 2}}

	report := Compare(base, head, "base", "head")
	test.That(t, len(report.Regressions()), test.ShouldEqual, 0)
}

func TestCompareCleanRunHasNoVerdict(t *testing.T) {
	base := []Record{
		{Mode: "warm", Index: 0, Name: "000/a/move", PlanMS: 1000, TrajectoryLen: 100},
		{Mode: "warm", Index: 1, Name: "001/b/move", PlanMS: 500, TrajectoryLen: 50},
	}
	head := []Record{
		{Mode: "warm", Index: 0, Name: "000/a/move", PlanMS: 1010, TrajectoryLen: 100},
		{Mode: "warm", Index: 1, Name: "001/b/move", PlanMS: 495, TrajectoryLen: 50},
	}
	report := Compare(base, head, "base", "head")
	test.That(t, report.Verdict(), test.ShouldBeNil)
	test.That(t, report.Markdown(), test.ShouldContainSubstring, "No individual plan moved")
}

func TestRecordsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.ndjson")
	records := []Record{
		{Mode: "cold", Index: 0, Name: "000/a/move", PlanMS: 1.5},
		{Mode: "cold", Index: 1, Name: "001/b/move", PlanMS: 2.5, Err: "boom"},
	}
	test.That(t, WriteRecords(path, records), test.ShouldBeNil)

	got, err := ReadRecords(path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResemble, records)
}

func TestOptionsDefaults(t *testing.T) {
	opts := Options{}.WithDefaults()
	test.That(t, opts.Mode, test.ShouldEqual, ModeWarm)
	test.That(t, opts.Repeat, test.ShouldEqual, 1)
	test.That(t, opts.GCPercent, test.ShouldEqual, defaultGCPercent)
	test.That(t, opts.Logger, test.ShouldNotBeNil)

	// An explicit zero GC target is not reachable through the options; that is deliberate, since an
	// unpinned GC target is the difference a cross-revision comparison would silently absorb.
	opts = Options{Mode: ModeCold, Repeat: 3, GCPercent: 100}.WithDefaults()
	test.That(t, opts.Mode, test.ShouldEqual, ModeCold)
	test.That(t, opts.Repeat, test.ShouldEqual, 3)
	test.That(t, opts.GCPercent, test.ShouldEqual, 100)
}

func TestEntryFromEnvRoundTrip(t *testing.T) {
	entry := Entry{Index: 7, File: "007-x.json", Step: "brewing", Motion: "move", RecordedAt: time.Now().UTC().Truncate(time.Millisecond)}
	data, err := json.Marshal(entry)
	test.That(t, err, test.ShouldBeNil)
	t.Setenv(subprocessEnvEntry, string(data))

	got, err := EntryFromEnv()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResemble, entry)
	test.That(t, got.Name(), test.ShouldEqual, "007/brewing/move")
}

// writeCapture lays down one file in the nested `tag=` layout that `viam data export binary`
// produces, with just enough content for the ingest to treat it as a plan capture.
func writeCapture(t *testing.T, exportDir, order, step, motion, stamp string) {
	t.Helper()
	dir := filepath.Join(exportDir, "data", "tag="+order, "tag=step_"+step, "tag=motion_"+motion, "tag=planning_success")
	test.That(t, os.MkdirAll(dir, 0o750), test.ShouldBeNil)
	path := filepath.Join(dir, stamp+"_"+motion+".json")
	body := `{"step":"` + step + `"}`
	test.That(t, os.WriteFile(path, []byte(body), 0o600), test.ShouldBeNil)
}
