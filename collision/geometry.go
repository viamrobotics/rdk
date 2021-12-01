package collision

import (
	"math"

	"github.com/golang/geo/r3"

	spatial "go.viam.com/core/spatialmath"
)

type Box struct {
	Position r3.Vector
	Axes     axes
	HalfSize r3.Vector
}

type axes struct {
	X r3.Vector
	Y r3.Vector
	Z r3.Vector
}

// Initialize a new 3D box from a pose and a half size vector
func NewBox(center spatial.Pose, halfSize r3.Vector) *Box {
	rm := center.Orientation().RotationMatrix()
	return &Box{center.Point(), axes{rm.Row(0), rm.Row(1), rm.Row(2)}, halfSize}
}

// BoxVsBox takes two Boxes as arguments and returns a bool describing if they are in collision
// reference: https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
func BoxVsBox(a, b *Box) bool {
	positionDelta := a.Position.Sub(b.Position)
	return !(separatingPlaneTest(positionDelta, a.Axes.X, a, b) ||
		separatingPlaneTest(positionDelta, a.Axes.Y, a, b) ||
		separatingPlaneTest(positionDelta, a.Axes.Z, a, b) ||
		separatingPlaneTest(positionDelta, b.Axes.X, a, b) ||
		separatingPlaneTest(positionDelta, b.Axes.Y, a, b) ||
		separatingPlaneTest(positionDelta, b.Axes.Z, a, b) ||
		separatingPlaneTest(positionDelta, a.Axes.X.Cross(b.Axes.X), a, b) ||
		separatingPlaneTest(positionDelta, a.Axes.X.Cross(b.Axes.Y), a, b) ||
		separatingPlaneTest(positionDelta, a.Axes.X.Cross(b.Axes.Z), a, b) ||
		separatingPlaneTest(positionDelta, a.Axes.Y.Cross(b.Axes.X), a, b) ||
		separatingPlaneTest(positionDelta, a.Axes.Y.Cross(b.Axes.Y), a, b) ||
		separatingPlaneTest(positionDelta, a.Axes.Y.Cross(b.Axes.Z), a, b) ||
		separatingPlaneTest(positionDelta, a.Axes.Z.Cross(b.Axes.X), a, b) ||
		separatingPlaneTest(positionDelta, a.Axes.Z.Cross(b.Axes.Y), a, b) ||
		separatingPlaneTest(positionDelta, a.Axes.Z.Cross(b.Axes.Z), a, b))
}

// Helper function to check if there is a separating plane in between the selected axes
// reference: https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
func separatingPlaneTest(positionDelta, plane r3.Vector, a, b *Box) bool {
	return math.Abs(positionDelta.Dot(plane)) > (math.Abs(a.Axes.X.Mul(a.HalfSize.X).Dot(plane)) +
		math.Abs(a.Axes.Y.Mul(a.HalfSize.Y).Dot(plane)) +
		math.Abs(a.Axes.Z.Mul(a.HalfSize.Z).Dot(plane)) +
		math.Abs(b.Axes.X.Mul(b.HalfSize.X).Dot(plane)) +
		math.Abs(b.Axes.Y.Mul(b.HalfSize.Y).Dot(plane)) +
		math.Abs(b.Axes.Z.Mul(b.HalfSize.Z).Dot(plane)))
}
