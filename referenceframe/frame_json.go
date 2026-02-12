package referenceframe

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/golang/geo/r3"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// The following are joint types we treat as constants.
const (
	FixedJoint      = "fixed"
	ContinuousJoint = "continuous"
	PrismaticJoint  = "prismatic"
	RevoluteJoint   = "revolute"
)

// LinkConfig is a StaticFrame that also has a specified parent.
type LinkConfig struct {
	ID          string                     `json:"id"`
	Translation r3.Vector                  `json:"translation"`
	Orientation *spatial.OrientationConfig `json:"orientation"`
	Geometry    *spatial.GeometryConfig    `json:"geometry,omitempty"`
	Parent      string                     `json:"parent,omitempty"`
}

// JointConfig is a frame with nonzero DOF. Supports rotational or translational.
type JointConfig struct {
	ID       string                  `json:"id"`
	Type     string                  `json:"type"`
	Parent   string                  `json:"parent"`
	Axis     spatial.AxisConfig      `json:"axis"`
	Max      float64                 `json:"max"`                // in mm or degs
	Min      float64                 `json:"min"`                // in mm or degs
	Geometry *spatial.GeometryConfig `json:"geometry,omitempty"` // only valid for prismatic/translational joints
}

// DHParamConfig is a revolute and static frame combined in a set of Denavit Hartenberg parameters.
type DHParamConfig struct {
	ID       string                  `json:"id"`
	Parent   string                  `json:"parent"`
	A        float64                 `json:"a"`
	D        float64                 `json:"d"`
	Alpha    float64                 `json:"alpha"`
	Max      float64                 `json:"max"` // in mm or degs
	Min      float64                 `json:"min"` // in mm or degs
	Geometry *spatial.GeometryConfig `json:"geometry,omitempty"`
}

// NewLinkConfig constructs a config from a Frame.
// NOTE: this currently only works with Frames of len(DoF)==0 and will error otherwise
// NOTE: this will not work if more than one Geometry is returned by the Geometries function.
func NewLinkConfig(frame Frame) (*LinkConfig, error) {
	if len(frame.DoF()) > 0 {
		return nil, fmt.Errorf("cannot create link config for Frame with %d DoF", len(frame.DoF()))
	}

	pose, err := frame.Transform([]Input{})
	if err != nil {
		return nil, err
	}
	orient, err := spatial.NewOrientationConfig(pose.Orientation())
	if err != nil {
		return nil, err
	}

	var geom *spatial.GeometryConfig
	gif, err := frame.Geometries([]Input{})
	if err != nil {
		return nil, err
	}
	geometries := gif.Geometries()
	if len(geometries) > 0 {
		// TODO: when we support having multiple geometries on a pb.Frame in the API this will need to go away
		if len(geometries) > 1 {
			return nil, fmt.Errorf("cannot create link config for Frame with %d geometries", len(geometries))
		}
		geom, err = spatial.NewGeometryConfig(geometries[0])
		if err != nil {
			return nil, err
		}
	}

	return &LinkConfig{
		ID:          frame.Name(),
		Translation: pose.Point(),
		Orientation: orient,
		Geometry:    geom,
	}, nil
}

// ParseConfig converts a LinkConfig into a staticFrame.
func (cfg *LinkConfig) ParseConfig() (*LinkInFrame, error) {
	pose, err := cfg.Pose()
	if err != nil {
		return nil, err
	}
	var geom spatial.Geometry
	if cfg.Geometry != nil {
		geom, err = cfg.Geometry.ParseConfig()
		if err != nil {
			return nil, err
		}
		if geom.Label() == "" {
			geom.SetLabel(cfg.ID)
		}
	}
	return NewLinkInFrame(cfg.Parent, pose, cfg.ID, geom), nil
}

// Pose will parse out the Pose of a LinkConfig and return it if it is valid.
func (cfg *LinkConfig) Pose() (spatial.Pose, error) {
	pt := cfg.Translation
	if cfg.Orientation != nil {
		orient, err := cfg.Orientation.ParseConfig()
		if err != nil {
			return nil, err
		}
		return spatial.NewPose(pt, orient), nil
	}
	return spatial.NewPoseFromPoint(pt), nil
}

// ToFrame converts a JointConfig into a joint frame.
func (cfg *JointConfig) ToFrame() (Frame, error) {
	switch cfg.Type {
	case RevoluteJoint:
		return NewRotationalFrame(cfg.ID, cfg.Axis.ParseConfig(),
			Limit{Min: utils.DegToRad(cfg.Min), Max: utils.DegToRad(cfg.Max)})
	case PrismaticJoint:
		return NewTranslationalFrame(cfg.ID, r3.Vector(cfg.Axis),
			Limit{Min: cfg.Min, Max: cfg.Max})
	default:
		return nil, NewUnsupportedJointTypeError(cfg.Type)
	}
}

// ToDHFrames converts a DHParamConfig into a joint frame and a link frame.
func (cfg *DHParamConfig) ToDHFrames() (Frame, Frame, error) {
	jointID := cfg.ID + "_j"
	rFrame, err := NewRotationalFrame(jointID, spatial.R4AA{RX: 0, RY: 0, RZ: 1},
		Limit{Min: utils.DegToRad(cfg.Min), Max: utils.DegToRad(cfg.Max)})
	if err != nil {
		return nil, nil, err
	}

	// Link part of DH param
	linkID := cfg.ID
	pose := spatial.NewPoseFromDH(cfg.A, cfg.D, utils.DegToRad(cfg.Alpha))
	var lFrame Frame
	if cfg.Geometry != nil {
		geometryCreator, err := cfg.Geometry.ParseConfig()
		if err != nil {
			return nil, nil, err
		}
		lFrame, err = NewStaticFrameWithGeometry(linkID, pose, geometryCreator)
		if err != nil {
			return nil, nil, err
		}
	} else {
		lFrame, err = NewStaticFrame(cfg.ID, pose)
		if err != nil {
			return nil, nil, err
		}
	}
	return rFrame, lFrame, nil
}

// frameToJSON marshals an implementer of the Frame interface into JSON.
func frameToJSON(frame Frame) ([]byte, error) {
	type typedFrame struct {
		FrameType string `json:"frame_type"`
		Frame     Frame  `json:"frame"`
	}
	for name, f := range registeredFrameImplementers {
		if reflect.ValueOf(frame).Type().Elem() == f {
			return json.Marshal(&typedFrame{
				FrameType: name,
				Frame:     frame,
			})
		}
	}
	return []byte{}, fmt.Errorf("Frame of type %T is not a registered Frame implementation", frame)
}

// jsonToFrame converts raw JSON into a Frame by using a key called "frame_type"
// to determine the explicit struct type to which the frame data (found under the key
// "frame") should be marshalled.
func jsonToFrame(data json.RawMessage) (Frame, error) {
	var sF map[string]json.RawMessage
	if err := json.Unmarshal(data, &sF); err != nil {
		return nil, err
	}
	var frameType string
	if err := json.Unmarshal(sF["frame_type"], &frameType); err != nil {
		return nil, err
	}
	if _, ok := sF["frame"]; !ok {
		return nil, fmt.Errorf("no frame data found for frame, type was %s", frameType)
	}

	implementer, ok := registeredFrameImplementers[frameType]
	if !ok {
		return nil, fmt.Errorf("%s is not a registered Frame implementation", frameType)
	}
	frameZeroStruct := reflect.New(implementer).Elem()
	frame := frameZeroStruct.Addr().Interface()
	frameI, err := utils.AssertType[Frame](frame)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(sF["frame"], frame); err != nil {
		return nil, err
	}

	return frameI, nil
}
