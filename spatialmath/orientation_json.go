package spatialmath

import (
	"encoding/json"
	"fmt"
)

// OrientationType defines what orientation representations are known.
type OrientationType string

// The set of allowed representations for orientation.
const (
	NoOrientationType            = OrientationType("")
	OrientationVectorDegreesType = OrientationType("ov_degrees")
	OrientationVectorRadiansType = OrientationType("ov_radians")
	EulerAnglesType              = OrientationType("euler_angles")
	AxisAnglesType               = OrientationType("axis_angles")
	QuaternionType               = OrientationType("quaternion")
)

// OrientationConfig holds the underlying type of orientation, and the value.
type OrientationConfig struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// NewOrientationConfig encodes the orientation interface to something serializable and human readable.
func NewOrientationConfig(o Orientation) (*OrientationConfig, error) {
	bytes, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}

	switch oType := o.(type) {
	case *R4AA:
		return &OrientationConfig{Type: string(AxisAnglesType), Value: json.RawMessage(bytes)}, nil
	case *OrientationVector:
		return &OrientationConfig{Type: string(OrientationVectorRadiansType), Value: json.RawMessage(bytes)}, nil
	case *OrientationVectorDegrees:
		return &OrientationConfig{Type: string(OrientationVectorDegreesType), Value: json.RawMessage(bytes)}, nil
	case *EulerAngles:
		return &OrientationConfig{Type: string(EulerAnglesType), Value: json.RawMessage(bytes)}, nil
	case *Quaternion:
		oj := quaternionJSONFromQuaternion(oType)
		bytes, err := json.Marshal(oj)
		if err != nil {
			return nil, err
		}
		return &OrientationConfig{Type: string(QuaternionType), Value: json.RawMessage(bytes)}, nil
	default:
		return nil, newOrientationTypeUnsupportedError(fmt.Sprintf("%T", oType))
	}
}

// ParseConfig will use the Type in OrientationConfig and convert into the correct struct that implements Orientation.
func (config *OrientationConfig) ParseConfig() (Orientation, error) {
	var err error
	// use the type to unmarshal the value
	switch OrientationType(config.Type) {
	case NoOrientationType:
		return NewZeroOrientation(), nil
	case OrientationVectorDegreesType:
		var o OrientationVectorDegrees
		err = json.Unmarshal(config.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, o.IsValid()
	case OrientationVectorRadiansType:
		var o OrientationVector
		err = json.Unmarshal(config.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, o.IsValid()
	case AxisAnglesType:
		var o R4AA
		err = json.Unmarshal(config.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case EulerAnglesType:
		var o EulerAngles
		err = json.Unmarshal(config.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case QuaternionType:
		var oj quaternionJSON
		err = json.Unmarshal(config.Value, &oj)
		if err != nil {
			return nil, err
		}
		return oj.toQuaternion(), nil
	default:
		return nil, newOrientationTypeUnsupportedError(config.Type)
	}
}
