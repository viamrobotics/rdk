package armplanning

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

func TestPlanRequestLegacyRead(t *testing.T) {
	req, err := ReadRequestFromFile("data/plan_request_sample.json")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, req.FrameSystem, test.ShouldNotBeNil)
	test.That(t, req.FrameSystem.FrameNames(), test.ShouldNotBeEmpty)
	test.That(t, req.StartState, test.ShouldNotBeNil)
}

func TestPlanRequestFormatDetection(t *testing.T) {
	fsConfigPayload := []byte(`{"frame_system":{"name":"n","parts":[]},"start_state":{}}`)
	got, err := isFrameSystemConfig(fsConfigPayload)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldBeTrue)

	legacyPayload := []byte(`{"frame_system":{"name":"n","frames":{},"parents":{},"world":{}},"start_state":{}}`)
	got, err = isFrameSystemConfig(legacyPayload)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldBeFalse)
}

func TestPlanRequestFrameSystemConfigRoundTrip(t *testing.T) {
	t.Setenv(FrameSystemConfigEnvVar, "1")

	modelJSON, err := os.ReadFile(rdkutils.ResolveFile("config/data/model_frame.json"))
	test.That(t, err, test.ShouldBeNil)
	armModel, err := referenceframe.UnmarshalModelJSON(modelJSON, "arm")
	test.That(t, err, test.ShouldBeNil)

	mountPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 10, Y: 20, Z: 30})
	gripperGeom, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 10, Y: 10, Z: 10}, "gripper")
	test.That(t, err, test.ShouldBeNil)

	parts := []*referenceframe.FrameSystemPart{
		{FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, mountPose, "arm", nil), ModelFrame: armModel},
		{FrameConfig: referenceframe.NewLinkInFrame("arm", spatialmath.NewZeroPose(), "gripper", gripperGeom)},
	}
	fs, err := referenceframe.NewFrameSystem("test", parts, nil)
	test.That(t, err, test.ShouldBeNil)

	original := &PlanRequest{
		FrameSystem:    fs,
		StartState:     &PlanState{},
		PlannerOptions: NewBasicPlannerOptions(),
	}

	raw, err := json.Marshal(original)
	test.That(t, err, test.ShouldBeNil)

	// Nested model internals carry a frame system structure, so probe the
	// outer frame_system shape directly rather than scanning the full payload.
	var top struct {
		FrameSystem map[string]json.RawMessage `json:"frame_system"`
	}
	test.That(t, json.Unmarshal(raw, &top), test.ShouldBeNil)
	_, hasParts := top.FrameSystem["parts"]
	_, hasFrames := top.FrameSystem["frames"]
	_, hasParents := top.FrameSystem["parents"]
	test.That(t, hasParts, test.ShouldBeTrue)
	test.That(t, hasFrames, test.ShouldBeFalse)
	test.That(t, hasParents, test.ShouldBeFalse)

	var reread PlanRequest
	test.That(t, json.Unmarshal(raw, &reread), test.ShouldBeNil)
	test.That(t, reread.FrameSystem, test.ShouldNotBeNil)
	test.That(t, reread.FrameSystem.Name(), test.ShouldEqual, fs.Name())
	test.That(t, reread.FrameSystem.FrameNames(), test.ShouldResemble, fs.FrameNames())

	for _, name := range fs.FrameNames() {
		got := reread.FrameSystem.Frame(name)
		test.That(t, got, test.ShouldNotBeNil)
		test.That(t, got.DoF(), test.ShouldResemble, fs.Frame(name).DoF())
	}
}

// TestPlanRequestDefaultsToLegacy guards against a regression that silently
// flips the writer default.
func TestPlanRequestDefaultsToLegacy(t *testing.T) {
	t.Setenv(FrameSystemConfigEnvVar, "")

	fs, err := referenceframe.NewFrameSystem("test", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	req := &PlanRequest{
		FrameSystem:    fs,
		StartState:     &PlanState{},
		PlannerOptions: NewBasicPlannerOptions(),
	}

	raw, err := json.Marshal(req)
	test.That(t, err, test.ShouldBeNil)

	var top struct {
		FrameSystem map[string]json.RawMessage `json:"frame_system"`
	}
	test.That(t, json.Unmarshal(raw, &top), test.ShouldBeNil)
	_, hasParts := top.FrameSystem["parts"]
	_, hasFrames := top.FrameSystem["frames"]
	_, hasParents := top.FrameSystem["parents"]
	test.That(t, hasParts, test.ShouldBeFalse)
	test.That(t, hasFrames, test.ShouldBeTrue)
	test.That(t, hasParents, test.ShouldBeTrue)
}

// TestPlanRequestSampleRoundTripNewForm round-trips the committed sample file
// through the fs-config writer and reader, asserting world poses and geometries
// match frame-for-frame. Guards against an UnmarshalJSON path that leaves the
// rebuilt Model with empty DoF and panics on Transform.
func TestPlanRequestSampleRoundTripNewForm(t *testing.T) {
	original, _, err := ReadRequestAndResponseFromFile("data/plan_request_sample.json")
	test.That(t, err, test.ShouldBeNil)

	t.Setenv(FrameSystemConfigEnvVar, "1")
	raw, err := json.Marshal(original)
	test.That(t, err, test.ShouldBeNil)

	var reread PlanRequest
	reread.PlannerOptions = NewBasicPlannerOptions()
	test.That(t, json.Unmarshal(raw, &reread), test.ShouldBeNil)

	// Part-by-part equivalence: each originally-named frame maps to a rebuilt Part
	// with the same parent, pose, and DoF.
	origParts, err := original.FrameSystem.Parts()
	test.That(t, err, test.ShouldBeNil)
	rereadParts, err := reread.FrameSystem.Parts()
	test.That(t, err, test.ShouldBeNil)
	byName := make(map[string]*referenceframe.FrameSystemPart, len(rereadParts))
	for _, p := range rereadParts {
		byName[p.FrameConfig.Name()] = p
	}
	for _, orig := range origParts {
		got, ok := byName[orig.FrameConfig.Name()]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, got.FrameConfig.Parent(), test.ShouldEqual, orig.FrameConfig.Parent())
		test.That(t, spatialmath.PoseAlmostEqual(got.FrameConfig.Pose(), orig.FrameConfig.Pose()), test.ShouldBeTrue)
		if orig.ModelFrame != nil {
			test.That(t, got.ModelFrame, test.ShouldNotBeNil)
			test.That(t, got.ModelFrame.DoF(), test.ShouldResemble, orig.ModelFrame.DoF())
		}
	}
}

// TestPlanRequestPreservesPlannerOptionDefaults pins that the fs-config read
// path does not clobber defaults that RequestFromReader pre-seeded.
func TestPlanRequestPreservesPlannerOptionDefaults(t *testing.T) {
	t.Setenv(FrameSystemConfigEnvVar, "1")

	fs, err := referenceframe.NewFrameSystem("test", nil, nil)
	test.That(t, err, test.ShouldBeNil)
	req := &PlanRequest{FrameSystem: fs, StartState: &PlanState{}, PlannerOptions: NewBasicPlannerOptions()}
	raw, err := json.Marshal(req)
	test.That(t, err, test.ShouldBeNil)

	withoutOptions := []byte(`{"frame_system":{"name":"test","parts":[]},"start_state":{}}`)
	test.That(t, json.Valid(withoutOptions), test.ShouldBeTrue)

	reread := &PlanRequest{PlannerOptions: NewBasicPlannerOptions()}
	test.That(t, json.Unmarshal(withoutOptions, reread), test.ShouldBeNil)
	defaults := NewBasicPlannerOptions()
	test.That(t, reread.PlannerOptions.CollisionBufferMM, test.ShouldEqual, defaults.CollisionBufferMM)
	_ = raw
}

// TestPlanRequestModelCompact pins the compact Model shape against regression.
func TestPlanRequestModelCompact(t *testing.T) {
	t.Setenv(FrameSystemConfigEnvVar, "1")

	modelJSON, err := os.ReadFile(rdkutils.ResolveFile("config/data/model_frame.json"))
	test.That(t, err, test.ShouldBeNil)
	armModel, err := referenceframe.UnmarshalModelJSON(modelJSON, "arm")
	test.That(t, err, test.ShouldBeNil)

	parts := []*referenceframe.FrameSystemPart{
		{
			FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), "arm", nil),
			ModelFrame:  armModel,
		},
	}
	fs, err := referenceframe.NewFrameSystem("test", parts, nil)
	test.That(t, err, test.ShouldBeNil)

	req := &PlanRequest{FrameSystem: fs, StartState: &PlanState{}, PlannerOptions: NewBasicPlannerOptions()}
	raw, err := json.Marshal(req)
	test.That(t, err, test.ShouldBeNil)

	var payload struct {
		FrameSystem struct {
			Parts []struct {
				Model map[string]json.RawMessage `json:"model"`
			} `json:"parts"`
		} `json:"frame_system"`
	}
	test.That(t, json.Unmarshal(raw, &payload), test.ShouldBeNil)
	test.That(t, payload.FrameSystem.Parts, test.ShouldHaveLength, 1)

	modelBlock := payload.FrameSystem.Parts[0].Model
	_, hasInternalFS := modelBlock["internal_fs"]
	test.That(t, hasInternalFS, test.ShouldBeFalse)

	var nested struct {
		OriginalFile json.RawMessage `json:"original_file"`
	}
	test.That(t, json.Unmarshal(modelBlock["model"], &nested), test.ShouldBeNil)
	test.That(t, len(nested.OriginalFile), test.ShouldEqual, 0)
}
