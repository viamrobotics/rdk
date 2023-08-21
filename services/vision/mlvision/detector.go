package mlvision

import (
	"context"
	"image"
	"math"
	"strconv"

	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

func attemptToBuildDetector(mlm mlmodel.Service) (objectdetection.Detector, error) {
	md, err := mlm.Metadata(context.Background())
	if err != nil {
		return nil, errors.New("could not get any metadata")
	}

	// Set up input type, height, width, and labels
	var inHeight, inWidth int
	if len(md.Inputs) < 1 {
		return nil, errors.New("no input tensors received")
	}
	inType := md.Inputs[0].DataType
	labels := getLabelsFromMetadata(md)
	boxOrder, err := getBoxOrderFromMetadata(md)
	if err != nil || len(boxOrder) < 4 {
		boxOrder = []int{1, 0, 3, 2}
	}

	if shapeLen := len(md.Inputs[0].Shape); shapeLen < 4 {
		return nil, errors.Errorf("invalid length of shape array (expected 4, got %d)", shapeLen)
	}

	if shape := md.Inputs[0].Shape; getIndex(shape, 3) == 1 {
		inHeight, inWidth = shape[2], shape[3]
	} else {
		inHeight, inWidth = shape[1], shape[2]
	}

	return func(ctx context.Context, img image.Image) ([]objectdetection.Detection, error) {
		origW, origH := img.Bounds().Dx(), img.Bounds().Dy()
		resizeW := inWidth
		if resizeW == -1 {
			resizeW = origW
		}
		resizeH := inHeight
		if resizeH == -1 {
			resizeH = origH
		}
		resized := img
		if (origW != resizeW) || (origH != resizeH) {
			resized = resize.Resize(uint(resizeW), uint(resizeH), img, resize.Bilinear)
		}
		inMap := make(map[string]interface{})
		switch inType {
		case UInt8:
			inMap["image"] = rimage.ImageToUInt8Buffer(resized)
		case Float32:
			inMap["image"] = rimage.ImageToFloatBuffer(resized)
		default:
			return nil, errors.New("invalid input type. try uint8 or float32")
		}
		outMap, err := mlm.Infer(ctx, inMap)
		if err != nil {
			return nil, err
		}

		var err2 error

		locations, err := unpack(outMap, "location")
		if err != nil || len(locations) == 0 {
			locations, err2 = unpack(outMap, DefaultOutTensorName+"0")
			if err2 != nil {
				return nil, multierr.Combine(err, err2)
			}
		}
		categories, err := unpack(outMap, "category")
		if err != nil || len(categories) == 0 {
			categories, err2 = unpack(outMap, DefaultOutTensorName+"1")
			if err2 != nil {
				return nil, multierr.Combine(err, err2)
			}
		}
		scores, err := unpack(outMap, "score")
		if err != nil || len(scores) == 0 {
			scores, err2 = unpack(outMap, DefaultOutTensorName+"2")
			if err2 != nil {
				return nil, multierr.Combine(err, err2)
			}
		}

		// Now reshape outMap into Detections
		if len(categories) != len(scores) || 4*len(scores) != len(locations) {
			return nil, errors.New("output tensor sizes did not match each other as expected")
		}
		detections := make([]objectdetection.Detection, 0, len(scores))
		for i := 0; i < len(scores); i++ {
			xmin, ymin, xmax, ymax := utils.Clamp(locations[4*i+getIndex(boxOrder, 0)], 0, 1)*float64(origW),
				utils.Clamp(locations[4*i+getIndex(boxOrder, 1)], 0, 1)*float64(origH),
				utils.Clamp(locations[4*i+getIndex(boxOrder, 2)], 0, 1)*float64(origW),
				utils.Clamp(locations[4*i+getIndex(boxOrder, 3)], 0, 1)*float64(origH)
			rect := image.Rect(int(xmin), int(ymin), int(xmax), int(ymax))
			labelNum := int(utils.Clamp(categories[i], 0, math.MaxInt))
			if labels != nil {
				detections = append(detections, objectdetection.NewDetection(rect, scores[i], labels[labelNum]))
			} else {
				detections = append(detections, objectdetection.NewDetection(rect, scores[i], strconv.Itoa(labelNum)))
			}
		}
		return detections, nil
	}, nil
}

// In the case that the model provided is not a detector, attemptToBuildDetector will return a
// detector function that function fails because the expected keys are not in the outputTensor.
// use checkIfDetectorWorks to get sample output tensors on gray image so we know if the functions
// returned from attemptToBuildDetector will fail ahead of time.
func checkIfDetectorWorks(ctx context.Context, df objectdetection.Detector) error {
	if df == nil {
		return errors.New("nil detector function")
	}

	// test image to check if the detector function works
	img := image.NewGray(image.Rectangle{Min: image.Point{0, 0}, Max: image.Point{5, 5}})

	_, err := df(ctx, img)
	if err != nil {
		return errors.New("Cannot use model as a detector")
	}
	return nil
}
