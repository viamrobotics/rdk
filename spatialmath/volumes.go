package spatialmath

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
