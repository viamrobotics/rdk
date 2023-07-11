package slam

import (
	"context"

	"go.opencensus.io/trace"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/spatialmath"
)

type method int64

const (
	getPosition      method = iota
	getPointCloudMap method = iota
)

func (m method) String() string {
	switch m {
	case getPosition:
		return "GetPosition"
	case getPointCloudMap:
		return "GetPointCloudMap"
	}
	return "Unknown"
}

// Position defines the struct returned by the capturer for GetPosition.
type Position struct {
	Pose               spatialmath.Pose
	ComponentReference string
}

func newGetPositionCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	slam, err := assertSLAM(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		_, span := trace.StartSpan(ctx, "slam::data::collector::CaptureFunc::GetPosition")
		defer span.End()

		pose, componentRef, err := slam.GetPosition(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, getPosition.String(), err)
		}
		return Position{Pose: pose, ComponentReference: componentRef}, nil
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
