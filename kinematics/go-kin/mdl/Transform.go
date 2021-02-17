package mdl

import (
	//~ "fmt"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/viamrobotics/robotcore/kinematics/go-kin/kinmath"
	"github.com/viamrobotics/robotcore/kinematics/go-kin/kinmath/spatial"
	"gonum.org/v1/gonum/graph"
)

type Transform struct {
	//~ Element
	in         *Frame
	out        *Frame
	t          *kinmath.Transform
	x          spatial.PlueckerTransform
	descriptor graph.Edge
	name       string
}

func NewTransform() *Transform {
	t := Transform{}
	t.t = kinmath.NewTransform()
	t.x.Rotation = mgl64.Ident3()
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
	t.out.i.t.Mat = t.in.i.t.Mat.Mul4(t.t.Mat)
	t.out.i.x = t.x.Mult(t.in.i.x)
}

func (t *Transform) ForwardVelocity() {
	t.out.v = t.x.MultMV(t.in.v)
}
