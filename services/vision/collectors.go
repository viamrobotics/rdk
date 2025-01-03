package vision

import (
	"context"
	"time"

	"github.com/pkg/errors"
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

type methodParamsDecoded struct {
	cameraName    string
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
	minConfidenceScore := decodedParams.minConfidence

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (data.CaptureResult, error) {
		timeRequested := time.Now()
		var res data.CaptureResult
		visCaptureOptions := viscapture.CaptureOptions{
			ReturnImage:           true,
			ReturnDetections:      true,
			ReturnClassifications: true,
		}
		visCapture, err := vision.CaptureAllFromCamera(ctx, cameraName, visCaptureOptions, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a service. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, captureAllFromCamera.String(), err)
		}

		if visCapture.Image == nil {
			return res, errors.New("vision service didn't return an image")
		}

		protoImage, err := imageToProto(ctx, visCapture.Image, cameraName)
		if err != nil {
			return res, err
		}

		var width, height int
		if visCapture.Image != nil {
			width = visCapture.Image.Bounds().Dx()
			height = visCapture.Image.Bounds().Dy()
		}

		filteredBoundingBoxes := []data.BoundingBox{}
		for _, d := range visCapture.Detections {
			if d.Score() >= minConfidenceScore {
				filteredBoundingBoxes = append(filteredBoundingBoxes, toDataBoundingBox(d, width, height))
			}
		}

		filteredClassifications := []data.Classification{}
		for _, c := range visCapture.Classifications {
			if score := c.Score(); score >= minConfidenceScore {
				filteredClassifications = append(filteredClassifications, toDataClassification(c))
			}
		}

		ts := data.Timestamps{
			TimeRequested: timeRequested,
			TimeReceived:  time.Now(),
		}
		return data.NewBinaryCaptureResult(ts, []data.Binary{{
			Payload:  protoImage.Image,
			MimeType: data.CameraFormatToMimeType(protoImage.Format),
			Annotations: data.Annotations{
				BoundingBoxes:   filteredBoundingBoxes,
				Classifications: filteredClassifications,
			},
		}}), nil
	})

	return data.NewCollector(cFunc, params)
}

func toDataClassification(c classification.Classification) data.Classification {
	confidence := c.Score()
	return data.Classification{Label: c.Label(), Confidence: &confidence}
}

func toDataBoundingBox(d objectdetection.Detection, width, height int) data.BoundingBox {
	confidence := d.Score()
	bbox := d.BoundingBox()
	return data.BoundingBox{
		Label:          d.Label(),
		Confidence:     &confidence,
		XMinNormalized: float64(bbox.Min.X) / float64(width),
		XMaxNormalized: float64(bbox.Max.X) / float64(width),
		YMinNormalized: float64(bbox.Min.Y) / float64(height),
		YMaxNormalized: float64(bbox.Max.Y) / float64(height),
	}
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
