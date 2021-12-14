package referenceframe

import (
	"math"

	"github.com/go-errors/errors"
	"github.com/golang/geo/r3"

	spatial "go.viam.com/core/spatialmath"
)

// boxCreator implements the VolumeCreator interface for box structs
type boxCreator struct {
	halfSize r3.Vector
	offset   r3.Vector
}

// box is a collision geometry that represents a 3D rectangular prism, it has a pose and half size that fully define it
type box struct {
	pose     spatial.Pose
	halfSize r3.Vector
}

// NewBox instantiates a boxCreator class, which allows instantiating boxes given only a pose.
// These boxes have dimensions given by the provided halfSize vector
func NewBox(halfSize r3.Vector) *boxCreator {
	return &boxCreator{halfSize, r3.Vector{}}
}

// NewBoxFromOffset instantiates a boxCreator class, which allows instantiating boxes given only a pose which is applied
// at the specified offset from the pose. These boxes have dimensions given by the provided halfSize vector
func NewBoxFromOffset(halfSize, offset r3.Vector) *boxCreator {
	return &boxCreator{halfSize, offset}
}

// NewVolme instantiates a new box from a boxCreator class
func (bc *boxCreator) NewVolume(pose spatial.Pose) (*box, error) {
	fs := NewEmptySimpleFrameSystem("")
	link, err := NewStaticFrame("", pose)
	if err != nil {
		return nil, err
	}

	err = fs.AddFrame(link, fs.World())
	if err != nil {
		return nil, err
	}

	center, err := fs.TransformPoint(nil, bc.offset, link, fs.World())
	if err != nil {
		return nil, err
	}

	b := &box{}
	b.pose = spatial.NewPoseFromOrientation(center, pose.Orientation())
	b.halfSize = bc.halfSize
	return b, nil
}

// CollidesWith checks if the given box collides with the given volume and returns true if it does
func (b box) CollidesWith(v Volume) (bool, error) {
	if other, ok := v.(box); ok {
		return boxVsBox(&b, &other), nil
	}
	return true, errors.Errorf("collisions between box and %T are not supported", v)
}

// boxVsBox takes two Boxes as arguments and returns a bool describing if they are in collision
// reference: https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
func boxVsBox(a, b *box) bool {
	positionDelta := spatial.PoseDelta(a.pose, b.pose).Point()
	rmA := a.pose.Orientation().RotationMatrix()
	rmB := b.pose.Orientation().RotationMatrix()
	return !(separatingPlaneTest(positionDelta, rmA.Row(0), a, b) ||
		separatingPlaneTest(positionDelta, rmA.Row(1), a, b) ||
		separatingPlaneTest(positionDelta, rmA.Row(2), a, b) ||
		separatingPlaneTest(positionDelta, rmB.Row(0), a, b) ||
		separatingPlaneTest(positionDelta, rmB.Row(1), a, b) ||
		separatingPlaneTest(positionDelta, rmB.Row(2), a, b) ||
		separatingPlaneTest(positionDelta, rmA.Row(0).Cross(rmB.Row(0)), a, b) ||
		separatingPlaneTest(positionDelta, rmA.Row(0).Cross(rmB.Row(1)), a, b) ||
		separatingPlaneTest(positionDelta, rmA.Row(0).Cross(rmB.Row(2)), a, b) ||
		separatingPlaneTest(positionDelta, rmA.Row(1).Cross(rmB.Row(0)), a, b) ||
		separatingPlaneTest(positionDelta, rmA.Row(1).Cross(rmB.Row(1)), a, b) ||
		separatingPlaneTest(positionDelta, rmA.Row(1).Cross(rmB.Row(2)), a, b) ||
		separatingPlaneTest(positionDelta, rmA.Row(2).Cross(rmB.Row(0)), a, b) ||
		separatingPlaneTest(positionDelta, rmA.Row(2).Cross(rmB.Row(1)), a, b) ||
		separatingPlaneTest(positionDelta, rmA.Row(2).Cross(rmB.Row(2)), a, b))
}

// Helper function to check if there is a separating plane in between the selected axes
// reference: https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
func separatingPlaneTest(positionDelta, plane r3.Vector, a, b *box) bool {
	rmA := a.pose.Orientation().RotationMatrix()
	rmB := b.pose.Orientation().RotationMatrix()
	return math.Abs(positionDelta.Dot(plane)) > (math.Abs(rmA.Row(0).Mul(a.halfSize.X).Dot(plane)) +
		math.Abs(rmA.Row(1).Mul(a.halfSize.Y).Dot(plane)) +
		math.Abs(rmA.Row(2).Mul(a.halfSize.Z).Dot(plane)) +
		math.Abs(rmB.Row(0).Mul(b.halfSize.X).Dot(plane)) +
		math.Abs(rmB.Row(1).Mul(b.halfSize.Y).Dot(plane)) +
		math.Abs(rmB.Row(2).Mul(b.halfSize.Z).Dot(plane)))
}
