package camera

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

type method int64

const (
	nextPointCloud method = iota
	next           method = iota
)

func (m method) String() string {
	switch m {
	case nextPointCloud:
		return "NextPointCloud"
	case next:
		return "Next"
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
		_, span := trace.StartSpan(ctx, "camera::data::collector::CaptureFunc::NextPointCloud")
		defer span.End()

		v, err := camera.NextPointCloud(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, nextPointCloud.String(), err)
		}

		var buf bytes.Buffer
		headerSize := 200
		buf.Grow(headerSize + v.Size()*4*4) // 4 numbers per point, each 4 bytes
		err = pointcloud.ToPCD(v, &buf, pointcloud.PCDBinary)
		if err != nil {
			return nil, errors.Errorf("failed to convert returned point cloud to PCD: %v", err)
		}
		return buf.Bytes(), nil
	})
	return data.NewCollector(cFunc, params)
}

func newNextCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	camera, err := assertCamera(resource)
	if err != nil {
		return nil, err
	}
	// choose the best/fastest representation
	mimeType := params.MethodParams["mime_type"]
	if mimeType == "" || mimeType == utils.MimeTypeViamBest {
		// TODO: Potentially log the actual mime type at collector instantiation or include in response.
		mimeType = utils.MimeTypeRawRGBA
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		_, span := trace.StartSpan(ctx, "camera::data::collector::CaptureFunc::Next")
		defer span.End()

		img, release, err := camera.Next(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, next.String(), err)
		}
		defer func() {
			if release != nil {
				release()
			}
		}()

		outBytes, err := rimage.EncodeImage(ctx, img, mimeType)
		if err != nil {
			return nil, err
		}
		return outBytes, nil
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
