package config

import (
	"encoding/json"
	"fmt"

	spatial "go.viam.com/core/spatialmath"
)

// OrientationType defines what orientation representations are known
type OrientationType string

// The set of allowed representations for orientation
const (
	OrientationVectorDegrees = OrientationType("ov_degrees")
	OrientationVectorRadians = OrientationType("ov_radians")
	EulerAngles              = OrientationType("euler_angles")
	AxisAngles               = OrientationType("axis_angles")
)

// Translation is the translation between two objects in the grid system. It is always in millimeters.
type Translation struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

/*
Frame contains the information of the pose and parent of the frame that will be created.
The Orientation field is an interface. When writing a config file, the orientation field should be of the form
{
	"orientation" : {
		"type": "orientation_type"
		"value" : {
			"param0" : ...,
			"param1" : ...,
			etc.
		}
	}
}
*/
type Frame struct {
	Parent      string              `json:"parent"`
	Translation Translation         `json:"translation"`
	Orientation spatial.Orientation `json:"orientation"`
}

// rawOrientation holds the underlying type of orientation, and the value.
type rawOrientation struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// UnmarshalJSON will parse the Orientation field into a spatial.Orientation object from a json.rawMessage
func (fc *Frame) UnmarshalJSON(b []byte) error {
	temp := struct {
		Parent      string          `json:"parent"`
		Translation Translation     `json:"translation"`
		Orientation json.RawMessage `json:"orientation"`
	}{}

	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}
	orientation, err := parseOrientation(temp.Orientation)
	if err != nil {
		return err
	}
	fc.Parent = temp.Parent
	fc.Translation = temp.Translation
	fc.Orientation = orientation
	return nil
}

// parseOrientation will use the Type in rawOrientation to unmarshal the Value into the correct struct that implements Orientation.
func parseOrientation(j json.RawMessage) (spatial.Orientation, error) {
	// if there is no Orientation field, return a zero orientation
	if len(j) == 0 {
		return spatial.NewZeroOrientation(), nil
	}

	temp := rawOrientation{}
	err := json.Unmarshal(j, &temp)
	if err != nil {
		return nil, err
	}

	// use the type to unmarshal the value
	switch OrientationType(temp.Type) {
	case OrientationVectorDegrees:
		var o spatial.OrientationVectorDegrees
		err = json.Unmarshal(temp.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case OrientationVectorRadians:
		var o spatial.OrientationVector
		err = json.Unmarshal(temp.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case AxisAngles:
		var o spatial.R4AA
		err = json.Unmarshal(temp.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case EulerAngles:
		var o spatial.EulerAngles
		err = json.Unmarshal(temp.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	default:
		return nil, fmt.Errorf("orientation type %s not recognized", temp.Type)
	}
}
