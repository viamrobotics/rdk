// Package fake implements a fake slam service
package fake

import (
	"bytes"
	"context"
	"sync/atomic"
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
	dataCount    *atomic.Int32
	logger       logging.Logger
	mapTimestamp time.Time
}

func (slamSvc *SLAM) inc() int {
	return (int(slamSvc.dataCount.Add(1)) - 1) % maxDataCount
}

func (slamSvc *SLAM) getDataCount() int {
	return int(slamSvc.dataCount.Load()) % maxDataCount
}

// NewSLAM is a constructor for a fake slam service.
func NewSLAM(name resource.Name, logger logging.Logger) *SLAM {
	var dataCount atomic.Int32
	return &SLAM{
		Named:        name.AsNamed(),
		logger:       logger,
		dataCount:    &dataCount,
		mapTimestamp: time.Now().UTC(),
	}
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
	return fakePointCloudMap(ctx, datasetDirectory, slamSvc)
}

// InternalState returns a callback function which will return the next chunk of the current internal
// state of the slam algo.
func (slamSvc *SLAM) InternalState(ctx context.Context) (func() ([]byte, error), error) {
	ctx, span := trace.StartSpan(ctx, "slam::fake::InternalState")
	defer span.End()
	return fakeInternalState(ctx, datasetDirectory, slamSvc)
}

// Properties returns the mapping mode of the slam service as well as a boolean indicating if it is running
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
