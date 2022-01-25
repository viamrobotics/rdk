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
	AlmostEqual(Volume) bool
	Transform(Pose)
	CollidesWith(Volume) (bool, error)
	DistanceFrom(Volume) (float64, error)
}

// VolumeConfig specifies the format of volumes specified through the configuration file.
type VolumeConfig struct {
	Type string `json:"type"`

	// boxes and cylinders both use this parameter for their length
	Z float64 `json:"z"`

	// parameters used for defining a box's rectangular cross section
	X float64 `json:"x"`
	Y float64 `json:"y"`

	// parameters used for defining cylinder's circular cross section
	R float64 `json:"r"`

	// parameters used for defining mesh volume, external file path
	File string `json:"file"`

	// define an offset to position the volume
	TranslationOffset TranslationConfig `json:"translation"`
	OrientationOffset OrientationConfig `json:"orientation"`
}

// ParseConfig converts a VolumeConfig into the correct VolumeCreator type, as specified in its Type field.
func (config *VolumeConfig) ParseConfig(offset Pose) (VolumeCreator, error) {
	// TODO(rb): ignore passed in offset if offset is specified in config

	switch config.Type {
	case "box":
		return NewBox(r3.Vector{config.X, config.Y, config.Z}, offset)
	case "":
		// no type specified, iterate through supported types and try to infer intent
		creator, err := NewBox(r3.Vector{config.X, config.Y, config.Z}, offset)
		if err == nil {
			return creator, nil
		}
	}
	return nil, errors.Errorf("volume type %s unsupported", config.Type)
}
