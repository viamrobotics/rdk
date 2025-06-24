package spatialmath

import (
	"fmt"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

func makeTestMesh(o Orientation, pt r3.Vector, triangles []*Triangle) *Mesh {
	return NewMesh(NewPose(pt, o), triangles, "")
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

func TestMeshCollidesWithCapsule(t *testing.T) {
	mesh := makeSimpleTriangleMesh()
	// TODOTODOTODO: NEED TO REORGANIZE AS WITH BOX, AND ADD t.RUNS
	// e.g., like capsule line is not a necessary thing to specify, for example. glad to have better clarity on this.
	// looks like capsule canonically points on z-axis

	// Collision with triangle vertex
	// Capsule extreme vertex collision (with triangle vertex)
	// shift up by 1.5
	capsule, err := NewCapsule(NewPose(r3.Vector{X: 0, Y: 0, Z: 1.5},
		NewZeroOrientation()), 1, 3, "")
	test.That(t, err, test.ShouldBeNil)
	collides, err := mesh.CollidesWith(capsule, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)
	// Capsule non-extreme spherical vertex collision (with triangle vertex)
	// shift up by 1.5, then down by r/2 and back by 3r/4
	capsule, err = NewCapsule(NewPose(r3.Vector{X: -0.75, Y: 0, Z: 1},
		NewZeroOrientation()), 1, 3, "")
	test.That(t, err, test.ShouldBeNil)
	collides, err = mesh.CollidesWith(capsule, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)
	// Capsule cylinder vertex collision (with triangle vertex)
	// shift left by r
	capsule, err = NewCapsule(NewPose(r3.Vector{X: -1, Y: 0, Z: 0},
		NewZeroOrientation()), 1, 3, "")
	test.That(t, err, test.ShouldBeNil)
	collides, err = mesh.CollidesWith(capsule, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)
	// Capsule cylinder line collision (with triangle vertex) (not possible)

	// Collision with triangle edge
	// Capsule extreme vertex collision (with triangle edge)
	// shift (0.5, 0, 1.5)
	capsule, err = NewCapsule(NewPose(r3.Vector{X: 0.5, Y: 0, Z: 1.5},
		NewZeroOrientation()), 1, 3, "")
	test.That(t, err, test.ShouldBeNil)
	collides, err = mesh.CollidesWith(capsule, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)
	// Capsule non-extreme spherical vertex collision (with triangle edge)
	// shift (-0.75, 0.5, 1)
	capsule, err = NewCapsule(NewPose(r3.Vector{X: -0.75, Y: 0.5, Z: 1},
		NewZeroOrientation()), 1, 3, "")
	test.That(t, err, test.ShouldBeNil)
	collides, err = mesh.CollidesWith(capsule, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)
	// Capsule cylinder vertex collision (with triangle edge)
	capsule, err = NewCapsule(NewPose(r3.Vector{X: 0.5, Y: -1, Z: 0},
		NewZeroOrientation()), 1, 3, "")
	test.That(t, err, test.ShouldBeNil)
	collides, err = mesh.CollidesWith(capsule, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)
	// Capsule cylinder line collision (with triangle edge)
	capsule, err = NewCapsule(NewPose(r3.Vector{X: 0, Y: -1, Z: 0},
		&OrientationVector{OX: 1}), 1, 3, "")
	test.That(t, err, test.ShouldBeNil)
	collides, err = mesh.CollidesWith(capsule, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)

	// Collision with triangle face
	// Capsule extreme vertex collision (with triangle face)
	// (0.5, 0.5, 1.5)
	capsule, err = NewCapsule(NewPose(r3.Vector{X: 0.5, Y: 0.5, Z: 1.5},
		NewZeroOrientation()), 1, 3, "")
	test.That(t, err, test.ShouldBeNil)
	collides, err = mesh.CollidesWith(capsule, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)
	// Capsule non-extreme spherical vertex collision (with triangle face)
	// this one is super cringe... math.Sqrt()
	// (0,1,1) orientation, (0.5,0.5,1+math.Sqrt(2)/2)
	// this is presumably failing due to rounding error... except actually the error seems pretty big
	capsule, err = NewCapsule(NewPose(r3.Vector{X: 0.5, Y: 0.5, Z: 1 + math.Sqrt(2)/4},
		&OrientationVector{OY: 1, OZ: 1}), 1, 3, "")
	test.That(t, err, test.ShouldBeNil)
	collides, err = mesh.CollidesWith(capsule, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)
	// Capsule cylinder vertex collision (with triangle face)
	// NOT POSSIBLE
	// Capsule cylinder line collision (with triangle face)
	// on its side again, but tiny modified capsule
	capsule, err = NewCapsule(NewPose(r3.Vector{X: 0.2, Y: 0.2, Z: 0.1},
		&OrientationVector{OX: 1}), 0.1, 0.3, "")
	test.That(t, err, test.ShouldBeNil)
	collides, err = mesh.CollidesWith(capsule, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)

	// Partially encompassing capsule (could potentially divide into more cases, but this (only face collisions) should be most restrictive)
	capsule, err = NewCapsule(NewPose(r3.Vector{X: 0.2, Y: 0.2, Z: 0},
		NewZeroOrientation()), 0.1, 0.3, "")
	test.That(t, err, test.ShouldBeNil)
	collides, err = mesh.CollidesWith(capsule, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)

	// Completely encompassing capsule, no boundary collision
	capsule, err = NewCapsule(NewZeroPose(), 2, 4.5, "")
	test.That(t, err, test.ShouldBeNil)
	collides, err = mesh.CollidesWith(capsule, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)

	// Non-overlapping capsule
	capsule, err = NewCapsule(NewPose(r3.Vector{X: -1.1, Y: -1.1, Z: 0},
		NewZeroOrientation()), 1, 3, "")
	test.That(t, err, test.ShouldBeNil)
	collides, err = mesh.CollidesWith(capsule, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeFalse)
}

func TestMeshCollidesWithBox(t *testing.T) {

	mesh := makeSimpleTriangleMesh()
	// why is encompassing only specified as a special case for box? (what makes box different?)!
	// ^ this is quite confusing because what if you just make a mesh that looks like a meshified box?
	// AHHHHHHHHHH

	// non-point collisions (e.g., edges with more than a point of overlap) are redundant with point collisions
	// types of triangle points: {vertex, edge, face}
	// types of box points: {vertex, edge, face}
	// exhaust the 9 collision options:

	// Collision with triangle vertex
	// Box vertex collision (with triangle vertex)
	t.Run("Box vertex against triangle vertex", func(t *testing.T) {
		box, err := NewBox(NewPose(r3.Vector{X: 1.5, Y: 0.5, Z: 0.5}, NewZeroOrientation()),
			r3.Vector{X: 1, Y: 1, Z: 1}, "")
		test.That(t, err, test.ShouldBeNil)
		collides, err := mesh.CollidesWith(box, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeTrue)
	})

	// Box edge collision (with triangle vertex)
	t.Run("Box edge against triangle vertex", func(t *testing.T) {
		box, err := NewBox(NewPose(r3.Vector{X: 1.5, Y: 0, Z: 0.5}, NewZeroOrientation()),
			r3.Vector{X: 1, Y: 1, Z: 1}, "")
		test.That(t, err, test.ShouldBeNil)
		collides, err := mesh.CollidesWith(box, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeTrue)
	})

	// Box face collision (with triangle vertex)
	t.Run("Box face against triangle vertex", func(t *testing.T) {
		box, err := NewBox(NewPose(r3.Vector{X: 1.5, Y: 0, Z: 0}, NewZeroOrientation()),
			r3.Vector{X: 1, Y: 1, Z: 1}, "")
		test.That(t, err, test.ShouldBeNil)
		collides, err := mesh.CollidesWith(box, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeTrue)
	})

	// Collision with triangle edge
	// Box vertex collision (with triangle edge)
	t.Run("Box vertex against triangle edge", func(t *testing.T) {
		box, err := NewBox(NewPose(r3.Vector{X: 0.7, Y: 1.5 - 0.7*(3.0/2), Z: 0.5}, NewZeroOrientation()), // idk how to do orientation loool
			r3.Vector{X: 1, Y: 1, Z: 1}, "")
		test.That(t, err, test.ShouldBeNil)
		collides, err := mesh.CollidesWith(box, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeTrue)
	})
	// Box edge collision (with triangle edge)
	// TODO: revise this (just rotate 45deg about z axis) to make it more readable
	// I guess revision is like orientation Theta: math.Pi/4, then shift by half-diagonals (sqrt(2)/2)...
	t.Run("Box edge against triangle edge", func(t *testing.T) {
		box, err := NewBox(NewPose(r3.Vector{X: 0.5, Y: -0.5, Z: 0.5},
			&OrientationVector{Theta: math.Pi / 4, OY: 1, OZ: 1}), r3.Vector{X: 1, Y: 1, Z: 1}, "")
		test.That(t, err, test.ShouldBeNil)
		collides, err := mesh.CollidesWith(box, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeTrue)
	})

	t.Run("example edge collision failure", func(t *testing.T) {
		ABC := makeTestMesh(NewZeroOrientation(), r3.Vector{},
			[]*Triangle{NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: 0},
				r3.Vector{X: 1, Y: 0, Z: 0},
				r3.Vector{X: 0, Y: 1, Z: 0},
			)})
		DEF := makeTestMesh(NewZeroOrientation(), r3.Vector{},
			[]*Triangle{NewTriangle(
				r3.Vector{X: 0.5, Y: 0, Z: 0.5},
				r3.Vector{X: 0.5, Y: 0, Z: -0.5},
				r3.Vector{X: 0.5, Y: -1, Z: 0},
			)})
		collides, err := ABC.CollidesWith(DEF, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeTrue)
	})

	t.Run("debug edge collision", func(t *testing.T) {
		// continuing to litter all over this file, need to clean up

		p0 := r3.Vector{X: 0, Y: 0, Z: 0}
		p1 := r3.Vector{X: 0, Y: 1, Z: 0}
		p2 := r3.Vector{X: 1, Y: 0, Z: 0}
		basicTri := NewTriangle(p0, p1, p2)
		t0 := r3.Vector{X: 0.5, Y: 0, Z: 0.5}
		t1 := r3.Vector{X: 0.5, Y: 0, Z: -0.5}
		t2 := r3.Vector{X: 0.5, Y: -1, Z: 0}
		basicTri2 := NewTriangle(t0, t1, t2)

		mesh1 := makeTestMesh(NewZeroOrientation(), r3.Vector{},
			[]*Triangle{basicTri})
		mesh2 := makeTestMesh(NewZeroOrientation(), r3.Vector{},
			[]*Triangle{basicTri2})

		fmt.Println(mesh1.collidesWithMesh(mesh2, defaultCollisionBufferMM))
		fmt.Println(closestPointsSegmentTriangle(t0, t1, basicTri))
		fmt.Println(closestPointsSegmentTriangle(p0, p2, basicTri2))
		// looks like tendency is to overestimate the "t" parameter on the segment
		// no, underestimate

		segPt, _ := closestPointsSegmentPlane(t0, t1, basicTri.p0, basicTri.normal)
		triPt, inside := closestTriangleInsidePoint(basicTri, segPt)
		fmt.Println(segPt)
		fmt.Println(triPt)
		fmt.Println(inside)
		// looks like we are explicitly making an error here inside closestPointsSegmentPlane
		fmt.Println("BUFFER")
		fmt.Println(basicTri.normal) // no error here

		segVec := t1.Sub(t0)
		d := basicTri.p0.Dot(basicTri.normal)
		denom := basicTri.normal.Dot(segVec)
		time := (d - basicTri.normal.Dot(t0)) / (denom) //  + 1e-6 causing errors
		coplanarPt := segVec.Mul(time).Add(t0)
		fmt.Println(time)
		fmt.Println(coplanarPt)

		// yay! it really just is that denominator adjustment

		// test.That(t, R3VectorAlmostEqual(bestSegPt, r3.Vector{0.5, 0, 0}, 1e-7), test.ShouldBeTrue)
		// test.That(t, bestSegPt, test.ShouldAlmostEqual, r3.Vector{0.5, 0, 0})
	})

	//hmm....
	// ok so dummy_mesh does intersect, mesh does not !!!
	// dummy_mesh := makeTestMesh(NewZeroOrientation(), r3.Vector{},
	// 	[]*Triangle{NewTriangle(
	// 		r3.Vector{X: 0.5, Y: 0, Z: 0},
	// 		r3.Vector{X: 0, Y: 1, Z: 0},
	// 		r3.Vector{X: 1, Y: 0, Z: 0},
	// 	)})
	// box, err := NewBox(NewPose(r3.Vector{X: 0.5, Y: -0.5, Z: 0.5},
	// 	&OrientationVector{Theta: math.Pi / 4, OY: 1, OZ: 1}), r3.Vector{X: 1, Y: 1, Z: 1}, "")
	// test.That(t, err, test.ShouldBeNil)
	// collides, err := dummy_mesh.CollidesWith(box, defaultCollisionBufferMM)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, collides, test.ShouldBeTrue)

	// now lets try 2 perpendicular triangles
	t.Run("Box edge against two triangle edges", func(t *testing.T) {
		tri1 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		tri2 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 0, Z: -1},
		)
		extendedMesh := makeTestMesh(NewZeroOrientation(), r3.Vector{}, []*Triangle{tri1, tri2})
		box, err := NewBox(NewPose(r3.Vector{X: 0.5, Y: -0.5, Z: 0.5},
			&OrientationVector{Theta: math.Pi / 4, OY: 1, OZ: 1}), r3.Vector{X: 1, Y: 1, Z: 1}, "")
		test.That(t, err, test.ShouldBeNil)
		collides, err := extendedMesh.CollidesWith(box, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeTrue)
	})

	// try again with mesh, slight indentation
	// slight indentation FAILS
	t.Run("Box edge into triangle edge (slight indentation)", func(t *testing.T) {
		box, err := NewBox(NewPose(r3.Vector{X: 0.5, Y: -0.5, Z: 0.5},
			&OrientationVector{Theta: math.Pi / 4, OY: 1, OZ: 1}), r3.Vector{X: 1, Y: 1, Z: 1}, "")
		test.That(t, err, test.ShouldBeNil)
		collides, err := mesh.CollidesWith(box, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeTrue)
	})

	// Box face collision (with triangle edge) (redundant)
	// box, err = NewBox(NewPose(r3.Vector{X: -0.5, Y: 0.5, Z: 0}, NewZeroOrientation()),
	// 	r3.Vector{X: 1, Y: 1, Z: 1}, "")
	// test.That(t, err, test.ShouldBeNil)
	// collides, err = mesh.CollidesWith(box, defaultCollisionBufferMM)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, collides, test.ShouldBeTrue)

	// Collision with triangle face
	// Box vertex collision (with triangle face)
	// Box edge collision (with triangle face)
	// Box face collision (with triangle face)

	// Partially encompassing box, no vertex collisions
	// REVIST THIS TEST, BUT IT SHOULD BE USELESS
	// box, err = NewBox(NewZeroPose(),
	// 	r3.Vector{X: 1, Y: 1, Z: 1}, "")
	// test.That(t, err, test.ShouldBeNil)
	// collides, err = mesh.CollidesWith(box, defaultCollisionBufferMM)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, collides, test.ShouldBeTrue)

	// Completely encompassing box, no boundary collision
	t.Run("Box strictly encompassing triangle", func(t *testing.T) {
		box, err := NewBox(NewZeroPose(),
			r3.Vector{X: 4, Y: 4, Z: 4}, "")
		test.That(t, err, test.ShouldBeNil)
		collides, err := mesh.CollidesWith(box, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeTrue)
	})

	// // Create overlapping box
	// box, err = NewBox(NewZeroPose(), r3.Vector{X: 1, Y: 1, Z: 1}, "")
	// test.That(t, err, test.ShouldBeNil)

	// collides, err = mesh.CollidesWith(box, defaultCollisionBufferMM)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, collides, test.ShouldBeTrue)

	// Create non-overlapping box
	t.Run("Box vertex not touching triangle", func(t *testing.T) {
		box, err := NewBox(NewPose(r3.Vector{X: 2, Y: 2, Z: 2}, NewZeroOrientation()),
			r3.Vector{X: 1, Y: 1, Z: 1}, "")
		test.That(t, err, test.ShouldBeNil)

		collides, err := mesh.CollidesWith(box, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeFalse)
	})
}

// e.g., could write something like the below:
// func TestMeshCollisionExpectation(t *testing.T, mesh *Mesh, g Geometry, expected bool) {
// 	collides, err := mesh.CollidesWith(g, defaultCollisionBufferMM)
// 	test.That(t, err, test.ShouldBeNil)
// 	if expected {
// 		test.That(t, collides, test.ShouldBeTrue)
// 	} else {
// 		test.That(t, collides, test.ShouldBeFalse)
// 	}
// }
// BUT this doesnt seem like licit function for the testing file

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
	test.That(t, len(mesh.Triangles()), test.ShouldEqual, 3)

	// Verify points match triangle vertices
	expectedPoints := []r3.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 0, Z: 0},
		{X: 0, Y: 1, Z: 0},
		{X: 0.6, Y: 0.6, Z: 0},
		{X: 0, Y: 0, Z: 10},
		{X: 1, Y: 0, Z: 10},
		{X: 0, Y: 1, Z: 10},
		mesh.Triangles()[0].Centroid(),
		mesh.Triangles()[1].Centroid(),
		mesh.Triangles()[2].Centroid(),
	}

	points := mesh.ToPoints(1)
	test.That(t, len(points), test.ShouldEqual, len(expectedPoints))
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
