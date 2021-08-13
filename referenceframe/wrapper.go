package referenceframe

import (
	"go.viam.com/core/spatialmath"
)

// A FrameWrapper will wrap a Frame, allowing a new Parent to be set
type FrameWrapper struct {
	Frame
	offset *spatialmath.DualQuaternion
	parent string
}

// WrapFrame will take a frame and wrap it to have the given parent. Offset is not part of this constructor and must be
// set separately with SetOffset(). The reason for this is to ensure that if no offset is specified, the identity
// dual quaternion is used ((1,0,0,0),(0,0,0,0)) rather than someone passing in a spatialmath.DualQuaternion{}
func WrapFrame(frame Frame, parent string) *FrameWrapper {
	return &FrameWrapper{
		Frame:  frame,
		offset: spatialmath.NewDualQuaternion(),
		parent: parent,
	}
}

// Transform returns the quaternion associated with the wrapped frame, transformed by the
func (f *FrameWrapper) Transform(input []Input) *spatialmath.DualQuaternion {
	return &spatialmath.DualQuaternion{f.offset.Transformation(f.Frame.Transform(input).Number)}
}

// Parent will return the name of the next transform up the kinematics chain from this frame
func (f *FrameWrapper) Parent() string {
	return f.parent
}

// Dof returns the dof of the wrapped frame
func (f *FrameWrapper) SetOffset(offset *spatialmath.DualQuaternion) {
	f.offset = offset
}
