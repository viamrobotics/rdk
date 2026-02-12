package referenceframe

import (
	"math"
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestModelLoading(t *testing.T) {
	m, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.Name(), test.ShouldEqual, "xArm6")
	simpleM, ok := m.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, len(m.DoF()), test.ShouldEqual, 6)

	err = simpleM.validInputs([]Input{0.1, 0.1, 0.1, 0.1, 0.1, 0.1})
	test.That(t, err, test.ShouldBeNil)
	err = simpleM.validInputs([]Input{0.1, 0.1, 0.1, 0.1, 0.1, 99.1})
	test.That(t, err, test.ShouldNotBeNil)

	orig := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1}
	orig[5] += math.Pi * 2
	orig[4] -= math.Pi * 4

	randpos := GenerateRandomConfiguration(m, rand.New(rand.NewSource(1)))
	test.That(t, simpleM.validInputs(randpos), test.ShouldBeNil)

	m, err = ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "foo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.Name(), test.ShouldEqual, "foo")
}

func TestIncorrectInputs(t *testing.T) {
	m, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	dof := len(m.DoF())

	// test incorrect number of inputs
	pose, err := m.Transform(make([]Input, dof+1))
	test.That(t, pose, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, NewIncorrectDoFError(dof+1, dof).Error())

	// test incorrect number of inputs to Geometries
	gf, err := m.Geometries(make([]Input, dof-1))
	test.That(t, gf, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, NewIncorrectDoFError(dof-1, dof).Error())
}

func TestModelGeometries(t *testing.T) {
	// build a test model
	offset := spatial.NewPoseFromPoint(r3.Vector{0, 0, 10})
	bc, err := spatial.NewBox(offset, r3.Vector{1, 1, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	frame1, err := NewStaticFrameWithGeometry("link1", offset, bc)
	test.That(t, err, test.ShouldBeNil)
	frame2, err := NewRotationalFrame("", spatial.R4AA{RY: 1}, Limit{Min: -360, Max: 360})
	test.That(t, err, test.ShouldBeNil)
	frame3, err := NewStaticFrameWithGeometry("link2", offset, bc)
	test.That(t, err, test.ShouldBeNil)
	m, err := NewSerialModel("test", []Frame{frame1, frame2, frame3})
	test.That(t, err, test.ShouldBeNil)

	// test zero pose of model
	inputs := make([]Input, len(m.DoF()))
	geometries, err := m.Geometries(inputs)
	test.That(t, err, test.ShouldBeNil)
	link1 := geometries.GeometryByName("test:link1").Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(link1, r3.Vector{0, 0, 10}, defaultFloatPrecision), test.ShouldBeTrue)
	link2 := geometries.GeometryByName("test:link2").Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(link2, r3.Vector{0, 0, 20}, defaultFloatPrecision), test.ShouldBeTrue)

	// transform the model 90 degrees at the joint
	inputs[0] = math.Pi / 2
	geometries, _ = m.Geometries(inputs)
	test.That(t, geometries, test.ShouldNotBeNil)
	link1 = geometries.GeometryByName("test:link1").Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(link1, r3.Vector{0, 0, 10}, defaultFloatPrecision), test.ShouldBeTrue)
	link2 = geometries.GeometryByName("test:link2").Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(link2, r3.Vector{10, 0, 10}, defaultFloatPrecision), test.ShouldBeTrue)
}

func Test2DMobileModelFrame(t *testing.T) {
	expLimit := []Limit{{-10, 10}, {-10, 10}, {-2 * math.Pi, 2 * math.Pi}}
	sphere, err := spatial.NewSphere(spatial.NewZeroPose(), 10, "")
	test.That(t, err, test.ShouldBeNil)
	frame, err := New2DMobileModelFrame("test", expLimit, sphere)
	test.That(t, err, test.ShouldBeNil)
	// expected output
	expPose := spatial.NewPose(r3.Vector{3, 5, 0}, &spatial.OrientationVector{OZ: 1, Theta: math.Pi / 2})
	// get expected transform back
	pose, err := frame.Transform([]Input{3, 5, math.Pi / 2})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldResemble, expPose)
	// if you feed in too many inputs, should get error back
	_, err = frame.Transform([]Input{3, 5, 0, 10})
	test.That(t, err, test.ShouldNotBeNil)
	// if you feed in too few inputs, should get errr back
	_, err = frame.Transform([]Input{3})
	test.That(t, err, test.ShouldNotBeNil)
	// if you try to move beyond set limits, should get an error
	_, err = frame.Transform([]Input{3, 100})
	test.That(t, err, test.ShouldNotBeNil)
	// gets the correct limits back
	limit := frame.DoF()
	test.That(t, limit[0], test.ShouldResemble, expLimit[0])
}

func TestTreeTopologyParsing(t *testing.T) {
	// Parse a tree topology model with branching fingers
	m, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/tree_gripper.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.Name(), test.ShouldEqual, "tree_gripper")

	smodel, ok := m.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	// DoF should count all joints: wrist_joint (1) + finger1_joint (1) + finger2_joint (1) = 3
	test.That(t, len(m.DoF()), test.ShouldEqual, 3)

	// primaryOutputFrame should be as specified in JSON
	test.That(t, smodel.primaryOutputFrame, test.ShouldEqual, "finger1_tip")

	// Transform with zero inputs should return the finger1_tip pose
	zeroInputs := make([]Input, 3)
	pose, err := m.Transform(zeroInputs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldNotBeNil)
	// finger1_tip at zero: wrist (0 rotation) -> palm (z+50) -> finger1_joint (0 translation) -> finger1_tip (z+30)
	// Total: z = 80
	test.That(t, spatial.R3VectorAlmostEqual(pose.Point(), r3.Vector{0, 0, 80}, defaultFloatPrecision), test.ShouldBeTrue)
}

func TestTreeGeometries(t *testing.T) {
	// Parse a tree topology model
	m, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/tree_gripper.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// Geometries should return geometry from ALL branches
	zeroInputs := make([]Input, len(m.DoF()))
	geoms, err := m.Geometries(zeroInputs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, geoms, test.ShouldNotBeNil)

	// We should have geometries from palm, finger1_tip, and finger2_tip (links with geometry)
	allGeoms := geoms.Geometries()
	test.That(t, len(allGeoms), test.ShouldBeGreaterThanOrEqualTo, 3)
}

func TestMultiLeafNoPrimaryOutputFrame(t *testing.T) {
	// A tree with multiple leaves and no primary_output_frame should fail
	_, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/missinglink.json"), "")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, ErrNeedPrimaryOutputFrame.Error())
}

func TestSerialChainBackwardCompat(t *testing.T) {
	// Load existing arm model and verify it produces identical results
	m, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	smodel, ok := m.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	// Should have the correct DoF
	test.That(t, len(m.DoF()), test.ShouldEqual, 6)

	// Transform should work with valid inputs (all zeros are valid for all joints)
	inputs := make([]Input, 6)
	pose, err := m.Transform(inputs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose, test.ShouldNotBeNil)

	// Geometries should work
	geoms, err := m.Geometries(inputs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, geoms, test.ShouldNotBeNil)

	// Should have frames in the internal FS
	test.That(t, len(smodel.framesInOrder()), test.ShouldBeGreaterThan, 0)
}

func TestNewModel(t *testing.T) {
	x, err := NewTranslationalFrame("x", r3.Vector{X: 1}, Limit{Min: -100, Max: 100})
	test.That(t, err, test.ShouldBeNil)
	y, err := NewTranslationalFrame("y", r3.Vector{Y: 1}, Limit{Min: -100, Max: 100})
	test.That(t, err, test.ShouldBeNil)

	model, err := NewSerialModel("gantry", []Frame{x, y})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(model.DoF()), test.ShouldEqual, 2)

	pose, err := model.Transform([]Input{10, 20})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.R3VectorAlmostEqual(pose.Point(), r3.Vector{10, 20, 0}, defaultFloatPrecision), test.ShouldBeTrue)

	// Invalid primary output frame should error
	badFS, _, err := NewSerialFrameSystem([]Frame{x, y})
	test.That(t, err, test.ShouldBeNil)
	_, err = NewModel("bad", badFS, "nonexistent")
	test.That(t, err, test.ShouldNotBeNil)
}

func TestHash(t *testing.T) {
	j1, err := NewRotationalFrame("j1", spatial.R4AA{RZ: 1}, Limit{Min: -math.Pi, Max: math.Pi})
	test.That(t, err, test.ShouldBeNil)

	m1, err := NewSerialModel("model_a", []Frame{j1})
	test.That(t, err, test.ShouldBeNil)

	// Hash should be stable across calls
	h1 := m1.Hash()
	h2 := m1.Hash()
	test.That(t, h1, test.ShouldEqual, h2)

	// Different model name should produce a different hash
	m2, err := NewSerialModel("model_b", []Frame{j1})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m1.Hash(), test.ShouldNotEqual, m2.Hash())
}

func TestNewModelWithLimitOverrides(t *testing.T) {
	j1, err := NewRotationalFrame("j1", spatial.R4AA{RZ: 1}, Limit{Min: -math.Pi, Max: math.Pi})
	test.That(t, err, test.ShouldBeNil)
	j2, err := NewRotationalFrame("j2", spatial.R4AA{RY: 1}, Limit{Min: -math.Pi, Max: math.Pi})
	test.That(t, err, test.ShouldBeNil)

	base, err := NewSerialModel("test", []Frame{j1, j2})
	test.That(t, err, test.ShouldBeNil)

	// Override the limit of j1
	newLimit := Limit{Min: -0.5, Max: 0.5}
	overridden, err := NewModelWithLimitOverrides(base, map[string]Limit{"j1": newLimit})
	test.That(t, err, test.ShouldBeNil)

	// The overridden model should reflect the new limit
	test.That(t, overridden.DoF()[0], test.ShouldResemble, newLimit)
	// j2 should be unchanged
	test.That(t, overridden.DoF()[1], test.ShouldResemble, Limit{Min: -math.Pi, Max: math.Pi})
	// Original model should be unchanged
	test.That(t, base.DoF()[0], test.ShouldResemble, Limit{Min: -math.Pi, Max: math.Pi})

	// Override a nonexistent frame should error
	_, err = NewModelWithLimitOverrides(base, map[string]Limit{"nonexistent": newLimit})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found or has no DoF")
}

func TestNewSerialFrameSystemDuplicateNames(t *testing.T) {
	j1, err := NewRotationalFrame("joint", spatial.R4AA{RZ: 1}, Limit{Min: -math.Pi, Max: math.Pi})
	test.That(t, err, test.ShouldBeNil)
	j2, err := NewRotationalFrame("joint", spatial.R4AA{RY: 1}, Limit{Min: -math.Pi, Max: math.Pi})
	test.That(t, err, test.ShouldBeNil)

	fs, lastFrame, err := NewSerialFrameSystem([]Frame{j1, j2})
	test.That(t, err, test.ShouldBeNil)

	// The second frame should have been renamed to avoid collision
	test.That(t, lastFrame, test.ShouldEqual, "joint_2")

	// Both frames should be accessible in the frame system
	test.That(t, fs.Frame("joint"), test.ShouldNotBeNil)
	test.That(t, fs.Frame("joint_2"), test.ShouldNotBeNil)
}

func TestProtobufRoundtrip(t *testing.T) {
	// Build a model with a rotational joint (degrees<->radians conversion) and a translational joint
	rot, err := NewRotationalFrame("rot", spatial.R4AA{RZ: 1}, Limit{Min: -math.Pi, Max: math.Pi})
	test.That(t, err, test.ShouldBeNil)
	trans, err := NewTranslationalFrame("trans", r3.Vector{X: 1}, Limit{Min: -100, Max: 100})
	test.That(t, err, test.ShouldBeNil)

	m, err := NewSerialModel("test", []Frame{rot, trans})
	test.That(t, err, test.ShouldBeNil)

	// Inputs in radians/mm
	origInputs := []Input{math.Pi / 4, 50.0}

	// Convert to protobuf (rotational value should become degrees)
	jp := m.ProtobufFromInput(origInputs)
	test.That(t, jp.Values[0], test.ShouldAlmostEqual, 45.0, 1e-10) // pi/4 = 45 degrees
	test.That(t, jp.Values[1], test.ShouldAlmostEqual, 50.0, 1e-10) // translational stays the same

	// Convert back from protobuf
	roundtripped := m.InputFromProtobuf(jp)
	test.That(t, len(roundtripped), test.ShouldEqual, 2)
	test.That(t, roundtripped[0], test.ShouldAlmostEqual, origInputs[0], 1e-10)
	test.That(t, roundtripped[1], test.ShouldAlmostEqual, origInputs[1], 1e-10)
}

func TestExtractMeshMapFromModelConfig(t *testing.T) {
	// Use dummy bytes for testing - no need to load actual files
	stlBytes := []byte("fake stl data")
	plyBytes := []byte("fake ply data")

	t.Run("extracts meshes from model config", func(t *testing.T) {
		cfg := &ModelConfigJSON{
			Links: []LinkConfig{
				{
					ID: "link1",
					Geometry: &spatial.GeometryConfig{
						Type:            spatial.MeshType,
						MeshData:        stlBytes,
						MeshContentType: "stl",
						MeshFilePath:    "meshes/link1.stl",
					},
				},
				{
					ID: "link2",
					Geometry: &spatial.GeometryConfig{
						Type:            spatial.MeshType,
						MeshData:        plyBytes,
						MeshContentType: "ply",
						MeshFilePath:    "models/link2.ply",
					},
				},
				{
					ID: "link3",
					Geometry: &spatial.GeometryConfig{
						Type: spatial.BoxType,
						X:    1, Y: 2, Z: 3,
					},
				},
			},
		}

		meshMap := extractMeshMapFromModelConfig(cfg)
		test.That(t, len(meshMap), test.ShouldEqual, 2)

		// Verify STL mesh
		stlMesh := meshMap["meshes/link1.stl"]
		test.That(t, stlMesh.ContentType, test.ShouldEqual, "stl")
		test.That(t, stlMesh.Mesh, test.ShouldResemble, stlBytes)

		// Verify PLY mesh
		plyMesh := meshMap["models/link2.ply"]
		test.That(t, plyMesh.ContentType, test.ShouldEqual, "ply")
		test.That(t, plyMesh.Mesh, test.ShouldResemble, plyBytes)
	})
}
