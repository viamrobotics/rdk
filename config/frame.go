package config

import (
	"encoding/json"

	"github.com/golang/geo/r3"

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
	Parent      string              `json:"parent"`
	Translation spatial.Translation `json:"translation"`
	Orientation spatial.Orientation `json:"orientation"`
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
		Parent      string                 `json:"parent"`
		Translation spatial.Translation    `json:"translation"`
		Orientation spatial.RawOrientation `json:"orientation"`
	}{}

	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}
	orientation, err := spatial.ParseOrientation(temp.Orientation)
	if err != nil {
		return err
	}
	f.Parent = temp.Parent
	f.Translation = temp.Translation
	f.Orientation = orientation
	return nil
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
	translation := r3.Vector{cfg.Translation.X, cfg.Translation.Y, cfg.Translation.Z}
	return spatial.NewPoseFromOrientation(translation, cfg.Orientation)
}
