package config

import (
	"encoding/json"

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

// OrientationConfig specifies the type of orientation representation that is used, and the orientation value
type OrientationConfig struct {
	Type  string              `json:"type"`
	Value spatial.Orientation `json:"value"`
}

// UnmarshalJSON will find the correct struct that implements Orientation
func (oc *OrientationConfig) UnmarshalJSON(b []byte) error {
	// unmarshal everything into a string:RawMessage pair
	var objMap map[string]interface{}
	var err error
	err = json.Unmarshal(b, &objMap)
	if err != nil {
		return err
	}

	oc.Type = objMap["type"].(string)

	switch oc.Type {
	case "ov_degrees":
		o := objMap["value"].(spatial.OrientationVecDegrees)
		oc.Value = &o
	case "ov_radians":
		o := objMap["value"].(spatial.OrientationVec)
		oc.Value = &o
	case "axis_angles":
		o := objMap["value"].(spatial.R4AA)
		oc.Value = &o
	case "euler_angles":
		o := objMap["value"].(spatial.EulerAngles)
		oc.Value = &o
	default:
		oc.Value = spatial.NewZeroOrientation()
	}
	return nil
}
