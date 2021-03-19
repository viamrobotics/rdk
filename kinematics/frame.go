package kinematics

import (
	"go.viam.com/robotcore/kinematics/kinmath"
	"go.viam.com/robotcore/kinematics/kinmath/spatial"
	//~ "gonum.org/v1/gonum/graph/simple"
)

// TODO(pl): add more descriptive field names once I work out what they ought to be
type Frame struct {

	a spatial.MotionVector
	i Transform
	v spatial.MotionVector
	IsWorld       bool
	IsBody        bool
	id            int64
	selfcollision map[*Frame]bool
	Name          string
}

func NewFrame() *Frame {
	f := Frame{}
	f.IsWorld = false
	f.IsBody = false
	f.i.t = kinmath.NewQuatTrans()
	f.selfcollision = make(map[*Frame]bool)
	return &f
}

func (f *Frame) GetVertexDescriptor() int64 {
	return f.id
}

func (f *Frame) SetVertexDescriptor(newID int64) {
	f.id = newID
}

// ForwardPosition does nothing in a frame- it is handled by *Transform
// Why is it here? Because we need it to implement Element.
// That said, we can probably completely remove Element in the future
func (f *Frame) ForwardPosition() {
}

func (f *Frame) ForwardVelocity() {
}

func (f *Frame) GetVelocityVector() spatial.MotionVector {
	return f.v
}
