package camera

import (
	"bytes"
	"context"
	"time"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/utils"
)

type method int64

const (
	nextPointCloud method = iota
	readImage
	getImages
)

func (m method) String() string {
	switch m {
	case nextPointCloud:
		return "NextPointCloud"
	case readImage:
		return "ReadImage"
	case getImages:
		return "GetImages"
	}
	return "Unknown"
}

func newNextPointCloudCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	camera, err := assertCamera(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		_, span := trace.StartSpan(ctx, "camera::data::collector::CaptureFunc::NextPointCloud")
		defer span.End()

		ctx = context.WithValue(ctx, data.FromDMContextKey{}, true)

		v, err := camera.NextPointCloud(ctx)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, nextPointCloud.String(), err)
		}

		var buf bytes.Buffer
		headerSize := 200
		if v != nil {
			buf.Grow(headerSize + v.Size()*4*4) // 4 numbers per point, each 4 bytes
			err = pointcloud.ToPCD(v, &buf, pointcloud.PCDBinary)
			if err != nil {
				return res, errors.Errorf("failed to convert returned point cloud to PCD: %v", err)
			}
		}
		ts := data.Timestamps{
			TimeRequested: timeRequested,
			TimeReceived:  time.Now(),
		}
		return data.NewBinaryCaptureResult(ts, []data.Binary{{
			Payload:  buf.Bytes(),
			MimeType: data.MimeTypeApplicationPcd,
		}}), nil
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
		strWrapper := wrapperspb.String(utils.MimeTypeJPEG)
		mimeType, err = anypb.New(strWrapper)
		if err != nil {
			return nil, err
		}
	}

	mimeStr := new(wrapperspb.StringValue)
	if err := mimeType.UnmarshalTo(mimeStr); err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		_, span := trace.StartSpan(ctx, "camera::data::collector::CaptureFunc::ReadImage")
		defer span.End()

		img, metadata, err := camera.Image(ctx, mimeStr.Value, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}

			return res, data.FailedToReadErr(params.ComponentName, readImage.String(), err)
		}

		mimeType := data.CameraFormatToMimeType(utils.MimeTypeToFormat[metadata.MimeType])
		ts := data.Timestamps{
			TimeRequested: timeRequested,
			TimeReceived:  time.Now(),
		}
		return data.NewBinaryCaptureResult(ts, []data.Binary{{
			MimeType: mimeType,
			Payload:  img,
		}}), nil
	})
	return data.NewCollector(cFunc, params)
}

func newGetImagesCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	camera, err := assertCamera(resource)
	if err != nil {
		return nil, err
	}
	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		var res data.CaptureResult
		_, span := trace.StartSpan(ctx, "camera::data::collector::CaptureFunc::GetImages")
		defer span.End()
		ctx = context.WithValue(ctx, data.FromDMContextKey{}, true)

		resImgs, resMetadata, err := camera.Images(ctx)
		if err != nil {
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, getImages.String(), err)
		}

		var binaries []data.Binary
		for _, img := range resImgs {
			format, imgBytes, err := encodeImageFromUnderlyingType(ctx, img.Image)
			if err != nil {
				return res, err
			}
			binaries = append(binaries, data.Binary{
				Annotations: data.Annotations{Classifications: []data.Classification{{Label: img.SourceName}}},
				Payload:     imgBytes,
				MimeType:    data.CameraFormatToMimeType(format),
			})
		}
		ts := data.Timestamps{
			TimeRequested: resMetadata.CapturedAt,
			TimeReceived:  resMetadata.CapturedAt,
		}
		return data.NewBinaryCaptureResult(ts, binaries), nil
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
