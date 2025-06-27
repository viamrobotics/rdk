package slam

import (
	"context"
	"errors"
	"time"

	pb "go.viam.com/api/service/slam/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/spatialmath"
)

type method int64

const (
	position method = iota
	pointCloudMap
	doCommand
)

func (m method) String() string {
	if m == position {
		return "Position"
	}
	if m == pointCloudMap {
		return "PointCloudMap"
	}
	if m == doCommand {
		return "DoCommand"
	}
	return ""
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
			return res, data.NewFailedToReadError(params.ComponentName, position.String(), err)
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
			return res, data.NewFailedToReadError(params.ComponentName, pointCloudMap.String(), err)
		}

		pcd, err := HelperConcatenateChunksToFull(f)
		if err != nil {
			return res, data.NewFailedToReadError(params.ComponentName, pointCloudMap.String(), err)
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

// newDoCommandCollector returns a collector to register a doCommand action. If one is already registered
// with the same MethodMetadata it will panic.
func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	slam, err := assertSLAM(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult

		var payload map[string]interface{}

		if payloadAny, exists := params.MethodParams["docommand_input"]; exists && payloadAny != nil {
			if payloadAny.MessageIs(&structpb.Struct{}) {
				var s structpb.Struct
				if err := payloadAny.UnmarshalTo(&s); err != nil {
					return res, err
				}
				payload = s.AsMap()
			} else {
				// handle empty payload
				payload = make(map[string]interface{})
			}
		} else {
			// key does not exist
			return res, errors.New("missing payload")
		}

		values, err := slam.DoCommand(ctx, payload)
		if err != nil {
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, "DoCommand", err)
		}
		ts := data.Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return data.NewTabularCaptureResultDoCommand(ts, values)
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
