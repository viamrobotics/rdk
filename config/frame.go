package config

import (
	"encoding/json"
	"fmt"

	ref "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
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
type Frame struct {
	Parent      string                    `json:"parent"`
	Translation spatial.TranslationConfig `json:"translation"`
	Orientation spatial.Orientation       `json:"orientation"`
}

// Pose combines Translation and Orientation in a Pose.
func (f *Frame) Pose() spatial.Pose {
	return makePose(f)
}

// StaticFrame creates a new static frame from a config.
func (f *Frame) StaticFrame(name string) (ref.Frame, error) {
	pose := makePose(f)
	return ref.NewStaticFrame(name, pose)
}

// UnmarshalJSON will parse the Orientation field into a spatial.Orientation object from a json.rawMessage.
func (f *Frame) UnmarshalJSON(b []byte) error {
	temp := struct {
		Parent      string                    `json:"parent"`
		Translation spatial.TranslationConfig `json:"translation"`
		Orientation spatial.OrientationConfig `json:"orientation"`
	}{}

	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}
	orientation, err := temp.Orientation.ParseConfig()
	if err != nil {
		return fmt.Errorf("cannot unmarshal %s because of %w", string(b), err)
	}
	f.Parent = temp.Parent
	f.Translation = temp.Translation
	f.Orientation = orientation
	return nil
}

// MarshalJSON will encode the Orientation field into a spatial.OrientationConfig object instead of spatial.Orientation.
func (f *Frame) MarshalJSON() ([]byte, error) {
	temp := struct {
		Parent      string                    `json:"parent"`
		Translation spatial.TranslationConfig `json:"translation"`
		Orientation spatial.OrientationConfig `json:"orientation"`
	}{
		Parent:      f.Parent,
		Translation: f.Translation,
	}

	if f.Orientation != nil {
		orientationConfig, err := spatial.NewOrientationConfig(f.Orientation)
		if err != nil {
			return nil, err
		}
		temp.Orientation = *orientationConfig
	}

	return json.Marshal(temp)
}

// MergeFrameSystems will merge fromFS into toFS with an offset frame given by cfg. If cfg is nil, fromFS
// will be merged to the world frame of toFS with a 0 offset.
func MergeFrameSystems(toFS, fromFS ref.FrameSystem, cfg *Frame) error {
	var offsetFrame ref.Frame
	var err error
	if cfg == nil { // if nil, the parent is toFS's world, and the offset is 0
		offsetFrame = ref.NewZeroStaticFrame(fromFS.Name() + "_" + ref.World)
		err = toFS.AddFrame(offsetFrame, toFS.World())
		if err != nil {
			return err
		}
	} else { // attach the world of fromFS, with the given offset, to cfg.Parent found in toFS
		offsetFrame, err = cfg.StaticFrame(fromFS.Name() + "_" + ref.World)
		if err != nil {
			return err
		}
		err = toFS.AddFrame(offsetFrame, toFS.GetFrame(cfg.Parent))
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

// makePose creates a new pose from a config.
func makePose(cfg *Frame) spatial.Pose {
	// get the translation vector. If there is no translation/orientation attribute will default to 0
	return spatial.NewPoseFromOrientation(cfg.Translation.ParseConfig(), cfg.Orientation)
}
