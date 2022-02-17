package spatialmath

import (
	"encoding/json"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
)

// VolumeCreator provides a common way to instantiate Volumes.
type VolumeCreator interface {
	NewVolume(Pose) Volume
	json.Marshaler
}

// Volume is an entry point with which to access all types of collision geometries.
type Volume interface {
	Pose() Pose
	Vertices() []r3.Vector
	AlmostEqual(Volume) bool
	Transform(Pose)
	CollidesWith(Volume) (bool, error)
	DistanceFrom(Volume) (float64, error)
}

// VolumeConfig specifies the format of volumes specified through the configuration file.
type VolumeConfig struct {
	Type string `json:"type"`

	// parameters used for defining a box's rectangular cross section
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`

	// parameters used for defining a sphere, its radius
	R float64 `json:"r"`

	// define an offset to position the volume
	TranslationOffset TranslationConfig `json:"translation"`
	OrientationOffset OrientationConfig `json:"orientation"`
}

// NewVolumeConfig creates a config for a Volume from an offset Pose.
func NewVolumeConfig(offset Pose) (*VolumeConfig, error) {
	o := offset.Orientation()
	translationConfig := NewTranslationConfig(Compose(NewPoseFromOrientation(r3.Vector{}, OrientationInverse(o)), offset).Point())
	orientationConfig, err := NewOrientationConfig(o.AxisAngles())
	if err != nil {
		return nil, err
	}
	return &VolumeConfig{
		Type:              "point",
		TranslationOffset: *translationConfig,
		OrientationOffset: *orientationConfig,
	}, nil
}

// ParseConfig converts a VolumeConfig into the correct VolumeCreator type, as specified in its Type field.
func (config *VolumeConfig) ParseConfig() (VolumeCreator, error) {
	// determine offset to use
	orientation, err := config.OrientationOffset.ParseConfig()
	if err != nil {
		return nil, err
	}
	offset := Compose(NewPoseFromOrientation(r3.Vector{}, orientation), NewPoseFromPoint(config.TranslationOffset.ParseConfig()))

	// build VolumeCreator depending on specified type
	switch config.Type {
	case "box":
		return NewBox(r3.Vector{X: config.X, Y: config.Y, Z: config.Z}, offset)
	case "sphere":
		return NewSphere(config.R, offset)
	case "point":
		return NewPoint(offset), nil
	case "":
		// no type specified, iterate through supported types and try to infer intent
		creator, err := NewBox(r3.Vector{X: config.X, Y: config.Y, Z: config.Z}, offset)
		if err == nil {
			return creator, nil
		}
		creator, err = NewSphere(config.R, offset)
		if err == nil {
			return creator, nil
		}
		// never try to infer point volume if nothing is specified
	}
	return nil, errors.Errorf("volume type %s unsupported", config.Type)
}
