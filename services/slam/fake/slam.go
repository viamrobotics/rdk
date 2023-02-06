// Package fake implements a fake base.
package fake

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"strconv"
	"strings"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/utils"
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

const (
	internalStateTemplate = "slam/example_cartographer_outputs/internal_state/internal_state_%d.pbstream"
	maxDataCount          = 16
	pcdTemplate           = "slam/example_cartographer_outputs/pointcloud/pointcloud_%d.pcd"
	pngTemplate           = "slam/example_cartographer_outputs/image_map/image_map_%d.png"
)

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
				return &SLAM{Name: config.Name, dataCount: 1}, nil
			},
		},
	)
}

var _ = slam.Service(&SLAM{})

// SLAM is a fake slam that returns generic data.
type SLAM struct {
	generic.Echo
	Name      string
	dataCount int
}

func (slamSvc *SLAM) GetMap(ctx context.Context, name, mimeType string, cp *referenceframe.PoseInFrame,
	include bool, extra map[string]interface{},
) (string, image.Image, *vision.Object, error) {
	var err error
	var img image.Image
	var vObj *vision.Object

	// Increment data after getMap call
	slamSvc.incrementDataCount()

	switch mimeType {
	case rdkutils.MimeTypePCD:
		f, err := os.Open(artifact.MustPath(fmt.Sprintf(pcdTemplate, slamSvc.dataCount)))
		if err != nil {
			return "", nil, nil, err
		}
		defer utils.UncheckedErrorFunc(f.Close)
		pc, err := pointcloud.ReadPCDToBasicOctree(f)
		if err != nil {
			return "", nil, nil, err
		}
		vObj, err = vision.NewObject(pc)
		if err != nil {
			return "", nil, nil, err
		}

	case rdkutils.MimeTypeJPEG:
		img, err = rimage.NewImageFromFile(artifact.MustPath(fmt.Sprintf(pngTemplate, slamSvc.dataCount)))

	default:
		return "", nil, nil, errors.New("received invalid mimeType for GetMap call")
	}

	if err != nil {
		return "", nil, nil, err
	}

	return mimeType, img, vObj, nil
}

func (slamSvc *SLAM) Position(ctx context.Context, name string, extra map[string]interface{}) (*referenceframe.PoseInFrame, error) {
	data, err := os.ReadFile(artifact.MustPath(fmt.Sprintf("slam/example_cartographer_outputs/position/position_%d.txt", slamSvc.dataCount)))
	if err != nil {
		return nil, err
	}

	substrings := strings.Split(string(data), " | ")

	point, err := extract(strings.Split(substrings[0], " "))
	if err != nil {
		return nil, err
	}
	xyz := r3.Vector{X: point[0], Y: point[1], Z: point[2]}

	orientations, err := extract(strings.Split(substrings[1], " "))
	if err != nil {
		return nil, err
	}

	ori := spatialmath.NewR4AA()
	ori.RX = orientations[0]
	ori.RY = orientations[1]
	ori.RZ = orientations[2]
	ori.Theta = orientations[3]
	pose := spatialmath.NewPose(xyz, ori)

	pInFrame := referenceframe.NewPoseInFrame(name, pose)

	return pInFrame, nil
}

func extract(strings []string) ([]float64, error) {
	elems := make([]float64, len(strings))
	for i, v := range strings {
		x, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, err
		}
		elems[i] = x
	}
	return elems, nil
}

func (slamSvc *SLAM) GetInternalState(ctx context.Context, name string) ([]byte, error) {
	data, err := os.ReadFile(artifact.MustPath(fmt.Sprintf(internalStateTemplate, slamSvc.dataCount)))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (slamSvc *SLAM) incrementDataCount() {
	slamSvc.dataCount = ((slamSvc.dataCount + 1) % maxDataCount)
}
