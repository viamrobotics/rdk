package referenceframe

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang/geo/r3"
	//~ "github.com/mitchellh/mapstructure"

	spatial "go.viam.com/rdk/spatialmath"
	//~ "go.viam.com/rdk/utils"
)

/*
Frame contains the information of the pose and parent of the frame that will be created.
When using the pose as a transformation, the rotation is applied first, and then the translation.
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
	}.
*/
type StaticFrame struct {
	Parent      string                  `json:"parent"`
	Translation r3.Vector               `json:"translation"`
	Orientation spatial.Orientation     `json:"orientation"`
	Geometry    spatial.GeometryCreator `json:"geometry"`
}

type frameConfig struct {
	Parent      string                    `json:"parent"`
	Translation spatial.TranslationConfig `json:"translation"`
	Orientation spatial.OrientationConfig `json:"orientation"`
	Geometry    spatial.GeometryConfig    `json:"geometry"`
}

// Pose combines Translation and Orientation in a Pose.
func (f *StaticFrame) Pose() spatial.Pose {
	// get the translation vector. If there is no translation/orientation attribute will default to 0
	return spatial.NewPoseFromOrientation(f.Translation, f.Orientation)
}

// StaticFrame creates a new static frame from a config.
func (f *StaticFrame) StaticFrame(name string) (Frame, error) {
	return NewStaticFrameWithGeometry(name, f.Pose(), f.Geometry)
}

// UnmarshalJSON will parse unmarshall json corresponding to a frame config.
func (f *StaticFrame) UnmarshalJSON(b []byte) error {
	temp := frameConfig{}
	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}

	f.Parent = temp.Parent
	f.Translation = temp.Translation.ParseConfig()
	f.Orientation, err = temp.Orientation.ParseConfig()
	if err != nil {
		return fmt.Errorf("cannot unmarshal %s because of %w", string(b), err)
	}
	f.Geometry, err = temp.Geometry.ParseConfig()
	if err != nil && !strings.Contains(err.Error(), spatial.ErrGeometryTypeUnsupported.Error()) {
		return err
	}
	return nil
}

// MarshalJSON will encode the Orientation field into a spatial.OrientationConfig object instead of spatial.Orientation.
func (f *StaticFrame) MarshalJSON() ([]byte, error) {
	temp := frameConfig{
		Parent:      f.Parent,
		Translation: *spatial.NewTranslationConfig(f.Translation),
	}

	if f.Orientation != nil {
		orientationConfig, err := spatial.NewOrientationConfig(f.Orientation)
		if err != nil {
			return nil, err
		}
		temp.Orientation = *orientationConfig
	}

	if f.Geometry != nil {
		geometryConfig, err := spatial.NewGeometryConfig(f.Geometry)
		if err != nil {
			return nil, err
		}
		temp.Geometry = *geometryConfig
	}
	return json.Marshal(temp)
}

// MergeFrameSystems will merge fromFS into toFS with an offset frame given by cfg. If cfg is nil, fromFS
// will be merged to the world frame of toFS with a 0 offset.
func MergeFrameSystems(toFS, fromFS FrameSystem, cfg *StaticFrame) error {
	var offsetFrame Frame
	var err error
	if cfg == nil { // if nil, the parent is toFS's world, and the offset is 0
		offsetFrame = NewZeroStaticFrame(fromFS.Name() + "_" + World)
		err = toFS.AddFrame(offsetFrame, toFS.World())
		if err != nil {
			return err
		}
	} else { // attach the world of fromFS, with the given offset, to cfg.Parent found in toFS
		offsetFrame, err = cfg.StaticFrame(fromFS.Name() + "_" + World)
		if err != nil {
			return err
		}
		err = toFS.AddFrame(offsetFrame, toFS.Frame(cfg.Parent))
		if err != nil {
			return err
		}
	}
	err = toFS.MergeFrameSystem(fromFS, offsetFrame)
	if err != nil {
		return err
	}
	return nil
}


//~ // FrameMapConfig represents the format for configuring a Frame object.
//~ type FrameMapConfig map[string]interface{}

//~ // UnmarshalFrameJSON deserialized json into a reference referenceframe.
//~ func UnmarshalFrameJSON(data []byte) (Frame, error) {
	//~ config := FrameMapConfig{}
	//~ err := json.Unmarshal(data, &config)
	//~ if err != nil {
		//~ return nil, err
	//~ }

	//~ return config.ParseConfig()
//~ }

//~ // ParseConfig converts a FrameMapConfig to a Frame object.
//~ func (config FrameMapConfig) ParseConfig() (Frame, error) {
	//~ name, ok := config["name"].(string)
	//~ if !ok {
		//~ return nil, utils.NewUnexpectedTypeError(name, config["name"])
	//~ }

	//~ switch config["type"] {
	//~ case "static":
		//~ pose, ok := config["transform"].(map[string]interface{})
		//~ if !ok {
			//~ return nil, utils.NewUnexpectedTypeError(pose, config["transform"])
		//~ }
		//~ transform, err := decodePose(pose)
		//~ if err != nil {
			//~ return nil, fmt.Errorf("error decoding transform (%v) %w", config["transform"], err)
		//~ }
		//~ return NewStaticFrame(name, transform)
	//~ case "translational":
		//~ var transAxis r3.Vector
		//~ err := mapstructure.Decode(config["transAxis"], &transAxis)
		//~ if err != nil {
			//~ return nil, err
		//~ }
		//~ var limit []Limit
		//~ err = mapstructure.Decode(config["limit"], &limit)
		//~ if err != nil {
			//~ return nil, err
		//~ }
		//~ return NewTranslationalFrame(name, transAxis, limit[0])
	//~ case "rotational":
		//~ rotAxis, ok := config["rotAxis"].(map[string]interface{})
		//~ if !ok {
			//~ return nil, utils.NewUnexpectedTypeError(rotAxis, config["rotAxis"])
		//~ }
		//~ var axis spatial.R4AA
		//~ axis.RX, ok = rotAxis["X"].(float64)
		//~ if !ok {
			//~ return nil, utils.NewUnexpectedTypeError(axis.RX, rotAxis["X"])
		//~ }
		//~ axis.RY, ok = rotAxis["Y"].(float64)
		//~ if !ok {
			//~ return nil, utils.NewUnexpectedTypeError(axis.RY, rotAxis["Y"])
		//~ }
		//~ axis.RZ, ok = rotAxis["Z"].(float64)
		//~ if !ok {
			//~ return nil, utils.NewUnexpectedTypeError(axis.RZ, rotAxis["Z"])
		//~ }
		//~ var limit []Limit
		//~ err := mapstructure.Decode(config["limit"], &limit)
		//~ if err != nil {
			//~ return nil, err
		//~ }
		//~ return NewRotationalFrame(name, axis, limit[0])
	//~ default:
		//~ return nil, fmt.Errorf("no frame type: [%v]", config["type"])
	//~ }
//~ }

//~ func decodePose(config FrameMapConfig) (spatial.Pose, error) {
	//~ var point r3.Vector

	//~ err := mapstructure.Decode(config["point"], &point)
	//~ if err != nil {
		//~ return nil, err
	//~ }

	//~ orientationMap, ok := config["orientation"].(map[string]interface{})
	//~ if !ok {
		//~ return nil, utils.NewUnexpectedTypeError(orientationMap, config["orientation"])
	//~ }
	//~ oType, ok := orientationMap["type"].(string)
	//~ if !ok {
		//~ return nil, utils.NewUnexpectedTypeError(oType, orientationMap["type"])
	//~ }
	//~ oValue, ok := orientationMap["value"].(map[string]interface{})
	//~ if !ok {
		//~ return nil, utils.NewUnexpectedTypeError(oValue, orientationMap["value"])
	//~ }
	//~ jsonValue, err := json.Marshal(oValue)
	//~ if err != nil {
		//~ return nil, err
	//~ }

	//~ orientation, err := (&spatial.OrientationConfig{spatial.OrientationType(oType), jsonValue}).ParseConfig()
	//~ if err != nil {
		//~ return nil, err
	//~ }
	//~ return spatial.NewPoseFromOrientation(point, orientation), nil
//~ }
