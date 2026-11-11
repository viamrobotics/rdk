package mpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

// useTempPlanRoot points the server at a scratch plan root holding a directory named "seq" with
// two chained joint-space requests as 1-open.json and 2-move.json, and
// returns that directory's name relative to the new rdkRoot.
func useTempPlanRoot(t *testing.T) string {
	t.Helper()

	originalRoot, originalPlanRoot := rdkRoot, planFilesRoot
	rdkRoot = t.TempDir()
	planFilesRoot = filepath.Join(rdkRoot, "mplans")
	t.Cleanup(func() {
		rdkRoot, planFilesRoot = originalRoot, originalPlanRoot
	})

	sequenceDir := filepath.Join(planFilesRoot, "seq")
	test.That(t, os.MkdirAll(sequenceDir, 0o750), test.ShouldBeNil)

	// Joint-space goals on a bare arm: planning them is a straight line, so the test doesn't depend
	// on IK, a roadmap or any other state a recorded request would drag in.
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "arm")
	test.That(t, err, test.ShouldBeNil)

	frameSystem := referenceframe.NewEmptyFrameSystem("")
	test.That(t, frameSystem.AddFrame(model, frameSystem.World()), test.ShouldBeNil)

	configurations := [][]float64{
		{0, 0.5, -1.0, 0, 0.5, 0},
		{0.3, 0.5, -1.0, 0, 0.5, 0.3},
		{0.3, 0.7, -0.8, 0.2, 0.5, 0.3},
	}
	for index, name := range []string{"1-open.json", "2-move.json"} {
		request := &armplanning.PlanRequest{
			FrameSystem: frameSystem,
			StartState:  armplanning.NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": configurations[index]}),
			Goals: []*armplanning.PlanState{
				armplanning.NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": configurations[index+1]}),
			},
			PlannerOptions: armplanning.NewBasicPlannerOptions(),
		}
		test.That(t, request.WriteToFile(filepath.Join(sequenceDir, name)), test.ShouldBeNil)
	}

	// Neither of these is a plan file.
	test.That(t, os.WriteFile(filepath.Join(sequenceDir, "notes.txt"), []byte("hello"), 0o600), test.ShouldBeNil)
	test.That(t, os.MkdirAll(filepath.Join(sequenceDir, "nested"), 0o750), test.ShouldBeNil)

	return "mplans/seq"
}

func TestParseSequenceRange(t *testing.T) {
	first, last, err := parseSequenceRange("0", "4", 5)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, first, test.ShouldEqual, 0)
	test.That(t, last, test.ShouldEqual, 4)

	// A single plan is a valid sequence.
	_, _, err = parseSequenceRange("2", "2", 5)
	test.That(t, err, test.ShouldBeNil)

	for _, bad := range [][2]string{{"3", "2"}, {"-1", "2"}, {"0", "5"}, {"x", "2"}, {"0", ""}} {
		_, _, err = parseSequenceRange(bad[0], bad[1], 5)
		test.That(t, err, test.ShouldNotBeNil)
	}
}

func TestResolvePlanDirStaysInsidePlanRoot(t *testing.T) {
	useTempPlanRoot(t)

	absolute, err := resolvePlanDir("mplans/seq")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, absolute, test.ShouldEqual, filepath.Join(planFilesRoot, "seq"))

	for _, bad := range []string{"", "..", "mplans/../..", "/etc", "somewhere-else"} {
		_, err = resolvePlanDir(bad)
		test.That(t, err, test.ShouldNotBeNil)
	}
}

func TestIndexLinksDirectoriesWithPlansToSequencePage(t *testing.T) {
	tree := buildPlanTree("mplans", []string{
		"mplans/abc-123/a.json",
		"mplans/outer/nested/deep.json",
	})

	var sb strings.Builder
	test.That(t, indexTmpl.Execute(&sb, tree), test.ShouldBeNil)
	page := sb.String()

	test.That(t, strings.Contains(page, `href="/sequence?dir=mplans%2fabc-123"`), test.ShouldBeTrue)
	test.That(t, strings.Contains(page, `href="/sequence?dir=mplans%2fouter%2fnested"`), test.ShouldBeTrue)

	// outer holds no plan files of its own, only a directory that does.
	test.That(t, strings.Contains(page, `href="/sequence?dir=mplans%2fouter"`), test.ShouldBeFalse)
}

func TestSequencePageListsPlansInOrder(t *testing.T) {
	sequenceDir := useTempPlanRoot(t)

	recorder := httptest.NewRecorder()
	handleSequence(logging.NewTestLogger(t))(recorder, httptest.NewRequest(http.MethodGet, "/sequence?dir="+sequenceDir, nil))
	test.That(t, recorder.Code, test.ShouldEqual, http.StatusOK)

	page := recorder.Body.String()
	openIdx := strings.Index(page, "1-open.json")
	moveIdx := strings.Index(page, "2-move.json")
	test.That(t, openIdx, test.ShouldBeGreaterThan, -1)
	test.That(t, moveIdx, test.ShouldBeGreaterThan, -1)
	test.That(t, openIdx, test.ShouldBeLessThan, moveIdx)

	test.That(t, strings.Contains(page, "notes.txt"), test.ShouldBeFalse)

	// The range defaults to everything: the first entry as the first plan, the last as the last.
	test.That(t, strings.Contains(page, `<option value="0" selected>`), test.ShouldBeTrue)
	test.That(t, strings.Contains(page, `<option value="1" selected>`), test.ShouldBeTrue)
}

func TestSequenceRunPlansTheSelectedRange(t *testing.T) {
	sequenceDir := useTempPlanRoot(t)
	logger := logging.NewInMemoryLogger(t)

	recorder := httptest.NewRecorder()
	handleSequenceRun(logger)(recorder, httptest.NewRequest(http.MethodGet, "/sequence/run?dir="+sequenceDir+"&first=0&last=1", nil))

	var result sequenceRunResult
	test.That(t, json.Unmarshal(recorder.Body.Bytes(), &result), test.ShouldBeNil)
	test.That(t, result.Error, test.ShouldEqual, "")
	test.That(t, len(result.Plans), test.ShouldEqual, 2)
	test.That(t, result.Plans[0].Name, test.ShouldEqual, "1-open.json")
	test.That(t, result.Plans[0].Planned, test.ShouldBeTrue)
	test.That(t, result.Plans[1].Number, test.ShouldEqual, 2)
	test.That(t, result.Plans[1].Planned, test.ShouldBeTrue)

	// Only the second plan.
	recorder = httptest.NewRecorder()
	handleSequenceRun(logger)(recorder, httptest.NewRequest(http.MethodGet, "/sequence/run?dir="+sequenceDir+"&first=1&last=1", nil))
	result = sequenceRunResult{}
	test.That(t, json.Unmarshal(recorder.Body.Bytes(), &result), test.ShouldBeNil)
	test.That(t, len(result.Plans), test.ShouldEqual, 1)
	test.That(t, result.Plans[0].Number, test.ShouldEqual, 2)
}

func TestSequenceRunRejectsBadRange(t *testing.T) {
	sequenceDir := useTempPlanRoot(t)

	recorder := httptest.NewRecorder()
	handleSequenceRun(logging.NewTestLogger(t))(recorder,
		httptest.NewRequest(http.MethodGet, "/sequence/run?dir="+sequenceDir+"&first=1&last=0", nil))

	var result sequenceRunResult
	test.That(t, json.Unmarshal(recorder.Body.Bytes(), &result), test.ShouldBeNil)
	test.That(t, result.Error, test.ShouldNotEqual, "")
	test.That(t, len(result.Plans), test.ShouldEqual, 0)
}

func TestSequencePagePreselectsRangeFromLink(t *testing.T) {
	sequenceDir := useTempPlanRoot(t)

	recorder := httptest.NewRecorder()
	handleSequence(logging.NewTestLogger(t))(recorder,
		httptest.NewRequest(http.MethodGet, "/sequence?dir="+sequenceDir+"&first=2&last=2", nil))
	test.That(t, recorder.Code, test.ShouldEqual, http.StatusOK)

	page := recorder.Body.String()
	test.That(t, strings.Contains(page, `<option value="0" selected>`), test.ShouldBeFalse)
	test.That(t, strings.Count(page, `<option value="1" selected>`), test.ShouldEqual, 2)

	// A range that doesn't fit the directory falls back to the defaults rather than failing the page.
	recorder = httptest.NewRecorder()
	handleSequence(logging.NewTestLogger(t))(recorder,
		httptest.NewRequest(http.MethodGet, "/sequence?dir="+sequenceDir+"&first=1&last=9", nil))
	test.That(t, recorder.Code, test.ShouldEqual, http.StatusOK)
	test.That(t, strings.Contains(recorder.Body.String(), `<option value="0" selected>`), test.ShouldBeTrue)
}

func TestSequenceBranchFactorDefaultsAndComesFromLink(t *testing.T) {
	sequenceDir := useTempPlanRoot(t)

	recorder := httptest.NewRecorder()
	handleSequence(logging.NewTestLogger(t))(recorder, httptest.NewRequest(http.MethodGet, "/sequence?dir="+sequenceDir, nil))
	test.That(t, strings.Contains(recorder.Body.String(), `id="branch" min="1" step="1" value="3"`), test.ShouldBeTrue)

	recorder = httptest.NewRecorder()
	handleSequence(logging.NewTestLogger(t))(recorder, httptest.NewRequest(http.MethodGet, "/sequence?dir="+sequenceDir+"&branch=5", nil))
	test.That(t, strings.Contains(recorder.Body.String(), `value="5"`), test.ShouldBeTrue)

	recorder = httptest.NewRecorder()
	handleSequence(logging.NewTestLogger(t))(recorder, httptest.NewRequest(http.MethodGet, "/sequence?dir="+sequenceDir+"&branch=0", nil))
	test.That(t, strings.Contains(recorder.Body.String(), `value="3"`), test.ShouldBeTrue)
}

func TestSequenceRunRejectsBadBranchFactor(t *testing.T) {
	sequenceDir := useTempPlanRoot(t)

	recorder := httptest.NewRecorder()
	handleSequenceRun(logging.NewTestLogger(t))(recorder,
		httptest.NewRequest(http.MethodGet, "/sequence/run?dir="+sequenceDir+"&first=0&last=1&branch=0", nil))

	var result sequenceRunResult
	test.That(t, json.Unmarshal(recorder.Body.Bytes(), &result), test.ShouldBeNil)
	test.That(t, strings.Contains(result.Error, "branch"), test.ShouldBeTrue)
}
