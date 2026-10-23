package referenceframe

import (
	"encoding/json"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// TestGripperModelJSON loads a branching gripper model from JSON and verifies that:
//   - the model parses correctly with output_frames pointing to the TCP
//   - Transform() returns the TCP position (midpoint between the fingers, at the tip)
//   - Geometries() correctly places the finger box geometries in world space
//
// Gripper tree:
//
//	world -> base(static) -> left_joint(prismatic, +Y)  -> left_finger(static, box)
//	                      -> right_joint(prismatic, −Y) -> right_finger(static, box)
//	                      -> tcp(static, no geometry)   [output frame]
//
// BFS/schema order: base, left_joint(off=0), right_joint(off=1), tcp, left_finger, right_finger
// inputs: [left_joint_mm, right_joint_mm]
func TestGripperModelJSON(t *testing.T) {
	model, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/test_gripper.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, model.Name(), test.ShouldEqual, "test_gripper")
	test.That(t, len(model.DoF()), test.ShouldEqual, 2)

	// inputs: left_joint opens to +25 mm in Y, right_joint opens to +25 mm in −Y
	leftMM, rightMM := 25.0, 25.0
	inputs := []Input{leftMM, rightMM}

	// TCP is a static link at (0,0,30) from base; it does not move with the joints.
	tcpPose, err := model.Transform(inputs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.R3VectorAlmostEqual(tcpPose.Point(), r3.Vector{X: 0, Y: 0, Z: 30}, defaultFloatPrecision), test.ShouldBeTrue)

	geoms, err := model.Geometries(inputs)
	test.That(t, err, test.ShouldBeNil)

	// Geometry offsets are in the parent frame (left_joint / right_joint) coordinate system.
	// The geometry config specifies translation (0,0,15): center of the 30mm finger body
	// measured from the joint, spanning the full finger length from Z=0 to Z=30.
	const fingerBodyCenterZ = 15.0

	// left_finger: base → left_joint at (0,+25,0) → geometry center at (0,+25,15)
	leftPt := geoms.GeometryByName("test_gripper:left_finger").Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(
		leftPt, r3.Vector{X: 0, Y: leftMM, Z: fingerBodyCenterZ}, defaultFloatPrecision,
	), test.ShouldBeTrue)

	// right_finger: base → right_joint at (0,−25,0) → geometry center at (0,−25,15)
	rightPt := geoms.GeometryByName("test_gripper:right_finger").Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(
		rightPt, r3.Vector{X: 0, Y: -rightMM, Z: fingerBodyCenterZ}, defaultFloatPrecision,
	), test.ShouldBeTrue)

	// Fingers should be symmetric about Y=0 at the same Z.
	test.That(t, spatial.R3VectorAlmostEqual(
		r3.Vector{X: leftPt.X, Y: -leftPt.Y, Z: leftPt.Z},
		r3.Vector{X: rightPt.X, Y: rightPt.Y, Z: rightPt.Z},
		defaultFloatPrecision,
	), test.ShouldBeTrue)
}

func TestLimitsParsing(t *testing.T) {
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	smodel, ok := model.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	// Set custom limits on the simple model. Such that when we roundtrip through JSON
	// serialization, we can see those values persist.
	smodel.limits[0].Min = 0
	smodel.limits[0].Max = 1

	data, err := smodel.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	simpleModelDeserialized := new(SimpleModel)
	err = simpleModelDeserialized.UnmarshalJSON(data)
	test.That(t, err, test.ShouldBeNil)

	// Assert that the min/max member values reflect the above assignments. As well as the result of
	// the interface `DoF` method.
	test.That(t, simpleModelDeserialized.limits[0].Min, test.ShouldEqual, 0)
	test.That(t, simpleModelDeserialized.limits[0].Max, test.ShouldEqual, 1)
	test.That(t, simpleModelDeserialized.DoF()[0], test.ShouldResemble, Limit{Min: 0, Max: 1})
}

// Tests that yml files are properly parsed and correctly loaded into the model
// Should not need to actually test the contained rotation/translation values
// since that will be caught by tests to the actual kinematics
// So we'll just check that we read in the right number of joints.
func TestParseJSONFile(t *testing.T) {
	goodFiles := []string{
		"components/arm/kinematics/xarm6.json",
		"components/arm/kinematics/xarm7.json",
		"referenceframe/testfiles/ur5eDH.json",
		"components/arm/kinematics/ur5e.json",
		"components/arm/kinematics/dofbot.json",
	}

	badFiles := []string{
		"referenceframe/testfiles/kinematicsloop.json",
		"referenceframe/testfiles/worldjoint.json",
		"referenceframe/testfiles/worldlink.json",
		"referenceframe/testfiles/worldDH.json",
		"referenceframe/testfiles/missinglink.json",
	}

	badFilesErrors := []error{
		ErrCircularReference,
		NewReservedWordError("link", "world"),
		NewReservedWordError("joint", "world"),
		ErrNeedOneEndEffector, // 0 end effectors
		ErrNeedOneEndEffector, // 2 end effectors
	}

	for _, f := range goodFiles {
		t.Run(f, func(tt *testing.T) {
			model, err := ParseModelJSONFile(utils.ResolveFile(f), "")
			test.That(t, err, test.ShouldBeNil)

			smodel, ok := model.(*SimpleModel)
			test.That(t, ok, test.ShouldBeTrue)
			data, err := json.Marshal(smodel.modelConfig)
			test.That(t, err, test.ShouldBeNil)

			model2, err := UnmarshalModelJSON(data, "")
			test.That(t, err, test.ShouldBeNil)

			smodel2, ok := model2.(*SimpleModel)
			test.That(t, ok, test.ShouldBeTrue)

			data2, err := json.Marshal(smodel2.modelConfig)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, data, test.ShouldResemble, data2)
		})
	}

	for i, f := range badFiles {
		t.Run(f, func(tt *testing.T) {
			_, err := ParseModelJSONFile(utils.ResolveFile(f), "")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, badFilesErrors[i].Error())
		})
	}
}

func TestMimicJSONRoundTrip(t *testing.T) {
	model, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/test_mimic_gripper.json"), "")
	test.That(t, err, test.ShouldBeNil)

	smodel, ok := model.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	// Marshal and unmarshal
	data, err := smodel.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	restored := new(SimpleModel)
	err = restored.UnmarshalJSON(data)
	test.That(t, err, test.ShouldBeNil)

	// Verify the round-tripped model is equivalent
	equal, err := framesAlmostEqual(smodel, restored, defaultFloatPrecision)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, equal, test.ShouldBeTrue)

	// Verify the mimic model still works after round-trip
	test.That(t, len(restored.DoF()), test.ShouldEqual, 1)
	pose, err := restored.Transform([]Input{25})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.R3VectorAlmostEqual(pose.Point(), r3.Vector{X: 0, Y: 0, Z: 30}, defaultFloatPrecision), test.ShouldBeTrue)
}

// A link carries every collision geometry its config lists. An unlabelled geometry is named after
// the link when it is the only one and <link>_collision_<i> otherwise, a label in the file is kept,
// and the model's JSON writes the first geometry back into the field an old reader knows.
func TestSeveralGeometriesPerLink(t *testing.T) {
	data := []byte(`{
		"name": "arm",
		"links": [
			{"id": "base", "parent": "world",
			 "geometry": {"type": "sphere", "r": 1, "translation": {"x": 10}},
			 "geometries": [
				{"type": "sphere", "r": 1, "translation": {"x": 20}, "label": "named"},
				{"type": "sphere", "r": 1, "translation": {"x": 30}}
			 ]},
			{"id": "tip", "parent": "base", "translation": {"z": 100},
			 "geometry": {"type": "sphere", "r": 1, "translation": {"x": 40}}}
		],
		"output_frames": ["tip"]
	}`)
	model, err := UnmarshalModelJSON(data, "")
	test.That(t, err, test.ShouldBeNil)

	labelsAndX := func(m Model) map[string]float64 {
		gif, err := m.Geometries(nil)
		test.That(t, err, test.ShouldBeNil)
		out := map[string]float64{}
		for _, g := range gif.Geometries() {
			out[g.Label()] = g.Pose().Point().X
		}
		return out
	}
	want := map[string]float64{"arm:base_collision_0": 10, "arm:named": 20, "arm:base_collision_2": 30, "arm:tip": 40}
	test.That(t, labelsAndX(model), test.ShouldResemble, want)

	// the config holds the split the file used, and a frame built from it writes the same split back
	sm, ok := model.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	frame := sm.internalFS.Frame("base")
	lc, err := NewLinkConfig(frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lc.Geometry.TranslationOffset.X, test.ShouldEqual, 10)
	test.That(t, len(lc.Geometries), test.ShouldEqual, 2)
	test.That(t, lc.Geometries[0].Label, test.ShouldEqual, "named")
	frameJSON, err := json.Marshal(frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(frameJSON), test.ShouldContainSubstring, `"geometries":`)
	tipJSON, err := json.Marshal(sm.internalFS.Frame("tip"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(tipJSON), test.ShouldNotContainSubstring, `"geometries":`)

	// a LinkInFrame holds one geometry, so the config refuses to become one, while a lone entry
	// under geometries is still one geometry
	_, err = lc.ParseConfig()
	test.That(t, err, test.ShouldNotBeNil)
	lone := &LinkConfig{ID: "lone", Parent: World, Geometries: []spatial.GeometryConfig{{Type: spatial.SphereType, R: 1}}}
	lif, err := lone.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lif.Geometry().Label(), test.ShouldEqual, "lone")

	reread := &SimpleModel{}
	test.That(t, json.Unmarshal(mustMarshal(t, model), reread), test.ShouldBeNil)
	test.That(t, labelsAndX(reread), test.ShouldResemble, want)
}

// A translational frame carries its geometries along the axis and names them the same way, but
// its JSON form is a prismatic joint with room for one.
func TestTranslationalFrameWithSeveralGeometries(t *testing.T) {
	a, err := spatial.NewSphere(spatial.NewZeroPose(), 1, "")
	test.That(t, err, test.ShouldBeNil)
	b, err := spatial.NewSphere(spatial.NewPoseFromPoint(r3.Vector{Y: 5}), 1, "b")
	test.That(t, err, test.ShouldBeNil)
	frame, err := NewTranslationalFrameWithGeometries("slide", r3.Vector{X: 1}, Limit{Min: 0, Max: 100}, []spatial.Geometry{a, b})
	test.That(t, err, test.ShouldBeNil)

	gif, err := frame.Geometries([]Input{7})
	test.That(t, err, test.ShouldBeNil)
	geometries := gif.Geometries()
	test.That(t, len(geometries), test.ShouldEqual, 2)
	test.That(t, geometries[0].Label(), test.ShouldEqual, "slide_collision_0")
	test.That(t, geometries[0].Pose().Point(), test.ShouldResemble, r3.Vector{X: 7})
	test.That(t, geometries[1].Label(), test.ShouldEqual, "b")
	test.That(t, geometries[1].Pose().Point(), test.ShouldResemble, r3.Vector{X: 7, Y: 5})

	_, err = json.Marshal(frame)
	test.That(t, err, test.ShouldNotBeNil)
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	test.That(t, err, test.ShouldBeNil)
	return data
}

// Collision checking keys on the label, so a model is refused when two geometries share one or a
// geometry is labelled with another frame's name.
func TestGeometryLabelsMustBeUnique(t *testing.T) {
	model := func(baseLabel, tipLabel string) []byte {
		return []byte(`{
			"name": "arm",
			"links": [
				{"id": "base", "parent": "world", "geometry": {"type": "sphere", "r": 1, "label": "` + baseLabel + `"}},
				{"id": "tip", "parent": "base", "geometry": {"type": "sphere", "r": 1, "label": "` + tipLabel + `"}}
			],
			"output_frames": ["tip"]
		}`)
	}
	_, err := UnmarshalModelJSON(model("", ""), "")
	test.That(t, err, test.ShouldBeNil)
	_, err = UnmarshalModelJSON(model("shared", "shared"), "")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `label "shared" is used on frames "base" and "tip"`)
	_, err = UnmarshalModelJSON(model("tip", ""), "")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "the name of another frame")
	_, err = UnmarshalModelJSON(model("world", ""), "")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "reserved")
}

// A prismatic joint in a kinematics file carries its geometry along the axis, the way the frame
// JSON path always has.
func TestPrismaticJointGeometryFromFile(t *testing.T) {
	data := []byte(`{
		"name": "slider",
		"links": [{"id": "base", "parent": "world"}, {"id": "tip", "parent": "slide"}],
		"joints": [{"id": "slide", "parent": "base", "type": "prismatic", "axis": {"x": 1}, "min": 0, "max": 100,
			"geometry": {"type": "sphere", "r": 1}}],
		"output_frames": ["tip"]
	}`)
	model, err := UnmarshalModelJSON(data, "")
	test.That(t, err, test.ShouldBeNil)
	gif, err := model.Geometries([]Input{7})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(gif.Geometries()), test.ShouldEqual, 1)
	test.That(t, gif.Geometries()[0].Label(), test.ShouldEqual, "slider:slide")
	test.That(t, gif.Geometries()[0].Pose().Point(), test.ShouldResemble, r3.Vector{X: 7})
}

func TestFrameConstructorsRefuseNilGeometry(t *testing.T) {
	_, err := NewStaticFrameWithGeometries("a", spatial.NewZeroPose(), []spatial.Geometry{nil})
	test.That(t, err, test.ShouldNotBeNil)
	_, err = NewTranslationalFrameWithGeometries("a", r3.Vector{X: 1}, Limit{Min: 0, Max: 1}, []spatial.Geometry{nil})
	test.That(t, err, test.ShouldNotBeNil)
	// a caller's unlabelled geometry is copied before it is named, so the caller's stays as given
	sphere, err := spatial.NewSphere(spatial.NewZeroPose(), 1, "")
	test.That(t, err, test.ShouldBeNil)
	_, err = NewStaticFrameWithGeometries("a", spatial.NewZeroPose(), []spatial.Geometry{sphere})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sphere.Label(), test.ShouldEqual, "")
}

// A frame's hash sees the order of its geometries, since the default labels depend on it and the
// primitive geometries do not hash their label.
func TestFrameHashSeesGeometryOrder(t *testing.T) {
	a, err := spatial.NewSphere(spatial.NewZeroPose(), 1, "")
	test.That(t, err, test.ShouldBeNil)
	b, err := spatial.NewSphere(spatial.NewPoseFromPoint(r3.Vector{X: 5}), 1, "")
	test.That(t, err, test.ShouldBeNil)
	ab, ba := []spatial.Geometry{a, b}, []spatial.Geometry{b, a}

	sab, err := NewStaticFrameWithGeometries("f", spatial.NewZeroPose(), ab)
	test.That(t, err, test.ShouldBeNil)
	sba, err := NewStaticFrameWithGeometries("f", spatial.NewZeroPose(), ba)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sab.Hash(), test.ShouldNotEqual, sba.Hash())

	tab, err := NewTranslationalFrameWithGeometries("t", r3.Vector{X: 1}, Limit{Min: 0, Max: 1}, ab)
	test.That(t, err, test.ShouldBeNil)
	tba, err := NewTranslationalFrameWithGeometries("t", r3.Vector{X: 1}, Limit{Min: 0, Max: 1}, ba)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, tab.Hash(), test.ShouldNotEqual, tba.Hash())

	pab, err := NewPoseFrame("p", ab)
	test.That(t, err, test.ShouldBeNil)
	pba, err := NewPoseFrame("p", ba)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pab.Hash(), test.ShouldNotEqual, pba.Hash())
}
