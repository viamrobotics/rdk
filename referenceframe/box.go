package referenceframe

import (
	"math"

	"github.com/go-errors/errors"
	"github.com/golang/geo/r3"
	spatial "go.viam.com/core/spatialmath"
)

type boxCreator struct {
	halfSize r3.Vector
	offset   r3.Vector
}

type box struct {
	position r3.Vector
	axes     axes
	halfSize r3.Vector
}

type axes struct {
	x r3.Vector
	y r3.Vector
	z r3.Vector
}

func NewBox(halfSize r3.Vector) *boxCreator {
	return &boxCreator{halfSize, r3.Vector{}}
}

func NewBoxFromOffset(halfSize, offset r3.Vector) *boxCreator {
	return &boxCreator{halfSize, offset}
}

func (bc *boxCreator) NewVolume(pose spatial.Pose) (*box, error) {
	fs := NewEmptySimpleFrameSystem("")
	link, err := NewStaticFrame("", pose)
	if err != nil {
		return nil, err
	}

	fs.AddFrame(link, fs.World())
	center, err := fs.TransformPoint(nil, bc.offset, link, fs.World())
	if err != nil {
		return nil, err
	}

	rm := pose.Orientation().RotationMatrix()

	b := &box{}
	b.position = center
	b.axes.x = rm.Row(0)
	b.axes.y = rm.Row(1)
	b.axes.z = rm.Row(2)
	b.halfSize = bc.halfSize
	return b, nil
}

func (b box) CollidesWith(v Volume) (bool, error) {
	if other, ok := v.(box); ok {
		return boxVsBox(&b, &other), nil
	}
	return true, errors.Errorf("collisions between box and %T are not supported", v)
}

// boxVsBox takes two Boxes as arguments and returns a bool describing if they are in collision
// reference: https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
func boxVsBox(a, b *box) bool {
	positionDelta := a.position.Sub(b.position)
	return !(separatingPlaneTest(positionDelta, a.axes.x, a, b) ||
		separatingPlaneTest(positionDelta, a.axes.y, a, b) ||
		separatingPlaneTest(positionDelta, a.axes.z, a, b) ||
		separatingPlaneTest(positionDelta, b.axes.x, a, b) ||
		separatingPlaneTest(positionDelta, b.axes.y, a, b) ||
		separatingPlaneTest(positionDelta, b.axes.z, a, b) ||
		separatingPlaneTest(positionDelta, a.axes.x.Cross(b.axes.x), a, b) ||
		separatingPlaneTest(positionDelta, a.axes.x.Cross(b.axes.y), a, b) ||
		separatingPlaneTest(positionDelta, a.axes.x.Cross(b.axes.z), a, b) ||
		separatingPlaneTest(positionDelta, a.axes.y.Cross(b.axes.x), a, b) ||
		separatingPlaneTest(positionDelta, a.axes.y.Cross(b.axes.y), a, b) ||
		separatingPlaneTest(positionDelta, a.axes.y.Cross(b.axes.z), a, b) ||
		separatingPlaneTest(positionDelta, a.axes.z.Cross(b.axes.x), a, b) ||
		separatingPlaneTest(positionDelta, a.axes.z.Cross(b.axes.y), a, b) ||
		separatingPlaneTest(positionDelta, a.axes.z.Cross(b.axes.z), a, b))
}

// Helper function to check if there is a separating plane in between the selected axes
// reference: https://gamedev.stackexchange.com/questions/112883/simple-3d-obb-collision-directx9-c
func separatingPlaneTest(positionDelta, plane r3.Vector, a, b *box) bool {
	return math.Abs(positionDelta.Dot(plane)) > (math.Abs(a.axes.x.Mul(a.halfSize.X).Dot(plane)) +
		math.Abs(a.axes.y.Mul(a.halfSize.Y).Dot(plane)) +
		math.Abs(a.axes.z.Mul(a.halfSize.Z).Dot(plane)) +
		math.Abs(b.axes.x.Mul(b.halfSize.X).Dot(plane)) +
		math.Abs(b.axes.y.Mul(b.halfSize.Y).Dot(plane)) +
		math.Abs(b.axes.z.Mul(b.halfSize.Z).Dot(plane)))
}
