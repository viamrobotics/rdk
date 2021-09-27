package config

import (
	"encoding/json"
	"fmt"

	spatial "go.viam.com/core/spatialmath"
)

// Translation is the translation between two objects in the grid system. It is always in millimeters.
type Translation struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// FrameConfig the pose and parent of the frame that will be created.
type FrameConfig struct {
	Parent      string             `json:"parent"`
	Translation Translation        `json:"translation"`
	Orientation *OrientationConfig `json:"orientation"`
}

// OrientationConfig specifies the type of orientation representation that is used, and the orientation value.
// The valid types are: "ov_degrees", "ov_radians", "euler_angles", and "axis_angles"
type OrientationConfig struct {
	Type  string              `json:"type"`
	Value spatial.Orientation `json:"value"`
}

// NewOrientation initializes an empty orientation config
func NewOrientation() *OrientationConfig {
	return &OrientationConfig{"", spatial.NewZeroOrientation()}
}

// UnmarshalJSON will set defaults for the FrameConfig if some fields are empty
func (fc *FrameConfig) UnmarshalJSON(b []byte) error {
	fc.Orientation = NewOrientation() // create a default orientation
	type Alias FrameConfig            // alias to prevent endless loop
	tmp := (*Alias)(fc)
	return json.Unmarshal(b, tmp)
}

// UnmarshalJSON will use the Type field in OrientationConfig to unmarshal into the correct struct that implements Orientation
func (oc *OrientationConfig) UnmarshalJSON(b []byte) error {
	// unmarshal everything into a string:RawMessage pair
	var objMap map[string]json.RawMessage
	var err error
	err = json.Unmarshal(b, &objMap)
	if err != nil {
		return err
	}

	// unmarshal the type
	var objType string
	err = json.Unmarshal(objMap["type"], &objType)
	if err != nil {
		return err
	}
	oc.Type = objType

	// use the type to unmarshal the value
	switch oc.Type {
	case "ov_degrees":
		var o spatial.OrientationVecDegrees
		err = json.Unmarshal(objMap["value"], &o)
		if err != nil {
			return err
		}
		oc.Value = &o
	case "ov_radians":
		var o spatial.OrientationVec
		err = json.Unmarshal(objMap["value"], &o)
		if err != nil {
			return err
		}
		oc.Value = &o
	case "axis_angles":
		var o spatial.R4AA
		err = json.Unmarshal(objMap["value"], &o)
		if err != nil {
			return err
		}
		oc.Value = &o
	case "euler_angles":
		var o spatial.EulerAngles
		err = json.Unmarshal(objMap["value"], &o)
		if err != nil {
			return err
		}
		oc.Value = &o
	default:
		return fmt.Errorf("orientation type %s not recognized", oc.Type)
	}
	return nil
}
