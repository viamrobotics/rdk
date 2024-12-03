package slam

import (
	"context"
	"time"

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

func newPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	slam, err := assertSLAM(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		pose, err := slam.Position(ctx)
		if err != nil {
			return res, data.FailedToReadErr(params.ComponentName, position.String(), err)
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResult(ts, &pb.GetPositionResponse{Pose: spatialmath.PoseToProtobuf(pose)})
	})
	return data.NewCollector(cFunc, params)
}

func newPointCloudMapCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	slam, err := assertSLAM(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		// edited maps do not need to be captured because they should not be modified
		f, err := slam.PointCloudMap(ctx, false)
		if err != nil {
			return res, data.FailedToReadErr(params.ComponentName, pointCloudMap.String(), err)
		}

		pcd, err := HelperConcatenateChunksToFull(f)
		if err != nil {
			return res, data.FailedToReadErr(params.ComponentName, pointCloudMap.String(), err)
		}

		ts := data.Timestamps{
			TimeRequested: timeRequested,
			TimeReceived:  time.Now(),
		}
		return data.NewBinaryCaptureResult(ts, []data.Binary{{
			Payload:  pcd,
			MimeType: data.MimeTypeApplicationPcd,
		}}), nil
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
