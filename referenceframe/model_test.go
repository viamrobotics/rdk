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
	m1, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "foo")
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

func TestLimitOverrideRoundTrip(t *testing.T) {
	j1, err := NewRotationalFrame("j1", spatial.R4AA{RZ: 1}, Limit{Min: -math.Pi, Max: math.Pi})
	test.That(t, err, test.ShouldBeNil)
	j2, err := NewRotationalFrame("j2", spatial.R4AA{RY: 1}, Limit{Min: -math.Pi, Max: math.Pi})
	test.That(t, err, test.ShouldBeNil)

	base, err := NewSerialModel("test", []Frame{j1, j2})
	test.That(t, err, test.ShouldBeNil)

	overriddenLimit := Limit{Min: -0.5, Max: 0.5}
	overridden, err := NewModelWithLimitOverrides(base, map[string]Limit{"j1": overriddenLimit})
	test.That(t, err, test.ShouldBeNil)

	data, err := overridden.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	restored := new(SimpleModel)
	err = restored.UnmarshalJSON(data)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, restored.DoF()[0], test.ShouldResemble, overriddenLimit)
	test.That(t, restored.DoF()[1], test.ShouldResemble, Limit{Min: -math.Pi, Max: math.Pi})
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

// TestBranchingModelTransform verifies that Transform correctly uses schema offsets
// rather than a sequential posIdx when the internal frame system has branches with
// nonzero DoF.  Without the fix, D's input would be read from C's slot in the flat
// input vector, producing the wrong pose.
//
// Tree:  world -> A(X,1DOF) -> B(Y,1DOF) -> D(Z,1DOF)  [primary output]
//
//	-> C(Z,1DOF)
//
// BFS/schema order (children sorted alphabetically): A(off=0), B(off=1), C(off=2), D(off=3)
// transformChain: [A, B, D]; offsets must be [0, 1, 3] – not [0, 1, 2].
func TestBranchingModelTransform(t *testing.T) {
	a, err := NewTranslationalFrame("A", r3.Vector{X: 1}, Limit{Min: -100, Max: 100})
	test.That(t, err, test.ShouldBeNil)
	b, err := NewTranslationalFrame("B", r3.Vector{Y: 1}, Limit{Min: -100, Max: 100})
	test.That(t, err, test.ShouldBeNil)
	c, err := NewTranslationalFrame("C", r3.Vector{Z: 1}, Limit{Min: -100, Max: 100})
	test.That(t, err, test.ShouldBeNil)
	d, err := NewTranslationalFrame("D", r3.Vector{Z: 1}, Limit{Min: -100, Max: 100})
	test.That(t, err, test.ShouldBeNil)

	fs := NewEmptyFrameSystem("internal")
	test.That(t, fs.AddFrame(a, fs.World()), test.ShouldBeNil)
	test.That(t, fs.AddFrame(b, a), test.ShouldBeNil)
	test.That(t, fs.AddFrame(c, a), test.ShouldBeNil) // branch sibling of B with nonzero DoF
	test.That(t, fs.AddFrame(d, b), test.ShouldBeNil) // primary output

	model, err := NewModel("branching", fs, "D")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(model.DoF()), test.ShouldEqual, 4)

	// Schema order (BFS, children alphabetical): A(0), B(1), C(2), D(3)
	// inputs: [aVal, bVal, cVal, dVal]
	aVal, bVal, cVal, dVal := 3.0, 5.0, 99.0, 7.0
	_ = cVal // C is on the sibling branch; its value must NOT affect D's pose
	pose, err := model.Transform([]Input{aVal, bVal, cVal, dVal})
	test.That(t, err, test.ShouldBeNil)

	// D's world pose should be (aVal, bVal, dVal) — X from A, Y from B, Z from D.
	test.That(t, spatial.R3VectorAlmostEqual(pose.Point(), r3.Vector{X: aVal, Y: bVal, Z: dVal}, defaultFloatPrecision), test.ShouldBeTrue)
}

// TestBranchingModelGeometries verifies that Geometries() correctly places geometry
// at the end of both the primary chain and non-primary branches when those branches
// have nonzero DoF.
//
// Tree:  world -> A(X,1DOF) -> B(Y,1DOF) -> primaryEnd(static, geometry) [primary]
//
//	-> C(Z,1DOF) -> branchEnd(static, geometry)
//
// BFS/schema order: A(off=0,dof=1), B(off=1,dof=1), C(off=2,dof=1)
// 0-DoF frames contribute no inputs → total DoF = 3.
// inputs: [aVal, bVal, cVal]
//
// Expected world positions:
//
//	primaryEnd geometry: (aVal, bVal, 0) — path through A then B
//	branchEnd  geometry: (aVal, 0,    cVal) — path through A then C (not B)
func TestBranchingModelGeometries(t *testing.T) {
	a, err := NewTranslationalFrame("A", r3.Vector{X: 1}, Limit{Min: -100, Max: 100})
	test.That(t, err, test.ShouldBeNil)
	b, err := NewTranslationalFrame("B", r3.Vector{Y: 1}, Limit{Min: -100, Max: 100})
	test.That(t, err, test.ShouldBeNil)
	c, err := NewTranslationalFrame("C", r3.Vector{Z: 1}, Limit{Min: -100, Max: 100})
	test.That(t, err, test.ShouldBeNil)

	zp := spatial.NewZeroPose()
	box, err := spatial.NewBox(zp, r3.Vector{X: 1, Y: 1, Z: 1}, "")
	test.That(t, err, test.ShouldBeNil)
	primaryEnd, err := NewStaticFrameWithGeometry("primaryEnd", zp, box)
	test.That(t, err, test.ShouldBeNil)
	branchEnd, err := NewStaticFrameWithGeometry("branchEnd", zp, box)
	test.That(t, err, test.ShouldBeNil)

	fs := NewEmptyFrameSystem("internal")
	test.That(t, fs.AddFrame(a, fs.World()), test.ShouldBeNil)
	test.That(t, fs.AddFrame(b, a), test.ShouldBeNil)
	test.That(t, fs.AddFrame(c, a), test.ShouldBeNil) // branch sibling of B with nonzero DoF
	test.That(t, fs.AddFrame(primaryEnd, b), test.ShouldBeNil)
	test.That(t, fs.AddFrame(branchEnd, c), test.ShouldBeNil)

	model, err := NewModel("branching", fs, "primaryEnd")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(model.DoF()), test.ShouldEqual, 3)

	aVal, bVal, cVal := 3.0, 5.0, 7.0
	geoms, err := model.Geometries([]Input{aVal, bVal, cVal})
	test.That(t, err, test.ShouldBeNil)

	// Primary chain end: goes through A(X) then B(Y), Z unchanged.
	primaryPt := geoms.GeometryByName("branching:primaryEnd").Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(primaryPt, r3.Vector{X: aVal, Y: bVal, Z: 0}, defaultFloatPrecision), test.ShouldBeTrue)

	// Branch end: goes through A(X) then C(Z), Y unchanged.
	// If cVal's input were incorrectly mapped, this position would be wrong.
	branchPt := geoms.GeometryByName("branching:branchEnd").Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(branchPt, r3.Vector{X: aVal, Y: 0, Z: cVal}, defaultFloatPrecision), test.ShouldBeTrue)
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
