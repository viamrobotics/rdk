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
func (f *FrameWrapper) Transform(input []Input) (spatial.Pose, error) {
	wrappedPose, err := f.Frame.Transform(input)
	if err != nil && wrappedPose == nil {
		return nil, err
	}
	return spatial.Compose(f.offset, wrappedPose), err
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
func (f *FrameInverter) Transform(input []Input) (spatial.Pose, error) {
	wrappedPose, err := f.Frame.Transform(input)
	if err != nil && wrappedPose == nil {
		return nil, err
	}
	return wrappedPose.Invert(), err
}
