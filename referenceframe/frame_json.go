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
	name, ok := config["name"].(string)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(name, config["name"])
	}

	switch config["type"] {
	case "static":
		pose, ok := config["transform"].(map[string]interface{})
		if !ok {
			return nil, utils.NewUnexpectedTypeError(pose, config["transform"])
		}
		transform, err := decodePose(pose)
		if err != nil {
			return nil, err
		}
		return NewStaticFrame(name, transform)
	case "translational":
		var transAxis r3.Vector
		err := mapstructure.Decode(config["transAxis"], &transAxis)
		if err != nil {
			return nil, err
		}
		limits, err := decodeLimits(config)
		if err != nil {
			return nil, err
		}
		return NewTranslationalFrame(name, transAxis, limits[0])
	case "rotational":
		axis, err := decodeAxis(config)
		if err != nil {
			return nil, err
		}
		limits, err := decodeLimits(config)
		if err != nil {
			return nil, err
		}
		return NewRotationalFrame(name, axis, limits[0])
	case "linearly-actuated-rotational":
		axis, err := decodeAxis(config)
		if err != nil {
			return nil, err
		}
		limits, err := decodeLimits(config)
		if err != nil {
			return nil, err
		}
		var a, b float64
		err = mapstructure.Decode(config["a"], &a)
		if err != nil {
			return nil, err
		}
		err = mapstructure.Decode(config["b"], &b)
		if err != nil {
			return nil, err
		}
		return NewLinearlyActuatedRotationalFrame(name, axis, limits[0], a, b)
	case "mobile2D":
		limits, err := decodeLimits(config)
		if err != nil {
			return nil, err
		}
		return NewMobile2DFrame(name, limits)
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

	orientation, err := (&spatial.OrientationConfig{Type: spatial.OrientationType(oType), Value: jsonValue}).ParseConfig()
	if err != nil {
		return nil, err
	}
	return spatial.NewPoseFromOrientation(point, orientation), nil
}

func decodeAxis(config FrameMapConfig) (spatial.R4AA, error) {
	rotAxis, ok := config["rotAxis"].(map[string]interface{})
	if !ok {
		return spatial.R4AA{}, utils.NewUnexpectedTypeError(rotAxis, config["rotAxis"])
	}
	var axis spatial.R4AA
	axis.RX, ok = rotAxis["X"].(float64)
	if !ok {
		return spatial.R4AA{}, utils.NewUnexpectedTypeError(axis.RX, rotAxis["X"])
	}
	axis.RY, ok = rotAxis["Y"].(float64)
	if !ok {
		return spatial.R4AA{}, utils.NewUnexpectedTypeError(axis.RY, rotAxis["Y"])
	}
	axis.RZ, ok = rotAxis["Z"].(float64)
	if !ok {
		return spatial.R4AA{}, utils.NewUnexpectedTypeError(axis.RZ, rotAxis["Z"])
	}
	return axis, nil
}

func decodeLimits(config FrameMapConfig) ([]Limit, error) {
	var limit []Limit
	err := mapstructure.Decode(config["limit"], &limit)
	if err != nil {
		return []Limit{}, err
	}
	return limit, nil
}
