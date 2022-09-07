package referenceframe

import "github.com/pkg/errors"

var (
	// OOBError is a string that all OOB errors should contain, so that they can be checked for distinct from other Transform errors.
	OOBError = errors.New("input out of bounds")

	// ParentMissingError returns an error indicating that the parent frame is nil.
	ParentMissingError = errors.New("parent frame is nil")

	NilPoseError = errors.New("pose is not allowed to be nil")
)

// NewIncorrectInputLengthError returns an error indicating that the length of the Innput array does not match the DoF of the frame
func NewIncorrectInputLengthError(actual, expected int) error {
	return errors.Errorf("number of inputs does not correspond to frame DoF, expected %d but got %d", expected, actual)
}

// NewFrameMissingError returns an error indicating that the given frame is missing from the framesystem.
func NewFrameMissingError(frameName string) error {
	return errors.Errorf("frame with name %q not in frame system", frameName)
}

// NewFrameAlreadyExistsError returns an error indicating that a frame of the given name already exists.
func NewFrameAlreadyExistsError(frameName string) error {
	return errors.Errorf("frame with name %q already in frame system", frameName)
}

func NewNilGeometryCreatorError(frame Frame) error {
	return errors.Errorf("frame of type %T has nil geometryCreator", frame)
}
