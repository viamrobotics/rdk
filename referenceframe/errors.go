package referenceframe

import "github.com/pkg/errors"

// ErrCircularReference is an error indicating that a circular path exists somewhere between the end effector and the world.
var ErrCircularReference = errors.New("infinite loop finding path from end effector to world")

// ErrEmptyStringFrameName denotes an error when a frame with a name "" is specified.
var ErrEmptyStringFrameName = errors.New("frame with name \"\" cannot be used")

// ErrNilPoseInFrame denotes an error when a pose in frame is nil.
var ErrNilPoseInFrame = errors.New("pose in frame was nil")

// ErrNilPose denotes an error when a pose is nil.
var ErrNilPose = errors.New("pose was nil")

// ErrMarshalingHighDOFFrame describes the error when attempting to marshal a frame with multiple degrees of freedom.
var ErrMarshalingHighDOFFrame = errors.New("cannot marshal frame with >1 DOF, use a Model instead")

// ErrNoWorldConnection describes the error when a frame system is built but nothing is connected to the world node.
var ErrNoWorldConnection = errors.New("there are no robot parts that connect to a 'world' node. Root node must be named 'world'")

// NewParentFrameMissingError returns an error for when a part has named a parent whose part is missing from the collection of Parts
// that are becoming a FrameSystem object.
func NewParentFrameMissingError(partName, parentName string) error {
	return errors.Errorf("part with name %s references non-existent parent %s", partName, parentName)
}

// NewParentFrameNilError returns an error indicating that the parent frame is nil.
func NewParentFrameNilError(frameName string) error {
	return errors.New("frame with name %q has a parent that is nil")
}

// NewFrameMissingError returns an error indicating that the given frame is missing from the framesystem.
func NewFrameMissingError(frameName string) error {
	return errors.Errorf("frame with name %q not in frame system", frameName)
}

// NewFrameAlreadyExistsError returns an error indicating that a frame of the given name already exists.
func NewFrameAlreadyExistsError(frameName string) error {
	return errors.Errorf("frame with name %q already in frame system", frameName)
}

// NewIncorrectInputLengthError returns an error indicating that the length of the Innput array does not match the DoF of the frame.
func NewIncorrectInputLengthError(actual, expected int) error {
	return errors.Errorf("number of inputs does not match frame DoF, expected %d but got %d", expected, actual)
}

// NewUnsupportedJointTypeError returns an error indicating that a given joint type is not supported by current model parsing.
func NewUnsupportedJointTypeError(jointType string) error {
	return errors.Errorf("unsupported joint type detected: %q", jointType)
}

// NewDuplicateGeometryNameError returns an error indicating that multiple geometry names have attempted
// to be registered where this is not allowed.
func NewDuplicateGeometryNameError(name string) error {
	return errors.Errorf("cannot specify multiple geometries with the same name: %s", name)
}
