// register registers all relevant cameras and also subtype specific functions
package register

import (
	"go.viam.com/core/component/camera"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"

	_ "go.viam.com/core/component/camera/fake"
	_ "go.viam.com/core/component/camera/gopro"
	_ "go.viam.com/core/component/camera/imagesource"
	_ "go.viam.com/core/component/camera/velodyne"
)

func init() {
	registry.RegisterResourceSubtype(camera.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return camera.WrapWithReconfigurable(r)
		},
	})
}
