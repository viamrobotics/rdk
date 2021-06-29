package kinematics

import (
	"go.viam.com/core/spatialmath"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/num/dualquat"
)

// Transform TODO
type Transform struct {
	in         *Frame
	out        *Frame
	t          *spatialmath.DualQuaternion
	descriptor graph.Edge
	name       string
	Rev        bool
}

// NewTransform TODO
func NewTransform() *Transform {
	t := Transform{}
	t.t = spatialmath.NewDualQuaternion()
	return &t
}

// SetEdgeDescriptor TODO
func (t *Transform) SetEdgeDescriptor(edge graph.Edge) {
	t.descriptor = edge
}

// GetEdgeDescriptor TODO
func (t *Transform) GetEdgeDescriptor() graph.Edge {
	return t.descriptor
}

// SetName TODO
func (t *Transform) SetName(name string) {
	t.name = name
}

// GetName TODO
func (t *Transform) GetName() string {
	return t.name
}

// SetIn TODO
func (t *Transform) SetIn(in *Frame) {
	t.in = in
}

// GetIn TODO
func (t *Transform) GetIn() *Frame {
	return t.in
}

// SetOut TODO
func (t *Transform) SetOut(out *Frame) {
	t.out = out
}

// GetOut TODO
func (t *Transform) GetOut() *Frame {
	return t.out
}

// ForwardPosition TODO
func (t *Transform) ForwardPosition() {
	if t.Rev {
		t.out.i.t.Quat = t.in.i.t.Transformation(dualquat.Conj(t.t.Quat))
	} else {
		t.out.i.t.Quat = t.in.i.t.Transformation(t.t.Quat)
	}
}

// ForwardVelocity TODO
func (t *Transform) ForwardVelocity() {
	if t.Rev {
		t.out.v = dualquat.Mul(t.in.v, dualquat.Conj(t.t.Quat))
	} else {
		t.out.v = dualquat.Mul(t.in.v, t.t.Quat)
	}

}
