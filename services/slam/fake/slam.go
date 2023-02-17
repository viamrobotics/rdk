// Package fake implements a fake base.
package fake

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
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
	positionTemplate      = "slam/example_cartographer_outputs/position/position_%d.txt"
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
				return &SLAM{Name: config.Name, logger: logger}, nil
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
	logger    golog.Logger
}

// GetMap returns either a vision.Object or image.Image based on request mimeType.
func (slamSvc *SLAM) GetMap(ctx context.Context, name, mimeType string, cp *referenceframe.PoseInFrame,
	include bool, extra map[string]interface{},
) (string, image.Image, *vision.Object, error) {
	slamSvc.incrementDataCount()

	var err error
	var img image.Image
	var vObj *vision.Object

	switch mimeType {
	case rdkutils.MimeTypePCD:
		path := filepath.Clean(artifact.MustPath(fmt.Sprintf(pcdTemplate, slamSvc.dataCount)))
		slamSvc.logger.Debug("Reading " + path)

		f, err := os.Open(path)
		if err != nil {
			return "", nil, nil, err
		}
		defer utils.UncheckedErrorFunc(f.Close)

		pc, err := pointcloud.ReadPCD(f)
		if err != nil {
			return "", nil, nil, err
		}

		vObj, err = vision.NewObject(pc)
		if err != nil {
			return "", nil, nil, err
		}

	case rdkutils.MimeTypeJPEG:
		path := filepath.Clean(artifact.MustPath(fmt.Sprintf(pngTemplate, slamSvc.dataCount)))
		slamSvc.logger.Debug("Reading " + path)
		img, err = rimage.NewImageFromFile(path)

	default:
		return "", nil, nil, errors.New("received invalid mimeType for GetMap call")
	}

	if err != nil {
		return "", nil, nil, err
	}

	return mimeType, img, vObj, nil
}

// Position returns a PoseInFrame of the robot's current location according to SLAM.
func (slamSvc *SLAM) Position(ctx context.Context, name string, extra map[string]interface{}) (*referenceframe.PoseInFrame, error) {
	path := filepath.Clean(artifact.MustPath(fmt.Sprintf(positionTemplate, slamSvc.dataCount)))
	slamSvc.logger.Debug("Reading " + path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	substrings := strings.Split(string(data), " | ")

	rawXYZ, err := extract(strings.Split(substrings[0], " "))
	if err != nil {
		return nil, err
	}
	xyz := r3.Vector{X: rawXYZ[0], Y: rawXYZ[1], Z: rawXYZ[2]}

	rawAA, err := extract(strings.Split(substrings[1], " "))
	if err != nil {
		return nil, err
	}

	axisAngle := spatialmath.NewR4AA().AxisAngles()
	axisAngle.RX, axisAngle.RY, axisAngle.RZ, axisAngle.Theta = rawAA[0], rawAA[1], rawAA[2], rawAA[3]
	pose := spatialmath.NewPose(xyz, axisAngle)
	pInFrame := referenceframe.NewPoseInFrame(name, pose)

	return pInFrame, nil
}

// GetInternalState returns the internal state of a slam algo. Currently the internal state of cartogropher.
func (slamSvc *SLAM) GetInternalState(ctx context.Context, name string) ([]byte, error) {
	path := filepath.Clean(artifact.MustPath(fmt.Sprintf(internalStateTemplate, slamSvc.dataCount)))
	slamSvc.logger.Debug("Reading " + path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// incrementDataCount is not thread safe but that is ok as we only intend a single user to be interacting
// with it at a time.
func (slamSvc *SLAM) incrementDataCount() {
	slamSvc.dataCount = ((slamSvc.dataCount + 1) % maxDataCount)
}

func extract(s []string) ([]float64, error) {
	elems := make([]float64, len(s))
	for i, v := range s {
		x, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return nil, err
		}
		elems[i] = x
	}
	return elems, nil
}
