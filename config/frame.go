package config

import (
	"encoding/json"
	"fmt"

	ref "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
)

// OrientationType defines what orientation representations are known
type OrientationType string

// The set of allowed representations for orientation
const (
	OrientationVectorDegrees = OrientationType("ov_degrees")
	OrientationVectorRadians = OrientationType("ov_radians")
	EulerAngles              = OrientationType("euler_angles")
	AxisAngles               = OrientationType("axis_angles")
)

// Translation is the translation between two objects in the grid system. It is always in millimeters.
type Translation struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

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
}
*/
type Frame struct {
	Parent      string              `json:"parent"`
	Translation Translation         `json:"translation"`
	Orientation spatial.Orientation `json:"orientation"`
}

// Pose combines Translation and Orientation in a Pose
func (f *Frame) Pose() spatial.Pose {
	return makePose(f)
}

// StaticFrame creates a new static frame from a config
func (f *Frame) StaticFrame(name string) (ref.Frame, error) {
	pose := makePose(f)
	return ref.NewStaticFrame(name, pose)
}

// UnmarshalJSON will parse the Orientation field into a spatial.Orientation object from a json.rawMessage
func (f *Frame) UnmarshalJSON(b []byte) error {
	temp := struct {
		Parent      string          `json:"parent"`
		Translation Translation     `json:"translation"`
		Orientation json.RawMessage `json:"orientation"`
	}{}

	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}
	orientation, err := parseOrientation(temp.Orientation)
	if err != nil {
		return err
	}
	f.Parent = temp.Parent
	f.Translation = temp.Translation
	f.Orientation = orientation
	return nil
}

// rawOrientation holds the underlying type of orientation, and the value.
type rawOrientation struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// parseOrientation will use the Type in rawOrientation to unmarshal the Value into the correct struct that implements Orientation.
func parseOrientation(j json.RawMessage) (spatial.Orientation, error) {
	// if there is no Orientation field, return a zero orientation
	if len(j) == 0 {
		return spatial.NewZeroOrientation(), nil
	}

	temp := rawOrientation{}
	err := json.Unmarshal(j, &temp)
	if err != nil {
		return nil, err
	}

	// use the type to unmarshal the value
	switch OrientationType(temp.Type) {
	case OrientationVectorDegrees:
		var o spatial.OrientationVectorDegrees
		err = json.Unmarshal(temp.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case OrientationVectorRadians:
		var o spatial.OrientationVector
		err = json.Unmarshal(temp.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case AxisAngles:
		var o spatial.R4AA
		err = json.Unmarshal(temp.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case EulerAngles:
		var o spatial.EulerAngles
		err = json.Unmarshal(temp.Value, &o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	default:
		return nil, fmt.Errorf("orientation type %s not recognized", temp.Type)
	}
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

// makePose creates a new pose from a config
func makePose(cfg *Frame) spatial.Pose {
	// get the translation vector. If there is no translation/orientation attribute will default to 0
	translation := r3.Vector{cfg.Translation.X, cfg.Translation.Y, cfg.Translation.Z}
	return spatial.NewPoseFromOrientation(translation, cfg.Orientation)
}
