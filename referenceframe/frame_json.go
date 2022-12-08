package referenceframe

import (
	"github.com/golang/geo/r3"
	"go.viam.com/rdk/utils"
	spatial "go.viam.com/rdk/spatialmath"
)

// LinkConfig is a StaticFrame that also has a specified parent
type LinkConfig struct {
	ID          string                    `json:"id"`
	Translation spatial.TranslationConfig `json:"translation"`
	Orientation *spatial.OrientationConfig `json:"orientation"`
	Geometry    *spatial.GeometryConfig    `json:"geometry,omitempty"`
	Parent      string                    `json:"parent,omitempty"`
}

type JointConfig struct {
	ID     string             `json:"id"`
	Type   string             `json:"type"`
	Parent string             `json:"parent"`
	Axis   spatial.AxisConfig `json:"axis"`
	Max    float64            `json:"max"` // in mm or degs
	Min    float64            `json:"min"` // in mm or degs
	Geometry    *spatial.GeometryConfig    `json:"geometry,omitempty"` // only valid for prismatic/translational joints
}

type DHParamConfig struct {
	ID       string                 `json:"id"`
	Parent   string                 `json:"parent"`
	A        float64                `json:"a"`
	D        float64                `json:"d"`
	Alpha    float64                `json:"alpha"`
	Max      float64                `json:"max"` // in mm or degs
	Min      float64                `json:"min"` // in mm or degs
	Geometry *spatial.GeometryConfig `json:"geometry,omitempty"`
}

// NewLinkConfig constructs a config from a Frame.
func NewLinkConfig(frame staticFrame) (*LinkConfig, error) {
	var geom *spatial.GeometryConfig
	orient, err := spatial.NewOrientationConfig(frame.transform.Orientation())
	if err != nil {
		return nil, err
	}
	if frame.geometryCreator != nil {
		geom, err = spatial.NewGeometryConfig(frame.geometryCreator)
		if err != nil {
			return nil, err
		}
	}
	return &LinkConfig{
		ID: frame.name,
		Translation: *spatial.NewTranslationConfig(frame.transform.Point()),
		Orientation: orient,
		Geometry: geom,
	}, nil
}

// ParseConfig converts a LinkConfig into a staticFrame
func (cfg *LinkConfig) ParseConfig() (Frame, error) {
	return cfg.ToStaticFrame(cfg.ID)
}

// ToStaticFrame converts a LinkConfig into a staticFrame with a new name
func (cfg *LinkConfig) ToStaticFrame(name string) (Frame, error) {
	if name == "" {
		name = cfg.ID
	}
	pose, err := cfg.Pose()
	if err != nil {
		return nil, err
	}
	
	if cfg.Geometry != nil {
		geom, err := cfg.Geometry.ParseConfig()
		if err != nil {
			return nil, err
		}
		NewStaticFrameWithGeometry(name, pose, geom)
	}
	
	return NewStaticFrame(name, pose)
}

func (cfg *LinkConfig) Pose() (spatial.Pose, error) {
	pt := cfg.Translation.ParseConfig()
	if cfg.Orientation != nil {
		orient, err := cfg.Orientation.ParseConfig()
		if err != nil {
			return nil, err
		}
		return spatial.NewPoseFromOrientation(pt, orient), nil
	}
	return spatial.NewPoseFromPoint(pt), nil
}

// ToFrame converts a JointConfig into a joint frame
func (cfg *JointConfig) ToFrame() (Frame, error) {
	switch cfg.Type {
	case "revolute":
		return NewRotationalFrame(cfg.ID, cfg.Axis.ParseConfig(),
			Limit{Min: utils.DegToRad(cfg.Min), Max: utils.DegToRad(cfg.Max)})
	case "prismatic":
		return NewTranslationalFrame(cfg.ID, r3.Vector(cfg.Axis),
			Limit{Min: cfg.Min, Max: cfg.Max})
	default:
		return nil, NewUnsupportedJointTypeError(cfg.Type)
	}
}
