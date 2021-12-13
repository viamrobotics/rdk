package referenceframe

import spatial "go.viam.com/core/spatialmath"

type VolumeCreator interface {
	NewVolume(spatial.Pose) (Volume, error)
}

type Volume interface {
	CollidesWith(Volume) (bool, error)
}
