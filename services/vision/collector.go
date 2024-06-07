package vision

import (
	"context"

	servicepb "go.viam.com/api/service/vision/v1"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/vision/viscapture"
)

type method int64

const (
	captureAllFromCamera method = iota
)

func (m method) String() string {
	switch m {
	case captureAllFromCamera:
		return "CaptureAllFromCamera"
	}
	return "Unknown"
}

func newCaptureAllFromCameraCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	vision, err := assertVision(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		visCaptureOptions := viscapture.CaptureOptions{
			ReturnImage:           true,
			ReturnDetections:      true,
			ReturnClassifications: true,
			ReturnObject:          true,
		}
		visCapture, err := vision.CaptureAllFromCamera(ctx, params.ComponentName, visCaptureOptions, nil)

		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, captureAllFromCamera.String(), err)
		}

		protoImage, err := imageToProto(ctx, visCapture.Image, params.ComponentName)

		if err != nil {
			return nil, err
		}

		protoObjects, err := segmentsToProto(params.ComponentName, visCapture.Objects)

		if err != nil {
			return nil, err
		}

		return &servicepb.CaptureAllFromCameraResponse{Image: protoImage, Detections: detsToProto(visCapture.Detections), Classifications: clasToProto(visCapture.Classifications), Objects: protoObjects}, nil
	})

	return data.NewCollector(cFunc, params)
}

func assertVision(resource interface{}) (Service, error) {
	visionService, ok := resource.(Service)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}

	return visionService, nil
}
