package referenceframe

import (
	spatial "go.viam.com/core/spatialmath"
)

// A FrameWrapper will wrap a single Frame, allowing a static offset to be set
type FrameWrapper struct {
	Frame
	offset *spatial.DualQuaternion
}

// WrapFrame will take a frame and wrap it to have the given parent. Offset is not part of this constructor and must be
// set separately with SetOffset(). The reason for this is to ensure that if no offset is specified, the identity
// dual quaternion is used ((1,0,0,0),(0,0,0,0)) rather than someone passing in a spatialmath.DualQuaternion{}
func WrapFrame(frame, parent Frame) *FrameWrapper {
	return &FrameWrapper{
		Frame:  frame,
		offset: spatial.NewDualQuaternion(),
	}
}

// Transform returns the quaternion associated with the wrapped frame, transformed by the offset
func (f *FrameWrapper) Transform(input []Input) spatial.Pose {
	return spatial.Compose(f.offset, f.Frame.Transform(input))
}

// SetOffset sets the offset of the wrapped frame
func (f *FrameWrapper) SetOffset(offset *spatial.DualQuaternion) {
	f.offset = offset
}

// A FrameInverter will wrap a single Frame, inverting the transform 
type FrameInverter struct {
	Frame
}

// Transform returns the ConjQuat of the quaternion associated with the wrapped frame
func (f *FrameInverter) Transform(input []Input) spatial.Pose {
	return f.Transform(input).Invert()
}

//~ // A FrameSetWrapper will wrap any number of frames, allowing multiple dynamic frames to be combined into one for IK.
//~ // The frames wrapped MUST be a single, unbranched chain.
//~ type FrameSetWrapper struct {
	//~ frames []Frame
	//~ parent Frame
//~ }

//~ // WrapFrameSet will wrap up the given frames into a single frame. The 
//~ func WrapFrameSet(parent Frame, frames ...Frame) (*FrameSetWrapper, error) {
	
	//~ return &FrameSetWrapper{
		//~ frames:  frames,
		//~ parent: parent,
	//~ }, nil
//~ }

//~ // Transform returns the quaternion associated with the wrapped frame, transformed by the offset
//~ func (f *FrameSetWrapper) Transform(input []Input) *spatialmath.DualQuaternion {
	//~ return f.Transform(input)
//~ }

//~ // Parent will return the name of the next transform up the kinematics chain from this frame
//~ func (f *FrameSetWrapper) Parent() Frame {
	//~ return f.parent
//~ }
