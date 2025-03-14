package spatialmath

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

func makeTestMesh(o Orientation, pt r3.Vector, triangles []*Triangle) *Mesh {
	return &Mesh{
		pose:      NewPose(pt, o),
		triangles: triangles,
	}
}

func makeSimpleTriangleMesh() *Mesh {
	// Create a simple triangle mesh at origin
	tri1 := NewTriangle(
		r3.Vector{X: 0, Y: 0, Z: 0},
		r3.Vector{X: 1, Y: 0, Z: 0},
		r3.Vector{X: 0, Y: 1, Z: 0},
	)
	tri2 := NewTriangle(
		r3.Vector{X: 0.6, Y: 0.6, Z: 0},
		r3.Vector{X: 1, Y: 0, Z: 0},
		r3.Vector{X: 0, Y: 1, Z: 0},
	)
	tri3 := NewTriangle(
		r3.Vector{X: 0, Y: 0, Z: 10},
		r3.Vector{X: 1, Y: 0, Z: 10},
		r3.Vector{X: 0, Y: 1, Z: 10},
	)
	return makeTestMesh(NewZeroOrientation(), r3.Vector{}, []*Triangle{tri1, tri2, tri3})
}

func TestNewMesh(t *testing.T) {
	tri := NewTriangle(
		r3.Vector{X: 0, Y: 0, Z: 0},
		r3.Vector{X: 1, Y: 0, Z: 0},
		r3.Vector{X: 0, Y: 1, Z: 0},
	)
	pose := NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, NewZeroOrientation())

	mesh := NewMesh(pose, []*Triangle{tri}, "test_mesh")

	test.That(t, mesh.Label(), test.ShouldEqual, "test_mesh")
	test.That(t, PoseAlmostEqual(mesh.Pose(), pose), test.ShouldBeTrue)
	test.That(t, len(mesh.Triangles()), test.ShouldEqual, 1)
}

func TestMeshProtoConversion(t *testing.T) {
	m, err := NewMeshFromPLYFile(utils.ResolveFile("spatialmath/data/simple.ply"))
	test.That(t, err, test.ShouldBeNil)
	m2, err := NewGeometryFromProto(m.ToProtobuf())
	test.That(t, err, test.ShouldBeNil)

	test.That(t, PoseAlmostEqual(m.Pose(), m2.Pose()), test.ShouldBeTrue)
	test.That(t, m.Label(), test.ShouldResemble, m2.Label())
	test.That(t, len(m.Triangles()), test.ShouldEqual, 2)
	test.That(t, len(m2.(*Mesh).Triangles()), test.ShouldEqual, 2)
	test.That(t, m.Triangles()[0], test.ShouldResemble, m2.(*Mesh).Triangles()[0])
	test.That(t, m.Triangles()[1], test.ShouldResemble, m2.(*Mesh).Triangles()[1])
}

func TestMeshTransform(t *testing.T) {
	mesh := makeSimpleTriangleMesh()

	// Transform mesh by translation
	newPose := NewPose(r3.Vector{X: 1, Y: 0, Z: 0}, NewZeroOrientation())
	transformed := mesh.Transform(newPose)

	// Check that transformed mesh has correct pose
	test.That(t, transformed.Pose().Point().X, test.ShouldEqual, 1)

	// Original mesh should be unchanged
	test.That(t, mesh.Pose().Point().X, test.ShouldEqual, 0)
}

func TestMeshCollidesWithMesh(t *testing.T) {
	mesh1 := makeSimpleTriangleMesh()

	// Test collision with overlapping mesh
	mesh2 := makeTestMesh(NewZeroOrientation(), r3.Vector{X: 0.5, Y: 0.5, Z: 0},
		[]*Triangle{NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)})

	collides, err := mesh1.CollidesWith(mesh2, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)

	// Test collision with non-overlapping mesh
	mesh3 := makeTestMesh(NewZeroOrientation(), r3.Vector{X: 2, Y: 2, Z: 0},
		[]*Triangle{NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)})

	collides, err = mesh1.CollidesWith(mesh3, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeFalse)
}

func TestMeshCollidesWithBox(t *testing.T) {
	mesh := makeSimpleTriangleMesh()

	// Create overlapping box
	box, err := NewBox(NewZeroPose(), r3.Vector{X: 1, Y: 1, Z: 1}, "")
	test.That(t, err, test.ShouldBeNil)

	collides, err := mesh.CollidesWith(box, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)

	// Create non-overlapping box
	box2, err := NewBox(NewPose(r3.Vector{X: 2, Y: 2, Z: 2}, NewZeroOrientation()),
		r3.Vector{X: 1, Y: 1, Z: 1}, "")
	test.That(t, err, test.ShouldBeNil)

	collides, err = mesh.CollidesWith(box2, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeFalse)
}

func TestMeshDistanceFrom(t *testing.T) {
	mesh1 := makeSimpleTriangleMesh()

	// Test distance from overlapping mesh
	mesh2 := makeTestMesh(NewZeroOrientation(), r3.Vector{},
		[]*Triangle{NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)})

	dist, err := mesh1.DistanceFrom(mesh2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dist, test.ShouldEqual, 0)

	// Test distance from non-overlapping mesh
	mesh3 := makeTestMesh(NewZeroOrientation(), r3.Vector{X: 2, Y: 0, Z: 0},
		[]*Triangle{NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)})

	dist, err = mesh1.DistanceFrom(mesh3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dist, test.ShouldBeGreaterThan, 0)
}

func TestMeshToPoints(t *testing.T) {
	mesh := makeSimpleTriangleMesh()

	points := mesh.ToPoints(1)
	test.That(t, len(points), test.ShouldEqual, 7)

	// Verify points match triangle vertices
	expectedPoints := []r3.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 0, Z: 0},
		{X: 0, Y: 1, Z: 0},
		{X: 0.6, Y: 0.6, Z: 0},
		{X: 0, Y: 0, Z: 10},
		{X: 1, Y: 0, Z: 10},
		{X: 0, Y: 1, Z: 10},
	}

	for _, expected := range expectedPoints {
		found := false
		for _, actual := range points {
			if R3VectorAlmostEqual(actual, expected, 1e-10) {
				found = true
				break
			}
		}
		test.That(t, found, test.ShouldBeTrue)
	}
}

func TestMeshEncompassedBy(t *testing.T) {
	mesh := makeSimpleTriangleMesh()

	// Test with encompassing box
	box, err := NewBox(NewZeroPose(), r3.Vector{X: 20, Y: 20, Z: 20}, "")
	test.That(t, err, test.ShouldBeNil)

	encompassed, err := mesh.EncompassedBy(box)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, encompassed, test.ShouldBeTrue)

	// Test with box encompassing some but not all triangles
	smallBox, err := NewBox(NewZeroPose(), r3.Vector{X: 2, Y: 2, Z: 2}, "")
	test.That(t, err, test.ShouldBeNil)

	encompassed, err = mesh.EncompassedBy(smallBox)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, encompassed, test.ShouldBeFalse)
}
