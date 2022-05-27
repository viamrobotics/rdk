package camera

import (
	"bytes"
	"context"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/pointcloud"
)

type method int64

const (
	nextPointCloud method = iota
)

func (m method) String() string {
	if m == nextPointCloud {
		return "NextPointCloud"
	}
	return "Unknown"
}

// TODO: add tests for this file.

func newNextPointCloudCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	camera, err := assertCamera(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		_, span := trace.StartSpan(ctx, "camera::data::collector::CaptureFunc")
		defer span.End()

		v, err := camera.NextPointCloud(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, nextPointCloud.String(), err)
		}

		var buf bytes.Buffer
		buf.Grow(v.Size() * 4 * 4) // 4 numbers per point, each 4 bytes
		err = pointcloud.ToPCD(v, &buf, pointcloud.PCDBinary)
		if err != nil {
			return nil, errors.Errorf("failed to convert returned point cloud to PCD: %v", err)
		}
		return buf, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertCamera(resource interface{}) (Camera, error) {
	cam, ok := resource.(Camera)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return cam, nil
}
