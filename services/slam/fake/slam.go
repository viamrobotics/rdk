// Package fake implements a fake slam service.
package fake

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/vision"
)

var model = resource.NewDefaultModel("fake")

const datasetDirectory = "slam/example_cartographer_outputs/viam-office-02-22-1"

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
				return &SLAM{Name: config.Name, logger: logger, dataCount: -1}, nil
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

func (slamSvc *SLAM) getCount() int {
	if slamSvc.dataCount < 0 {
		return 0
	}
	return slamSvc.dataCount
}

// GetMap returns either a vision.Object or image.Image based on request mimeType.
func (slamSvc *SLAM) GetMap(ctx context.Context, name, mimeType string, cp *referenceframe.PoseInFrame,
	include bool, extra map[string]interface{},
) (string, image.Image, *vision.Object, error) {
	slamSvc.incrementDataCount()
	return fakeGetMap(datasetDirectory, slamSvc, mimeType)
}

// Position returns a PoseInFrame of the robot's current location according to SLAM.
func (slamSvc *SLAM) Position(ctx context.Context, name string, extra map[string]interface{}) (*referenceframe.PoseInFrame, error) {
	return fakePosition(datasetDirectory, slamSvc, name)
}

// GetInternalState returns the internal state of a slam algo. Currently the internal state of cartographer.
func (slamSvc *SLAM) GetInternalState(ctx context.Context, name string) ([]byte, error) {
	return fakeGetInternalState(datasetDirectory, slamSvc)
}

// GetPointCloudMapStream returns a callback function which will return the next chunk of the current pointcloud
// map.
func (slamSvc *SLAM) GetPointCloudMapStream(ctx context.Context, name string) (func() ([]byte, error), error) {
	return nil, errors.New("unimplemented stub")
}

// GetInternalStateStream returns a callback function which will return the next chunk of the current internal
// state of the slam algo.
func (slamSvc *SLAM) GetInternalStateStream(ctx context.Context, name string) (func() ([]byte, error), error) {
	return nil, errors.New("unimplemented stub")
}

// incrementDataCount is not thread safe but that is ok as we only intend a single user to be interacting
// with it at a time.
func (slamSvc *SLAM) incrementDataCount() {
	slamSvc.dataCount = ((slamSvc.dataCount + 1) % maxDataCount)
}
