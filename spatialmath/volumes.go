package spatialmath

import "github.com/golang/geo/r3"

// VolumeCreator provides a common way to instantiate Volumes.
type VolumeCreator interface {
	NewVolume(Pose) Volume
}

// Volume is an entry point with which to access all types of collision geometries.
type Volume interface {
	Pose() Pose
	AlmostEqual(Volume) bool
	Transform(Pose)
	CollidesWith(Volume) (bool, error)
	DistanceFrom(Volume) (float64, error)
}

type VolumeConfig struct {
	Type string `json:type`

	// boxes and cylinders both use this parameter for their length
	Z float64 `json:z`

	// parameters used for defining a box's rectangular cross section
	X float64 `json:x`
	Y float64 `json:y`

	// parameters used for defining cylinder's circular cross section
	R float64 `json:r`

	// parameters used for defining mesh volume, external file path
	File string `json:file`
}

func (config *VolumeConfig) ParseConfig(offset Pose) VolumeCreator {
	creator, err := NewBox(r3.Vector{config.X, config.Y, config.Z}, offset)
	if err == nil {
		return creator
	}
	return nil
}
