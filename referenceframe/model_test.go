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
}

func TestHash(t *testing.T) {
	j1, err := NewRotationalFrame("j1", spatial.R4AA{RZ: 1}, Limit{Min: -math.Pi, Max: math.Pi})
	test.That(t, err, test.ShouldBeNil)

	m1, err := NewSerialModel("model_a", []Frame{j1})
	test.That(t, err, test.ShouldBeNil)

	// Hash should be stable across calls and clones
	h1 := m1.Hash()
	m1clone, err := clone(m1)
	test.That(t, err, test.ShouldBeNil)
	h2 := m1clone.Hash()
	test.That(t, h1, test.ShouldEqual, h2)
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

func TestSerialModelDuplicateNames(t *testing.T) {
	j1, err := NewRotationalFrame("joint", spatial.R4AA{RZ: 1}, Limit{Min: -math.Pi, Max: math.Pi})
	test.That(t, err, test.ShouldBeNil)
	j2, err := NewRotationalFrame("joint", spatial.R4AA{RY: 1}, Limit{Min: -math.Pi, Max: math.Pi})
	test.That(t, err, test.ShouldBeNil)

	model, err := NewSerialModel("test", []Frame{j1, j2})
	test.That(t, model, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, `duplicate frame name "joint" in serial model`)
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
