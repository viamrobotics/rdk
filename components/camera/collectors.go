package camera

import (
	"bytes"
	"context"
	"strings"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

type method int64

const (
	nextPointCloud method = iota
	readImage      method = iota
)

func (m method) String() string {
	switch m {
	case nextPointCloud:
		return "NextPointCloud"
	case readImage:
		return "ReadImage"
	}
	return "Unknown"
}

// TODO: add tests for this file.

func newNextPointCloudCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	camera, err := assertCamera(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		_, span := trace.StartSpan(ctx, "camera::data::collector::CaptureFunc::NextPointCloud")
		defer span.End()

		ctx = context.WithValue(ctx, data.FromDMContextKey{}, true)

		v, err := camera.NextPointCloud(ctx)
		if err != nil {
			// If err is from a modular filter component, propagate it to getAndPushNextReading(). We check for the substring presence here
			// because the error thrown by the modular component may be wrapped in an rpc error or may not be a golang error.
			if strings.Contains(err.Error(), data.ErrNoCaptureToStore.Error()) {
				return nil, data.ErrNoCaptureToStore
			}
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

func newReadImageCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	camera, err := assertCamera(resource)
	if err != nil {
		return nil, err
	}
	// choose the best/fastest representation
	mimeType := params.MethodParams["mime_type"]
	if mimeType == nil {
		// TODO: Potentially log the actual mime type at collector instantiation or include in response.
		strWrapper := wrapperspb.String(utils.MimeTypeRawRGBA)
		mimeType, err = anypb.New(strWrapper)
		if err != nil {
			return nil, err
		}
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		_, span := trace.StartSpan(ctx, "camera::data::collector::CaptureFunc::ReadImage")
		defer span.End()

		ctx = context.WithValue(ctx, data.FromDMContextKey{}, true)

		img, release, err := ReadImage(ctx, camera)
		if err != nil {
			// If err is from a modular filter component, propagate it to getAndPushNextReading().
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, data.ErrNoCaptureToStore
			}

			return nil, data.FailedToReadErr(params.ComponentName, readImage.String(), err)
		}
		defer func() {
			if release != nil {
				release()
			}
		}()

		mimeStr := new(wrapperspb.StringValue)
		if err := mimeType.UnmarshalTo(mimeStr); err != nil {
			return nil, err
		}

		outBytes, err := rimage.EncodeImage(ctx, img, mimeStr.Value)
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
		return nil, data.InvalidInterfaceErr(API)
	}
	return cam, nil
}
