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
	Type  OrientationType `json:"type"`
	Value map[string]any  `json:"value,omitempty"`
}

// NewOrientationConfig encodes the orientation interface to something serializable and human readable.
func NewOrientationConfig(o Orientation) (*OrientationConfig, error) {
	if o == nil {
		o = NewZeroOrientation()
	}

	bytes, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	value := map[string]any{}
	if err := json.Unmarshal(bytes, &value); err != nil {
		return nil, err
	}

	switch oType := o.(type) {
	case *R4AA:
		return &OrientationConfig{Type: AxisAnglesType, Value: value}, nil
	case *OrientationVector:
		return &OrientationConfig{Type: OrientationVectorRadiansType, Value: value}, nil
	case *OrientationVectorDegrees:
		return &OrientationConfig{Type: OrientationVectorDegreesType, Value: value}, nil
	case *EulerAngles:
		return &OrientationConfig{Type: EulerAnglesType, Value: value}, nil
	case *Quaternion:
		return &OrientationConfig{Type: QuaternionType, Value: value}, nil
	default:
		return nil, newOrientationTypeUnsupportedError(fmt.Sprintf("%T", oType))
	}
}

// ParseConfig will use the Type in OrientationConfig and convert into the correct struct that implements Orientation.
func (config *OrientationConfig) ParseConfig() (Orientation, error) {
	bytes, err := json.Marshal(config.Value)
	if err != nil {
		return nil, err
	}

	// use the type to unmarshal the value
	switch config.Type {
	case NoOrientationType:
		return NewZeroOrientation(), nil
	case OrientationVectorDegreesType:
		var o OrientationVectorDegrees
		err = json.Unmarshal(bytes, &o)
		if err != nil {
			return nil, err
		}
		return &o, o.IsValid()
	case OrientationVectorRadiansType:
		var o OrientationVector
		err = json.Unmarshal(bytes, &o)
		if err != nil {
			return nil, err
		}
		return &o, o.IsValid()
	case AxisAnglesType:
		var o R4AA
		err = json.Unmarshal(bytes, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case EulerAnglesType:
		var o EulerAngles
		err = json.Unmarshal(bytes, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case QuaternionType:
		var oj quaternionJSON
		err = json.Unmarshal(bytes, &oj)
		if err != nil {
			return nil, err
		}
		return oj.toQuaternion(), nil
	default:
		return nil, newOrientationTypeUnsupportedError(string(config.Type))
	}
}
