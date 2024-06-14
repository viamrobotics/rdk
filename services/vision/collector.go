package vision

import (
	"context"

	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/vision/v1"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
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

type visionServiceCamera struct {
	cameraName string
}

type ImageBounds struct {
	Height int
	Width  int
}

func NewCaptureAllFromCameraCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	vision, err := assertVision(resource)
	if err != nil {
		return nil, err
	}

	cameraParam := params.MethodParams["camera_name"]

	if cameraParam == nil {
		return nil, errors.New("must specify a camera_name in the additional_params")
	}

	cameraName := new(wrapperspb.StringValue)
	if err := cameraParam.UnmarshalTo(cameraName); err != nil {
		return nil, err
	}

	minConfidenceParam := params.MethodParams["min_confidence_score"]

	minConfidenceScore := 0.5

	if minConfidenceParam != nil {
		minConfidenceScoreWrapper := new(wrapperspb.DoubleValue)
		if err := minConfidenceParam.UnmarshalTo(minConfidenceScoreWrapper); err != nil {
			return nil, err
		}

		minConfidenceScore = float64(minConfidenceScoreWrapper.Value)

		if minConfidenceScore < 0 || minConfidenceScore > 1 {
			return nil, errors.New("min_confidence_score must be between 0 and 1 inclusive")
		}
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		visCaptureOptions := viscapture.CaptureOptions{
			ReturnImage:           true,
			ReturnDetections:      true,
			ReturnClassifications: true,
			ReturnObject:          true,
		}
		visCapture, err := vision.CaptureAllFromCamera(ctx, cameraName.Value, visCaptureOptions, nil)

		if err != nil {
			return nil, data.FailedToReadErr(cameraName.Value, CaptureAllFromCamera.String(), err)
		}

		protoImage, err := imageToProto(ctx, visCapture.Image, cameraName.Value)

		if err != nil {
			return nil, err
		}

		filteredDetections := []objectdetection.Detection{}
		for _, elem := range visCapture.Detections {
			if elem.Score() >= minConfidenceScore {
				filteredDetections = append(filteredDetections, elem)
			}
		}

		protoDetections := detsToProto(filteredDetections)

		filteredClassifications := classification.Classifications{}
		for _, elem := range visCapture.Classifications {
			if elem.Score() >= minConfidenceScore {
				filteredClassifications = append(filteredClassifications, elem)
			}
		}

		protoClassifications := clasToProto(filteredClassifications)

		protoObjects, err := segmentsToProto(cameraName.Value, visCapture.Objects)

		if err != nil {
			return nil, err
		}

		imageBounds := ImageBounds{
			Height: visCapture.Image.Bounds().Dy(),
			Width:  visCapture.Image.Bounds().Dx(),
		}

		imageBoundsPb, err := protoutils.StructToStructPb(imageBounds)

		if err != nil {
			return nil, err
		}

		return &servicepb.CaptureAllFromCameraResponse{Image: protoImage, Detections: protoDetections, Classifications: protoClassifications, Objects: protoObjects, Extra: imageBoundsPb}, nil
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
