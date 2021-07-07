package kinematics

import (
	"go.viam.com/core/spatialmath"

	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

// Frame TODO
// TODO(pl): add more descriptive field names once I work out what they ought to be
type Frame struct {
	i             Transform
	v             dualquat.Number
	IsWorld       bool
	IsBody        bool
	id            int64
	selfcollision map[*Frame]bool
	Name          string
}

// NewFrame TODO
func NewFrame() *Frame {
	f := Frame{}
	f.IsWorld = false
	f.IsBody = false
	f.i.t = spatialmath.NewDualQuaternion()
	f.selfcollision = make(map[*Frame]bool)
	f.v = dualquat.Number{quat.Number{Real: 1}, quat.Number{}}
	return &f
}

// GetVertexDescriptor TODO
func (f *Frame) GetVertexDescriptor() int64 {
	return f.id
}

// SetVertexDescriptor TODO
func (f *Frame) SetVertexDescriptor(newID int64) {
	f.id = newID
}

// ForwardPosition does nothing in a frame- it is handled by *Transform
// Why is it here? Because we need it to implement Element.
// That said, we can probably completely remove Element in the future
// TODO(pl): Rem
func (f *Frame) ForwardPosition() {
}

// ForwardVelocity TODO
func (f *Frame) ForwardVelocity() {
}

// GetVelocity TODO
func (f *Frame) GetVelocity() dualquat.Number {
	return f.v
}
