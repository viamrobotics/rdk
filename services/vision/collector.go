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
	captureAllFromCamera method = iota
)

func (m method) String() string {
	if m == captureAllFromCamera {
		return "CaptureAllFromCamera"
	}

	return "Unknown"
}

type extraFields struct {
	Height   int
	Width    int
	MimeType string
}

func newCaptureAllFromCameraCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	vision, err := assertVision(resource)
	if err != nil {
		return nil, err
	}

	cameraParam := params.MethodParams["camera_name"]

	if cameraParam == nil {
		return nil, errors.New("must specify a camera_name in the additional_params")
	}

	var cameraName string

	cameraNameWrapper := new(wrapperspb.StringValue)
	if err := cameraParam.UnmarshalTo(cameraNameWrapper); err != nil {
		return nil, err
	}
	cameraName = cameraNameWrapper.Value

	mimeTypeParam := params.MethodParams["mime_type"]

	if mimeTypeParam == nil {
		return nil, errors.New("must specify a mime_type in the additional_params")
	}

	var mimeType string

	mimeTypeWrapper := new(wrapperspb.StringValue)
	if err := mimeTypeParam.UnmarshalTo(mimeTypeWrapper); err != nil {
		return nil, err
	}

	mimeType = mimeTypeWrapper.Value

	minConfidenceParam := params.MethodParams["min_confidence_score"]

	// Default min_confidence_score is 0.5
	minConfidenceScore := 0.5

	if minConfidenceParam != nil {
		minConfidenceScoreWrapper := new(wrapperspb.DoubleValue)
		if err := minConfidenceParam.UnmarshalTo(minConfidenceScoreWrapper); err != nil {
			return nil, err
		}

		minConfidenceScore = minConfidenceScoreWrapper.Value

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
		visCapture, err := vision.CaptureAllFromCamera(ctx, cameraName, visCaptureOptions, nil)
		if err != nil {
			return nil, data.FailedToReadErr(cameraName, captureAllFromCamera.String(), err)
		}

		protoImage, err := imageToProto(ctx, visCapture.Image, cameraName)
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

		protoObjects, err := segmentsToProto(cameraName, visCapture.Objects)
		if err != nil {
			return nil, err
		}

		bounds := extraFields{}

		if visCapture.Image != nil {
			bounds = extraFields{
				Height:   visCapture.Image.Bounds().Dy(),
				Width:    visCapture.Image.Bounds().Dx(),
				MimeType: mimeType,
			}
		}

		boundsPb, err := protoutils.StructToStructPb(bounds)
		if err != nil {
			return nil, err
		}

		return &servicepb.CaptureAllFromCameraResponse{
			Image: protoImage, Detections: protoDetections, Classifications: protoClassifications,
			Objects: protoObjects, Extra: boundsPb,
		}, nil
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
