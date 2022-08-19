package protoutils

import (
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	
	"go.viam.com/rdk/spatialmath"
)

func goToProto(v interface{}) (*structpb.Value, error) {
	switch x := v.(type) {
	case spatialmath.AngularVelocity:
		v = map[string]interface{}{
			"x" : x.X,
			"y" : x.Y,
			"z" : x.Z,
			"_type" : "angular_velocity",
		}
	case r3.Vector: 
		v = map[string]interface{}{
			"x" : x.X,
			"y" : x.Y,
			"z" : x.Z,
			"_type" : "vector3",
		}
	case *spatialmath.EulerAngles: 
		v = map[string]interface{}{
			"roll" : x.Roll,
			"pitch" : x.Pitch,
			"yaw" : x.Yaw,
			"_type" : "euler",
		}
	case *spatialmath.Quaternion: 
		v = map[string]interface{}{
			"r" : x.Real,
			"i" : x.Imag,
			"j" : x.Jmag,
			"k" : x.Kmag,
			"_type" : "quat",
		}

	case *geo.Point: 
		v = map[string]interface{}{
			"lat" : x.Lat(),
			"lng" : x.Lng(),
			"_type" : "geopoint",
		}

	}

	return structpb.NewValue(v)
}

func SensorGoToProto(readings map[string]interface{}) (map[string]*structpb.Value, error) {
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

func SensorProtoToGo(readings map[string]*structpb.Value) (map[string]interface{}, error) {
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
		case "angular_velocity":
			return spatialmath.AngularVelocity{
				X : x["x"].(float64),
				Y : x["y"].(float64),
				Z : x["z"].(float64),
			}
		case "vector3":
			return r3.Vector{
				X : x["x"].(float64),
				Y : x["y"].(float64),
				Z : x["z"].(float64),
			}
		case "euler":
			return &spatialmath.EulerAngles{
				Roll : x["roll"].(float64),
				Pitch : x["pitch"].(float64),
				Yaw : x["yaw"].(float64),
			}
		case "quat":
			return &spatialmath.Quaternion{
				x["r"].(float64),
				x["i"].(float64),
				x["j"].(float64),
				x["k"].(float64),
			}

		case "geopoint":
			return geo.NewPoint(
				x["lat"].(float64),
				x["lng"].(float64),
			)
		}
			
	}
	return v			
}
