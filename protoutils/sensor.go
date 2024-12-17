package protoutils

import (
	"errors"
	"strings"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/spatialmath"
)

const (
	typeAngularVelocity          = "angular_velocity"
	typeVector3                  = "vector3"
	typeEuler                    = "euler"
	typeQuat                     = "quat"
	typeGeopoint                 = "geopoint"
	typeOrientationVector        = "orientation_vector_radians"
	typeOrientationVectorDegrees = "orientation_vector_degrees"
	typeAxisAngle                = "r4aa"
)

func goToProto(v interface{}) (*structpb.Value, error) {
	switch x := v.(type) {
	case spatialmath.AngularVelocity:
		v = map[string]interface{}{
			"x":     x.X,
			"y":     x.Y,
			"z":     x.Z,
			"_type": typeAngularVelocity,
		}
	case r3.Vector:
		v = map[string]interface{}{
			"x":     x.X,
			"y":     x.Y,
			"z":     x.Z,
			"_type": typeVector3,
		}
	case *spatialmath.EulerAngles:
		v = map[string]interface{}{
			"roll":  x.Roll,
			"pitch": x.Pitch,
			"yaw":   x.Yaw,
			"_type": typeEuler,
		}
	case *spatialmath.Quaternion:
		v = map[string]interface{}{
			"r":     x.Real,
			"i":     x.Imag,
			"j":     x.Jmag,
			"k":     x.Kmag,
			"_type": typeQuat,
		}
	case *spatialmath.OrientationVector:
		v = map[string]interface{}{
			"theta": x.Theta,
			"ox":    x.OX,
			"oy":    x.OY,
			"oz":    x.OZ,
			"_type": typeOrientationVector,
		}
	case *spatialmath.OrientationVectorDegrees:
		v = map[string]interface{}{
			"theta": x.Theta,
			"ox":    x.OX,
			"oy":    x.OY,
			"oz":    x.OZ,
			"_type": typeOrientationVectorDegrees,
		}
	case *spatialmath.R4AA:
		v = map[string]interface{}{
			"theta": x.Theta,
			"rx":    x.RX,
			"ry":    x.RY,
			"rz":    x.RZ,
			"_type": typeAxisAngle,
		}
	case spatialmath.Orientation:
		deg := x.OrientationVectorDegrees()
		v = map[string]interface{}{
			"theta": deg.Theta,
			"ox":    deg.OX,
			"oy":    deg.OY,
			"oz":    deg.OZ,
			"_type": typeOrientationVectorDegrees,
		}
	case *geo.Point:
		v = map[string]interface{}{
			"lat":   x.Lat(),
			"lng":   x.Lng(),
			"_type": typeGeopoint,
		}
	}

	vv, err := structpb.NewValue(v)
	// The structpb package sometimes inserts non-breaking spaces into their error messages (see
	// https://github.com/protocolbuffers/protobuf-go/blob/master/internal/errors/errors.go#L30).
	// However, we put error messages into the headers/trailers of GRPC responses, and client-side
	// parsing of those requires that they must be entirely ASCII. So, we need to ensure that any
	// errors from here do not contain non-breaking spaces.
	if err != nil {
		ascii := strings.ReplaceAll(
			err.Error(), " " /* non-breaking space */, " " /* normal space */)
		err = errors.New(ascii)
	}
	return vv, err
}

// ReadingGoToProto converts go readings to proto readings.
func ReadingGoToProto(readings map[string]interface{}) (map[string]*structpb.Value, error) {
	m := map[string]*structpb.Value{}

	for k, v := range readings {
		vv, err := goToProto(v)
		if err != nil {
			return nil, err
		}
		m[k] = vv
	}

	return m, nil
}

// ReadingProtoToGo converts proto readings to go readings.
func ReadingProtoToGo(readings map[string]*structpb.Value) (map[string]interface{}, error) {
	if readings == nil {
		return nil, nil
	}
	m := map[string]interface{}{}
	for k, v := range readings {
		m[k] = cleanSensorType(v.AsInterface())
	}
	return m, nil
}

func cleanSensorType(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		switch x["_type"] {
		case typeAngularVelocity:
			return spatialmath.AngularVelocity{
				X: x["x"].(float64),
				Y: x["y"].(float64),
				Z: x["z"].(float64),
			}
		case typeVector3:
			return r3.Vector{
				X: x["x"].(float64),
				Y: x["y"].(float64),
				Z: x["z"].(float64),
			}
		case typeEuler:
			return &spatialmath.EulerAngles{
				Roll:  x["roll"].(float64),
				Pitch: x["pitch"].(float64),
				Yaw:   x["yaw"].(float64),
			}
		case typeQuat:
			return &spatialmath.Quaternion{
				x["r"].(float64),
				x["i"].(float64),
				x["j"].(float64),
				x["k"].(float64),
			}
		case typeOrientationVector:
			return &spatialmath.OrientationVector{
				Theta: x["theta"].(float64),
				OX:    x["ox"].(float64),
				OY:    x["oy"].(float64),
				OZ:    x["oz"].(float64),
			}
		case typeOrientationVectorDegrees:
			return &spatialmath.OrientationVectorDegrees{
				Theta: x["theta"].(float64),
				OX:    x["ox"].(float64),
				OY:    x["oy"].(float64),
				OZ:    x["oz"].(float64),
			}
		case typeAxisAngle:
			return &spatialmath.R4AA{
				Theta: x["theta"].(float64),
				RX:    x["rx"].(float64),
				RY:    x["ry"].(float64),
				RZ:    x["rz"].(float64),
			}
		case typeGeopoint:
			return geo.NewPoint(
				x["lat"].(float64),
				x["lng"].(float64),
			)
		default:
			return v
		}
	default:
		return v
	}
}
