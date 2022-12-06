package referenceframe

import (
	//~ "encoding/json"
	"reflect"
	spatial "go.viam.com/rdk/spatialmath"
)

type StaticFrameCfg struct {
	ID          string                    `json:"id"`
	Translation spatial.TranslationConfig `json:"translation"`
	Orientation *spatial.OrientationConfig `json:"orientation"`
	Geometry    *spatial.GeometryConfig    `json:"geometry,omitempty"`
}

// LinkCfg is a StaticFrameCfg that also has a specified parent
type LinkCfg struct {
	*StaticFrameCfg
	Parent      string                    `json:"parent"`
}

type JointCfg struct {
	ID     string             `json:"id"`
	Type   string             `json:"type"`
	Parent string             `json:"parent"`
	Axis   spatial.AxisConfig `json:"axis"`
	Max    float64            `json:"max"` // in mm or degs
	Min    float64            `json:"min"` // in mm or degs
	Geometry    *spatial.GeometryConfig    `json:"geometry,omitempty"` // only valid for prismatic/translational joints
}

type DHParamCfg struct {
	ID       string                 `json:"id"`
	Parent   string                 `json:"parent"`
	A        float64                `json:"a"`
	D        float64                `json:"d"`
	Alpha    float64                `json:"alpha"`
	Max      float64                `json:"max"` // in mm or degs
	Min      float64                `json:"min"` // in mm or degs
	Geometry *spatial.GeometryConfig `json:"geometry,omitempty"`
}

// NewStaticFrameCfg constructs a config from a Frame.
func NewStaticFrameCfg(frame staticFrame) (*StaticFrameCfg, error) {
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
	return &StaticFrameCfg{
		ID: frame.name,
		Translation: *spatial.NewTranslationConfig(frame.transform.Point()),
		Orientation: orient,
		Geometry: geom,
	}, nil
}

// ParseConfig converts a StaticFrameCfg into a staticFrame
func (cfg *StaticFrameCfg) ParseConfig() (Frame, error) {
	return cfg.ToStaticFrame(cfg.ID)
}

// ToStaticFrame converts a StaticFrameCfg into a staticFrame with a new name
func (cfg *StaticFrameCfg) ToStaticFrame(name string) (Frame, error) {
	pose, err := cfg.Pose()
	
	var geom spatial.GeometryCreator
	if !reflect.DeepEqual(cfg.Geometry, spatial.GeometryConfig{}) {
		geom, err = cfg.Geometry.ParseConfig()
		if err != nil {
			return nil, err
		}
	}
	
	return NewStaticFrameWithGeometry(name, pose, geom)
}

func (cfg *StaticFrameCfg) Pose() (spatial.Pose, error) {
	pt := cfg.Translation.ParseConfig()
	orient, err := cfg.Orientation.ParseConfig()
	if err != nil {
		return nil, err
	}
	return spatial.NewPoseFromOrientation(pt, orient), nil
}
