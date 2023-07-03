// Package fake implements a fake slam service
package fake

import (
	"bytes"
	"context"
	"time"

	"github.com/edaniels/golog"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

var model = resource.DefaultModelFamily.WithModel("fake")

const datasetDirectory = "slam/example_cartographer_outputs/viam-office-02-22-3"

func init() {
	resource.RegisterService(
		slam.API,
		model,
		resource.Registration[slam.Service, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (slam.Service, error) {
				return NewSLAM(conf.ResourceName(), logger), nil
			},
		},
	)
}

// SLAM is a fake slam that returns generic data.
type SLAM struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	dataCount    int
	logger       golog.Logger
	mapTimestamp time.Time
}

// NewSLAM is a constructor for a fake slam service.
func NewSLAM(name resource.Name, logger golog.Logger) *SLAM {
	return &SLAM{
		Named:        name.AsNamed(),
		logger:       logger,
		dataCount:    -1,
		mapTimestamp: time.Now().UTC(),
	}
}

func (slamSvc *SLAM) getCount() int {
	if slamSvc.dataCount < 0 {
		return 0
	}
	return slamSvc.dataCount
}

// GetPosition returns a Pose and a component reference string of the robot's current location according to SLAM.
func (slamSvc *SLAM) GetPosition(ctx context.Context) (spatialmath.Pose, string, error) {
	ctx, span := trace.StartSpan(ctx, "slam::fake::GetPosition")
	defer span.End()
	return fakeGetPosition(ctx, datasetDirectory, slamSvc)
}

// GetPointCloudMap returns a callback function which will return the next chunk of the current pointcloud
// map.
func (slamSvc *SLAM) GetPointCloudMap(ctx context.Context) (func() ([]byte, error), error) {
	ctx, span := trace.StartSpan(ctx, "slam::fake::GetPointCloudMap")
	defer span.End()
	slamSvc.incrementDataCount()
	slamSvc.mapTimestamp = time.Now().UTC()
	return fakeGetPointCloudMap(ctx, datasetDirectory, slamSvc)
}

// GetInternalState returns a callback function which will return the next chunk of the current internal
// state of the slam algo.
func (slamSvc *SLAM) GetInternalState(ctx context.Context) (func() ([]byte, error), error) {
	ctx, span := trace.StartSpan(ctx, "slam::fake::GetInternalState")
	defer span.End()
	return fakeGetInternalState(ctx, datasetDirectory, slamSvc)
}

// GetLatestMapInfo returns a message indicating details regarding the latest map returned to the system.
func (slamSvc *SLAM) GetLatestMapInfo(ctx context.Context) (time.Time, error) {
	ctx, span := trace.StartSpan(ctx, "slam::fake::GetLatestMapInfo")
	defer span.End()
	return slamSvc.mapTimestamp, nil
}

// incrementDataCount is not thread safe but that is ok as we only intend a single user to be interacting
// with it at a time.
func (slamSvc *SLAM) incrementDataCount() {
	slamSvc.dataCount = ((slamSvc.dataCount + 1) % maxDataCount)
}

// GetLimits returns the bounds of the slam map as a list of referenceframe.Limits.
func (slamSvc *SLAM) GetLimits(ctx context.Context) ([]referenceframe.Limit, error) {
	data, err := slam.GetPointCloudMapFull(ctx, slamSvc)
	if err != nil {
		return nil, err
	}
	dims, err := pointcloud.GetPCDMetaData(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return []referenceframe.Limit{
		{Min: dims.MinX, Max: dims.MaxX},
		{Min: dims.MinY, Max: dims.MaxY},
	}, nil
}
