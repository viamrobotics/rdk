// Package fake implements a fake base.
package fake

import (
	"context"
	"errors"
	"image"
	"os"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

var model = resource.NewDefaultModel("fake")

func init() {
	registry.RegisterService(
		slam.Subtype,
		model,
		registry.Service{
			Constructor: func(
				ctx context.Context,
				_ registry.Dependencies,
				config config.Service,
				logger golog.Logger,
			) (interface{}, error) {
				return &FakeSLAM{Name: config.Name}, nil
			},
		},
	)
}

var _ = slam.Service(&FakeSLAM{})

// FakeSLAM is a fake slam that returns generic data.
type FakeSLAM struct {
	generic.Echo
	Name string
}

// GetMap does nothing.
func (slamSvc *FakeSLAM) GetMap(ctx context.Context, name, mimeType string, cp *referenceframe.PoseInFrame,
	include bool, extra map[string]interface{},
) (string, image.Image, *vision.Object, error) {
	var err error
	var img image.Image
	var vObj *vision.Object

	switch mimeType {
	case rdkutils.MimeTypePCD:
		pcdFile, err := os.Open(artifact.MustPath("slam/fakePCDMap.pcd"))
		if err != nil {
			return "", nil, nil, err
		}
		pc, err := pointcloud.ReadPCDToBasicOctree(pcdFile)
		if err != nil {
			return "", nil, nil, err
		}
		vObj, err = vision.NewObject(pc)
		if err != nil {
			return "", nil, nil, err
		}
	case rdkutils.MimeTypeJPEG:
		img, err = rimage.NewImageFromFile(artifact.MustPath("slam/fakeImageMap.jpg"))
	default:
		return "", nil, nil, errors.New("received invalid mimeType for GetMap call")
	}
	if err != nil {
		return "", nil, nil, err
	}
	return mimeType, img, vObj, nil
}

// Position does nothing.
func (slamSvc *FakeSLAM) Position(ctx context.Context, name string, extra map[string]interface{}) (*referenceframe.PoseInFrame, error) {
	pInFrame := referenceframe.NewPoseInFrame(name, spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector()))
	return pInFrame, nil
}

// GetInternalState does nothing.
func (slamSvc *FakeSLAM) GetInternalState(ctx context.Context, name string) ([]byte, error) {
	return []byte{0, 0, 0}, nil
}
