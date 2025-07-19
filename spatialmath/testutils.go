package spatialmath

import (
	"github.com/golang/geo/r3"
)

// MakeTestBox creates a test box with the given orientation, position, dimensions, and label.
func MakeTestBox(o Orientation, pt, dims r3.Vector, label string) Geometry {
	box, err := NewBox(NewPose(pt, o), dims, label)
	if err != nil {
		return nil
	}
	return box
}

// MakeTestCapsule creates a test capsule with the given orientation, position, radius, and length.
func MakeTestCapsule(o Orientation, pt r3.Vector, radius, length float64, label string) Geometry {
	c, err := NewCapsule(NewPose(pt, o), radius, length, label)
	if err != nil {
		return nil
	}
	return c
}

// MakeTestPoint creates a test point with the given position and label.
func MakeTestPoint(pt r3.Vector, label string) Geometry {
	return NewPoint(pt, label)
}

// MakeTestSphere creates a test sphere with the given position, radius, and label.
func MakeTestSphere(point r3.Vector, radius float64, label string) Geometry {
	sphere, err := NewSphere(NewPoseFromPoint(point), radius, label)
	if err != nil {
		return nil
	}
	return sphere
}

// MakeTestMesh creates a test mesh with the given orientation, position, and triangles.
func MakeTestMesh(o Orientation, pt r3.Vector, triangles []*Triangle, label string) Geometry {
	return NewMesh(NewPose(pt, o), triangles, label)
}

// MakeSimpleTriangleMesh creates a simple triangle mesh at origin for testing.
func MakeSimpleTriangleMesh(label string) Geometry {
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
	return MakeTestMesh(NewZeroOrientation(), r3.Vector{}, []*Triangle{tri1, tri2, tri3}, label)
}
