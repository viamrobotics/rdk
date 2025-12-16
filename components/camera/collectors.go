package camera

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils/trace"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/pointcloud"
)

type method int64

const (
	nextPointCloud method = iota
	readImage
	getImages
	doCommand
)

func (m method) String() string {
	switch m {
	case nextPointCloud:
		return "NextPointCloud"
	case readImage:
		return "ReadImage"
	case getImages:
		return "GetImages"
	case doCommand:
		return "DoCommand"
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

		pc, err := camera.NextPointCloud(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, nextPointCloud.String(), err)
		}
		bytes, err := pointcloud.ToBytes(pc)
		if err != nil {
			return res, errors.Errorf("failed to convert returned point cloud to PCD: %v", err)
		}
		ts := data.Timestamps{
			TimeRequested: timeRequested,
			TimeReceived:  time.Now(),
		}
		return data.NewBinaryCaptureResult(ts, []data.Binary{{
			Payload:  bytes,
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
	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		_, span := trace.StartSpan(ctx, "camera::data::collector::CaptureFunc::ReadImage")
		defer span.End()

		resImgs, resMetadata, err := camera.Images(ctx, nil, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, readImage.String(), err)
		}
		var binaries []data.Binary
		if len(resImgs) == 0 {
			return data.NewBinaryCaptureResult(data.Timestamps{}, binaries), nil
		}
		img := resImgs[0]
		imgBytes, err := img.Bytes(ctx)
		if err != nil {
			return res, data.NewFailedToReadError(params.ComponentName, getImages.String(), err)
		}
		annotations := img.Annotations
		annotations.Classifications = append(annotations.Classifications, data.Classification{Label: img.SourceName})
		binaries = append(binaries, data.Binary{
			Annotations: annotations,
			Payload:     imgBytes,
			MimeType:    data.MimeTypeStringToMimeType(img.MimeType()),
		})

		ts := data.Timestamps{
			TimeRequested: timeRequested,
			TimeReceived:  resMetadata.CapturedAt,
		}
		return data.NewBinaryCaptureResult(ts, binaries), nil
	})
	return data.NewCollector(cFunc, params)
}

func newGetImagesCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	camera, err := assertCamera(resource)
	if err != nil {
		return nil, err
	}
	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		_, span := trace.StartSpan(ctx, "camera::data::collector::CaptureFunc::GetImages")
		defer span.End()

		resImgs, resMetadata, err := camera.Images(ctx, nil, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.NewFailedToReadError(params.ComponentName, getImages.String(), err)
		}

		var binaries []data.Binary
		for _, img := range resImgs {
			imgBytes, err := img.Bytes(ctx)
			if err != nil {
				return res, data.NewFailedToReadError(params.ComponentName, getImages.String(), err)
			}
			annotations := img.Annotations
			annotations.Classifications = append(annotations.Classifications, data.Classification{Label: img.SourceName})
			binaries = append(binaries, data.Binary{
				Annotations: annotations,
				Payload:     imgBytes,
				MimeType:    data.MimeTypeStringToMimeType(img.MimeType()),
			})
		}
		ts := data.Timestamps{
			TimeRequested: timeRequested,
			TimeReceived:  resMetadata.CapturedAt,
		}
		return data.NewBinaryCaptureResult(ts, binaries), nil
	})
	return data.NewCollector(cFunc, params)
}

// newDoCommandCollector returns a collector to register a doCommand action. If one is already registered
// with the same MethodMetadata it will panic.
func newDoCommandCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	camera, err := assertCamera(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.NewDoCommandCaptureFunc(camera, params)
	return data.NewCollector(cFunc, params)
}

func assertCamera(resource interface{}) (Camera, error) {
	cam, ok := resource.(Camera)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return cam, nil
}
