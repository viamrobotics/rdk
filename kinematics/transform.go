package kinematics

import (
	"go.viam.com/robotcore/kinematics/kinmath"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/num/dualquat"
)

type Transform struct {
	in         *Frame
	out        *Frame
	t          *kinmath.QuatTrans
	descriptor graph.Edge
	name       string
}

func NewTransform() *Transform {
	t := Transform{}
	t.t = kinmath.NewQuatTrans()
	return &t
}

func (t *Transform) SetEdgeDescriptor(edge graph.Edge) {
	t.descriptor = edge
}

func (t *Transform) GetEdgeDescriptor() graph.Edge {
	return t.descriptor
}

func (t *Transform) SetName(name string) {
	t.name = name
}

func (t *Transform) GetName() string {
	return t.name
}

func (t *Transform) SetIn(in *Frame) {
	t.in = in
}

func (t *Transform) GetIn() *Frame {
	return t.in
}

func (t *Transform) SetOut(out *Frame) {
	t.out = out
}

func (t *Transform) GetOut() *Frame {
	return t.out
}

func (t *Transform) ForwardPosition() {
	t.out.i.t.Quat = t.in.i.t.Transformation(t.t.Quat)
}

//
func (t *Transform) ForwardVelocity() {
	t.out.v = dualquat.Mul(t.in.v, t.t.Quat)
}
