// Package fake implements a fake slam service
package fake

import (
	"bytes"
	"context"
	"time"

	"go.opencensus.io/trace"

	"go.viam.com/rdk/logging"
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
				logger logging.Logger,
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
	logger       logging.Logger
	mapTimestamp time.Time
}

// NewSLAM is a constructor for a fake slam service.
func NewSLAM(name resource.Name, logger logging.Logger) *SLAM {
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

// Position returns a Pose and a component reference string of the robot's current location according to SLAM.
func (slamSvc *SLAM) Position(ctx context.Context) (spatialmath.Pose, string, error) {
	ctx, span := trace.StartSpan(ctx, "slam::fake::Position")
	defer span.End()
	return fakePosition(ctx, datasetDirectory, slamSvc)
}

// PointCloudMap returns a callback function which will return the next chunk of the current pointcloud
// map.
func (slamSvc *SLAM) PointCloudMap(ctx context.Context) (func() ([]byte, error), error) {
	ctx, span := trace.StartSpan(ctx, "slam::fake::PointCloudMap")
	defer span.End()
	slamSvc.incrementDataCount()
	return fakePointCloudMap(ctx, datasetDirectory, slamSvc)
}

// InternalState returns a callback function which will return the next chunk of the current internal
// state of the slam algo.
func (slamSvc *SLAM) InternalState(ctx context.Context) (func() ([]byte, error), error) {
	ctx, span := trace.StartSpan(ctx, "slam::fake::InternalState")
	defer span.End()
	return fakeInternalState(ctx, datasetDirectory, slamSvc)
}

// LatestMapInfo returns information used to determine whether the slam mode is localizing.
// Fake Slam is always in mapping mode, so it always returns a new timestamp.
func (slamSvc *SLAM) LatestMapInfo(ctx context.Context) (time.Time, error) {
	_, span := trace.StartSpan(ctx, "slam::fake::LatestMapInfo")
	defer span.End()
	slamSvc.mapTimestamp = time.Now().UTC()
	return slamSvc.mapTimestamp, nil
}

// Properties returns the mapping mode of the slam service as well as a boolean indicating if it running
// in the cloud or locally. In the case of fake slam, it will return that the service is being run locally
// and is creating a new map.
func (slamSvc *SLAM) Properties(ctx context.Context) (slam.Properties, error) {
	_, span := trace.StartSpan(ctx, "slam::fake::Properties")
	defer span.End()

	prop := slam.Properties{
		CloudSlam:   false,
		MappingMode: slam.MappingModeNewMap,
	}
	return prop, nil
}

// incrementDataCount is not thread safe but that is ok as we only intend a single user to be interacting
// with it at a time.
func (slamSvc *SLAM) incrementDataCount() {
	slamSvc.dataCount = ((slamSvc.dataCount + 1) % maxDataCount)
}

// Limits returns the bounds of the slam map as a list of referenceframe.Limits.
func (slamSvc *SLAM) Limits(ctx context.Context) ([]referenceframe.Limit, error) {
	data, err := slam.PointCloudMapFull(ctx, slamSvc)
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
