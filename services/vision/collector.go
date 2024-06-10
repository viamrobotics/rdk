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
	CaptureAllFromCamera method = iota
)

func (m method) String() string {
	switch m {
	case CaptureAllFromCamera:
		return "CaptureAllFromCamera"
	}
	return "Unknown"
}

func NewCaptureAllFromCameraCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
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
		visCapture, err := vision.CaptureAllFromCamera(ctx, "camera-1", visCaptureOptions, nil)

		if err != nil {
			return nil, data.FailedToReadErr("camera-1", CaptureAllFromCamera.String(), err)
		}

		protoImage, err := imageToProto(ctx, visCapture.Image, "camera-1")

		if err != nil {
			return nil, err
		}

		protoObjects, err := segmentsToProto("camera-1", visCapture.Objects)

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
