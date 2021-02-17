package mdl

import (
	"github.com/viamrobotics/robotcore/kinematics/go-kin/kinmath"
	"github.com/viamrobotics/robotcore/kinematics/go-kin/kinmath/spatial"
	"gonum.org/v1/gonum/graph/simple"
)

// TODO: add more descriptive field names once I work out what they ought to be
type Frame struct {
	//~ Element
	a spatial.MotionVector
	c spatial.MotionVector
	f spatial.ForceVector
	//~ i              spatial.RigidBodyInertia
	//~ iA             spatial.ArticulatedBodyInertia
	pA spatial.ForceVector
	i  Transform
	v  spatial.MotionVector
	//~ x              spatial.PlueckerTransform
	descriptor    simple.Node
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
	f.i.t = kinmath.NewTransform()
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
// Why is it here? Because Robotics Library has it
func (f *Frame) ForwardPosition() {
}
func (f *Frame) ForwardVelocity() {
}

func (f *Frame) GetVelocityVector() spatial.MotionVector {
	return f.v
}
