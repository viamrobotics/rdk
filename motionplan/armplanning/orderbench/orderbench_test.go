package orderbench

import (
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

func TestTagLabels(t *testing.T) {
	labels := tagLabels("tag=step_opening_fridge/tag=motion_open_door/tag=planning_success/20260828_153733.046_carry.json")
	test.That(t, labels["step"], test.ShouldEqual, "opening_fridge")
	test.That(t, labels["motion"], test.ShouldEqual, "open_door")
	test.That(t, labels["planning"], test.ShouldEqual, "success")
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
		{Index: 0, Name: "000/a/move", PlanMS: 1000, TrajectoryLen: 100, TotalL2: 3},
		{Index: 1, Name: "001/b/move", PlanMS: 500, TrajectoryLen: 50, TotalL2: 2},
		{Index: 2, Name: "002/c/move", PlanMS: 200, TrajectoryLen: 20, TotalL2: 1},
	}
	head := []Record{
		{Index: 0, Name: "000/a/move", PlanMS: 6000, TrajectoryLen: 140, TotalL2: 4},
		{Index: 1, Name: "001/b/move", PlanMS: 480, TrajectoryLen: 50, TotalL2: 2},
		{Index: 2, Name: "002/c/move", PlanMS: 210, Err: "no path found"},
	}

	report := Compare(base, head, "v1.5.0", "main")
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
	base := []Record{{Name: "000/a/move", PlanMS: 3, TrajectoryLen: 2}}
	head := []Record{{Name: "000/a/move", PlanMS: 9, TrajectoryLen: 2}}

	report := Compare(base, head, "base", "head")
	test.That(t, len(report.Regressions()), test.ShouldEqual, 0)
}

func TestCompareCleanRunHasNoVerdict(t *testing.T) {
	base := []Record{
		{Index: 0, Name: "000/a/move", PlanMS: 1000, TrajectoryLen: 100},
		{Index: 1, Name: "001/b/move", PlanMS: 500, TrajectoryLen: 50},
	}
	head := []Record{
		{Index: 0, Name: "000/a/move", PlanMS: 1010, TrajectoryLen: 100},
		{Index: 1, Name: "001/b/move", PlanMS: 495, TrajectoryLen: 50},
	}
	report := Compare(base, head, "base", "head")
	test.That(t, report.Verdict(), test.ShouldBeNil)
	test.That(t, report.Markdown(), test.ShouldContainSubstring, "No individual plan moved")
}

func TestRecordsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.ndjson")
	records := []Record{
		{Index: 0, Name: "000/a/move", PlanMS: 1.5},
		{Index: 1, Name: "001/b/move", PlanMS: 2.5, Err: "boom"},
	}
	test.That(t, WriteRecords(path, records), test.ShouldBeNil)

	got, err := ReadRecords(path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResemble, records)
}

func TestOptionsDefaults(t *testing.T) {
	opts := Options{}.WithDefaults()
	test.That(t, opts.Repeat, test.ShouldEqual, 1)
	test.That(t, opts.GCPercent, test.ShouldEqual, defaultGCPercent)
	test.That(t, opts.Logger, test.ShouldNotBeNil)

	// An explicit zero GC target is not reachable through the options; that is deliberate, since an
	// unpinned GC target is the difference a cross-revision comparison would silently absorb.
	opts = Options{Repeat: 3, GCPercent: 100}.WithDefaults()
	test.That(t, opts.Repeat, test.ShouldEqual, 3)
	test.That(t, opts.GCPercent, test.ShouldEqual, 100)
}

func TestEntryName(t *testing.T) {
	entry := Entry{Index: 7, Step: "brewing", Motion: "move"}
	test.That(t, entry.Name(), test.ShouldEqual, "007/brewing/move")
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
