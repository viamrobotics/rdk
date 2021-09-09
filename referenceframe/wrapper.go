package referenceframe

import (
	spatial "go.viam.com/core/spatialmath"
)

// A FrameWrapper will wrap a single Frame, allowing a static offset to be set
type FrameWrapper struct {
	Frame
	offset spatial.Pose
}

// WrapFrame will take a frame and wrap it to have the given additional offset.
func WrapFrame(frame Frame, offset spatial.Pose) *FrameWrapper {
	return &FrameWrapper{
		Frame:  frame,
		offset: offset,
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
