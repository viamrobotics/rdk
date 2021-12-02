// Package register registers all relevant cameras and also subtype specific functions
package register

import (
	"go.viam.com/core/component/camera"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"

	_ "go.viam.com/core/component/camera/fake"        // for camera
	_ "go.viam.com/core/component/camera/gopro"       // for camera
	_ "go.viam.com/core/component/camera/imagesource" // for camera
	_ "go.viam.com/core/component/camera/velodyne"    // for camera
)

func init() {
	registry.RegisterResourceSubtype(camera.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return camera.WrapWithReconfigurable(r)
		},
	})
}
