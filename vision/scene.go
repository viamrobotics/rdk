package vision

import (
	"go.viam.com/rdk/pointcloud"
)

// A Scene is the 3D image, as well as a list of the objects in the image
type Scene interface {
	Objects() []*pointcloud.WithMetadata
}

type basicScene struct {
	objects []*pointcloud.WithMetadata
}

func NewScene(objects []*pointcloud.WithMetadata) (Scene, error) {
	return &basicScene{objects}, nil
}

func (b *basicScene) Objects() []*pointcloud.WithMetadata {
	return b.objects
}
