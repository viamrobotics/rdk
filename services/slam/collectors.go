package slam

import (
	"context"

	pb "go.viam.com/api/service/slam/v1"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/spatialmath"
)

type method int64

const (
	position method = iota
	pointCloudMap
)

func (m method) String() string {
	if m == position {
		return "Position"
	}
	if m == pointCloudMap {
		return "PointCloudMap"
	}
	return "Unknown"
}

func NewPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	slam, err := assertSLAM(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		pose, err := slam.Position(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, position.String(), err)
		}
		return &pb.GetPositionResponse{Pose: spatialmath.PoseToProtobuf(pose)}, nil
	})
	return data.NewCollector(cFunc, params)
}

// NewPointCloudMapCollector.
func NewPointCloudMapCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	slam, err := assertSLAM(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		// edited maps do not need to be captured because they should not be modified
		f, err := slam.PointCloudMap(ctx, false)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, pointCloudMap.String(), err)
		}

		pcd, err := HelperConcatenateChunksToFull(f)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, pointCloudMap.String(), err)
		}

		return pcd, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertSLAM(resource interface{}) (Service, error) {
	slamService, ok := resource.(Service)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return slamService, nil
}
