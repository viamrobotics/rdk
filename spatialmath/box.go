package spatialmath

import (
	"math"

	"github.com/go-errors/errors"
	"github.com/golang/geo/r3"
)

// BoxCreator implements the VolumeCreator interface for box structs
type boxCreator struct {
	halfSize r3.Vector
	offset   Pose
}

// box is a collision geometry that represents a 3D rectangular prism, it has a pose and half size that fully define it
type box struct {
	pose     Pose
	halfSize r3.Vector
}

// NewBox instantiates a BoxCreator class, which allows instantiating boxes given only a pose.
// These boxes have dimensions given by the provided halfSize vector
func NewBox(halfSize r3.Vector) VolumeCreator {
	return &boxCreator{halfSize, NewZeroPose()}
}

// NewBoxFromOffset instantiates a BoxCreator class, which allows instantiating boxes given only a pose which is applied
// at the specified offset from the pose. These boxes have dimensions given by the provided halfSize vector
func NewBoxFromOffset(halfSize r3.Vector, offset Pose) VolumeCreator {
	return &boxCreator{halfSize, offset}
}

// NewVolume instantiates a new box from a BoxCreator class
func (bc *boxCreator) NewVolume(pose Pose) Volume {
	b := &box{}
	b.pose = Compose(pose, bc.offset)
	b.halfSize = bc.halfSize
	return b
}

// CollidesWith checks if the given box collides with the given volume and returns true if it does
func (b *box) CollidesWith(v Volume) (bool, error) {
	if other, ok := v.(*box); ok {
		return boxVsBoxCollision(b, other), nil
	}
	return true, errors.Errorf("collisions between box and %T are not supported", v)
}

// boxVsBox takes two Boxes as arguments and returns a bool describing if they are in collision,
// true == collision, false == no collision
// reference: https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
func boxVsBoxCollision(a, b *box) bool {
	positionDelta := PoseDelta(a.pose, b.pose).Point()
	rmA := a.pose.Orientation().RotationMatrix()
	rmB := b.pose.Orientation().RotationMatrix()
	for i := 0; i < 3; i++ {
		if hasSeparatingPlane(positionDelta, rmA.Row(i), a, b) {
			return false
		}
		if hasSeparatingPlane(positionDelta, rmB.Row(i), a, b) {
			return false
		}
		for j := 0; j < 3; j++ {
			if hasSeparatingPlane(positionDelta, rmA.Row(i).Cross(rmB.Row(j)), a, b) {
				return false
			}
		}
	}
	return true
}

// Helper function to check if there is a separating plane in between the selected axes.  Per the separating hyperplane
// theorem, if such a plane exists (and true is returned) this proves that there is no collision between the boxes
// references: https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
//             https://gamedev.stackexchange.com/questions/25397/obb-vs-obb-collision-detection
func hasSeparatingPlane(positionDelta, plane r3.Vector, a, b *box) bool {
	rmA := a.pose.Orientation().RotationMatrix()
	rmB := b.pose.Orientation().RotationMatrix()
	return math.Abs(positionDelta.Dot(plane)) > (math.Abs(rmA.Row(0).Mul(a.halfSize.X).Dot(plane)) +
		math.Abs(rmA.Row(1).Mul(a.halfSize.Y).Dot(plane)) +
		math.Abs(rmA.Row(2).Mul(a.halfSize.Z).Dot(plane)) +
		math.Abs(rmB.Row(0).Mul(b.halfSize.X).Dot(plane)) +
		math.Abs(rmB.Row(1).Mul(b.halfSize.Y).Dot(plane)) +
		math.Abs(rmB.Row(2).Mul(b.halfSize.Z).Dot(plane)))
}
