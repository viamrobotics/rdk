package protoutils

import (
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	
	"go.viam.com/rdk/spatialmath"
)

const type_angular_velocity = "angular_velocity"
const type_vector3 = "vector3"
const type_euler = "euler"
const type_quat = "quat"
const type_geopoint = "geopoint"

func goToProto(v interface{}) (*structpb.Value, error) {
	switch x := v.(type) {
	case spatialmath.AngularVelocity:
		v = map[string]interface{}{
			"x" : x.X,
			"y" : x.Y,
			"z" : x.Z,
			"_type" : type_angular_velocity,
		}
	case r3.Vector: 
		v = map[string]interface{}{
			"x" : x.X,
			"y" : x.Y,
			"z" : x.Z,
			"_type" : type_vector3,
		}
	case *spatialmath.EulerAngles: 
		v = map[string]interface{}{
			"roll" : x.Roll,
			"pitch" : x.Pitch,
			"yaw" : x.Yaw,
			"_type" : type_euler,
		}
	case *spatialmath.Quaternion: 
		v = map[string]interface{}{
			"r" : x.Real,
			"i" : x.Imag,
			"j" : x.Jmag,
			"k" : x.Kmag,
			"_type" : type_quat,
		}

	case *geo.Point: 
		v = map[string]interface{}{
			"lat" : x.Lat(),
			"lng" : x.Lng(),
			"_type" : type_geopoint,
		}

	}

	v, err := toInterface(v)
	if err != nil {
		return nil, err
	}
	return structpb.NewValue(v)
}

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

func ReadingProtoToGo(readings map[string]*structpb.Value) (map[string]interface{}, error) {
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
		case type_angular_velocity:
			return spatialmath.AngularVelocity{
				X : x["x"].(float64),
				Y : x["y"].(float64),
				Z : x["z"].(float64),
			}
		case type_vector3:
			return r3.Vector{
				X : x["x"].(float64),
				Y : x["y"].(float64),
				Z : x["z"].(float64),
			}
		case type_euler:
			return &spatialmath.EulerAngles{
				Roll : x["roll"].(float64),
				Pitch : x["pitch"].(float64),
				Yaw : x["yaw"].(float64),
			}
		case type_quat:
			return &spatialmath.Quaternion{
				x["r"].(float64),
				x["i"].(float64),
				x["j"].(float64),
				x["k"].(float64),
			}

		case type_geopoint:
			return geo.NewPoint(
				x["lat"].(float64),
				x["lng"].(float64),
			)
		}
			
	}
	return v			
}
