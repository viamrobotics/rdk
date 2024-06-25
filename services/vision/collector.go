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

type methodParamsDecoded struct {
	cameraName    string
	mimeType      string
	minConfidence float64
}

func newCaptureAllFromCameraCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	vision, err := assertVision(resource)
	if err != nil {
		return nil, err
	}

	decodedParams, err := additionalParamExtraction(params.MethodParams)
	if err != nil {
		return nil, err
	}

	cameraName := decodedParams.cameraName
	mimeType := decodedParams.mimeType
	minConfidenceScore := decodedParams.minConfidence

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

		// We need this to pass in the height & width of an image in order to calculate
		// the normalized coordinate values of any bounding boxes. We also need the
		// mimeType to appropriately upload the image.
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

func additionalParamExtraction(methodParams map[string]*anypb.Any) (methodParamsDecoded, error) {
	cameraParam := methodParams["camera_name"]

	if cameraParam == nil {
		return methodParamsDecoded{}, errors.New("must specify a camera_name in the additional_params")
	}

	var cameraName string

	cameraNameWrapper := new(wrapperspb.StringValue)
	if err := cameraParam.UnmarshalTo(cameraNameWrapper); err != nil {
		return methodParamsDecoded{}, err
	}
	cameraName = cameraNameWrapper.Value

	mimeTypeParam := methodParams["mime_type"]

	if mimeTypeParam == nil {
		return methodParamsDecoded{}, errors.New("must specify a mime_type in the additional_params")
	}

	var mimeType string

	mimeTypeWrapper := new(wrapperspb.StringValue)
	if err := mimeTypeParam.UnmarshalTo(mimeTypeWrapper); err != nil {
		return methodParamsDecoded{}, err
	}

	mimeType = mimeTypeWrapper.Value

	minConfidenceParam := methodParams["min_confidence_score"]

	// Default min_confidence_score is 0.5
	minConfidenceScore := 0.5

	if minConfidenceParam != nil {
		minConfidenceScoreWrapper := new(wrapperspb.DoubleValue)
		if err := minConfidenceParam.UnmarshalTo(minConfidenceScoreWrapper); err != nil {
			return methodParamsDecoded{}, err
		}

		minConfidenceScore = minConfidenceScoreWrapper.Value

		if minConfidenceScore < 0 || minConfidenceScore > 1 {
			return methodParamsDecoded{}, errors.New("min_confidence_score must be between 0 and 1 inclusive")
		}
	}

	return methodParamsDecoded{
		cameraName:    cameraName,
		mimeType:      mimeType,
		minConfidence: minConfidenceScore,
	}, nil
}

func assertVision(resource interface{}) (Service, error) {
	visionService, ok := resource.(Service)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}

	return visionService, nil
}
