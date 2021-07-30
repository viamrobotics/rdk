package referenceframe

import (
	"go.viam.com/core/spatialmath"
)

// A FrameWrapper will wrap a Frame, allowing a new Parent to be set
type FrameWrapper struct{
	real Frame
	parent string
}

// Transform returns the quaternion associated with the wrapped frame
func (f *FrameWrapper) Transform(input []Input) *spatialmath.DualQuaternion {
	return f.real.Transform(input)
}

// Parent will return the name of the next transform up the kinematics chain from this frame
func (f *FrameWrapper) Parent() string {
	return f.parent
}

// Dof returns the dof of the wrapped frame
func (f *FrameWrapper) Dof() int {
	return f.real.Dof()
}
