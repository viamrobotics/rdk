package referenceframe

import (
	"encoding/json"
	"fmt"

	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// FrameMapConfig represents the format for configuring a Frame object.
type FrameMapConfig map[string]interface{}

// UnmarshalFrameJSON deserialized json into a reference referenceframe.
func UnmarshalFrameJSON(data []byte) (Frame, error) {
	config := FrameMapConfig{}
	err := json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return config.ParseConfig()
}

// ParseConfig converts a FrameMapConfig to a Frame object.
func (config FrameMapConfig) ParseConfig() (Frame, error) {
	var err error

	switch config["type"] {
	case "static":
		f := staticFrame{}
		var ok bool
		f.name, ok = config["name"].(string)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(f.name, config["name"])
		}

		pose, ok := config["transform"].(map[string]interface{})
		if !ok {
			return nil, utils.NewUnexpectedTypeError(pose, config["transform"])
		}
		f.transform, err = decodePose(pose)
		if err != nil {
			return nil, fmt.Errorf("error decoding transform (%v) %w", config["transform"], err)
		}
		return &f, nil
	case "translational":
		f := translationalFrame{}
		var ok bool
		f.name, ok = config["name"].(string)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(f.name, config["name"])
		}
		err := mapstructure.Decode(config["transAxis"], &f.transAxis)
		if err != nil {
			return nil, err
		}
		err = mapstructure.Decode(config["limit"], &f.limit)
		if err != nil {
			return nil, err
		}
		return &f, nil
	case "rotational":
		f := rotationalFrame{}
		var ok bool
		f.name, ok = config["name"].(string)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(f.name, config["name"])
		}

		rotAxis, ok := config["rotAxis"].(map[string]interface{})
		if !ok {
			return nil, utils.NewUnexpectedTypeError(rotAxis, config["rotAxis"])
		}

		f.rotAxis.X, ok = rotAxis["X"].(float64)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(f.rotAxis.X, rotAxis["X"])
		}
		f.rotAxis.Y, ok = rotAxis["Y"].(float64)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(f.rotAxis.Y, rotAxis["Y"])
		}
		f.rotAxis.Z, ok = rotAxis["Z"].(float64)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(f.rotAxis.Z, rotAxis["Z"])
		}

		err = mapstructure.Decode(config["limit"], &f.limit)
		if err != nil {
			return nil, err
		}
		return &f, nil

	default:
		return nil, fmt.Errorf("no frame type: [%v]", config["type"])
	}
}

func decodePose(config FrameMapConfig) (spatial.Pose, error) {
	var point r3.Vector

	err := mapstructure.Decode(config["point"], &point)
	if err != nil {
		return nil, err
	}

	orientationMap, ok := config["orientation"].(map[string]interface{})
	if !ok {
		return nil, utils.NewUnexpectedTypeError(orientationMap, config["orientation"])
	}
	oType, ok := orientationMap["type"].(string)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(oType, orientationMap["type"])
	}
	oValue, ok := orientationMap["value"].(map[string]interface{})
	if !ok {
		return nil, utils.NewUnexpectedTypeError(oValue, orientationMap["value"])
	}
	jsonValue, err := json.Marshal(oValue)
	if err != nil {
		return nil, err
	}

	orientation, err := (&spatial.OrientationConfig{spatial.OrientationType(oType), jsonValue}).ParseConfig()
	if err != nil {
		return nil, err
	}
	return spatial.NewPoseFromOrientation(point, orientation), nil
}
