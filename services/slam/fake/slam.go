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

var (
	model        = resource.NewDefaultModel("fake")
	maxDataCount = 16
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
				return &FakeSLAM{Name: config.Name, dataCount: 1}, nil
			},
		},
	)
}

var _ = slam.Service(&FakeSLAM{})

// FakeSLAM is a fake slam that returns generic data.
type FakeSLAM struct {
	generic.Echo
	Name      string
	dataCount int
}

// GetMap does nothing.
func (slamSvc *FakeSLAM) GetMap(ctx context.Context, name, mimeType string, cp *referenceframe.PoseInFrame,
	include bool, extra map[string]interface{},
) (string, image.Image, *vision.Object, error) {
	var err error
	var img image.Image
	var vObj *vision.Object

	// Increment data after getMap call
	slamSvc.incrementDataCount()

	switch mimeType {
	case rdkutils.MimeTypePCD:
		pcdFile, err := os.Open(artifact.MustPath(fmt.Sprintf("slam/example_cartographer_outputs/pointcloud/pointcloud_%d.pcd", slamSvc.dataCount)))
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
		img, err = rimage.NewImageFromFile(artifact.MustPath(fmt.Sprintf("slam/example_cartographer_outputs/image_map/image_map_%d.png", slamSvc.dataCount)))
	default:
		return "", nil, nil, errors.New("received invalid mimeType for GetMap call")
	}
	if err != nil {
		return "", nil, nil, err
	}

	slamSvc.Position(ctx, name, map[string]interface{}{})
	return mimeType, img, vObj, nil
}

// Position does nothing.
func (slamSvc *FakeSLAM) Position(ctx context.Context, name string, extra map[string]interface{}) (*referenceframe.PoseInFrame, error) {
	data, err := os.ReadFile(artifact.MustPath(fmt.Sprintf("slam/example_cartographer_outputs/position/position_%d.txt", slamSvc.dataCount)))
	if err != nil {
		return nil, err
	}

	substrings := strings.Split(string(data), " | ")

	// Extract point
	pointStrings := strings.Split(substrings[0], " ")

	point := make([]float64, len(pointStrings))
	for i, v := range pointStrings {
		x, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, err
		}
		point[i] = x
	}

	// Extract orientation
	orientationStrings := strings.Split(substrings[1], " ")
	orientations := make([]float64, len(orientationStrings))
	for i, v := range orientationStrings {
		x, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, err
		}
		orientations[i] = x
	}

	ori := spatialmath.NewR4AA()
	ori.RX = orientations[0]
	ori.RY = orientations[1]
	ori.RZ = orientations[2]
	ori.Theta = orientations[3]

	pInFrame := referenceframe.NewPoseInFrame(name, spatialmath.NewPose(r3.Vector{X: point[0], Y: point[1], Z: point[2]}, ori))
	return pInFrame, nil
}

// GetInternalState does nothing.
func (slamSvc *FakeSLAM) GetInternalState(ctx context.Context, name string) ([]byte, error) {
	data, err := os.ReadFile(artifact.MustPath(fmt.Sprintf("slam/example_cartographer_outputs/internal_state/internal_state_%d.pbstream", slamSvc.dataCount)))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (slamSvc *FakeSLAM) incrementDataCount() {
	slamSvc.dataCount = ((slamSvc.dataCount + 1) % maxDataCount)
}
