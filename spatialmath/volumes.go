package spatialmath

// VolumeCreator provides a common way to instantiate Volumes
type VolumeCreator interface {
	NewVolume(Pose) Volume
}

// Volume is an entry point with which to access all types of collision geometries
type Volume interface {
	CollidesWith(Volume) (bool, error)
}
