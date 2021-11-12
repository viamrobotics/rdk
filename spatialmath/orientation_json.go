package spatialmath

import (
	"encoding/json"
)

// OrientationType defines what orientation representations are known
type OrientationType string

// The set of allowed representations for orientation
const (
	NoOrientation            = OrientationType("")
	OrientationVectorDegrees = OrientationType("ov_degrees")
	OrientationVectorRadians = OrientationType("ov_radians")
	EulerAngles              = OrientationType("euler_angles")
	AxisAngles               = OrientationType("axis_angles")
)

// RawOrientation holds the underlying type of orientation, and the value.
type RawOrientation struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// ParseOrientation will use the Type in RawOrientation to unmarshal the Value into the correct struct that implements Orientation.
func ParseOrientation(ro RawOrientation) (Orientation, error) {
	// use the type to unmarshal the value
	switch OrientationType(ro.Type) {
	case NoOrientation:
		return NewZeroOrientation(), nil
	case OrientationVectorDegrees:
		var o OrientationVectorDegrees
		err = json.Unmarshal(ro.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case OrientationVectorRadians:
		var o OrientationVector
		err = json.Unmarshal(ro.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case AxisAngles:
		var o R4AA
		err = json.Unmarshal(ro.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case EulerAngles:
		var o EulerAngles
		err = json.Unmarshal(ro.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	default:
		return nil, fmt.Errorf("orientation type %s not recognized", ro.Type)
	}
}
