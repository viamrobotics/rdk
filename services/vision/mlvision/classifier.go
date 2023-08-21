package mlvision

import (
	"context"
	"image"
	"strconv"

	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/vision/classification"
)

func attemptToBuildClassifier(mlm mlmodel.Service) (classification.Classifier, error) {
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
	if shapeLen := len(md.Inputs[0].Shape); shapeLen < 4 {
		return nil, errors.Errorf("invalid length of shape array (expected 4, got %d)", shapeLen)
	}
	if shape := md.Inputs[0].Shape; getIndex(shape, 3) == 1 {
		inHeight, inWidth = shape[2], shape[3]
	} else {
		inHeight, inWidth = shape[1], shape[2]
	}

	return func(ctx context.Context, img image.Image) (classification.Classifications, error) {
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

		probs, err := unpack(outMap, "probability")
		if err != nil || len(probs) == 0 {
			probs, err2 = unpack(outMap, DefaultOutTensorName+"0")
			if err2 != nil {
				return nil, multierr.Combine(err, err2)
			}
		}

		confs := checkClassificationScores(probs)
		if labels != nil && len(labels) != len(confs) {
			return nil, errors.New("length of output expected to be length of label list (but is not)")
		}
		classifications := make(classification.Classifications, 0, len(confs))
		for i := 0; i < len(confs); i++ {
			if labels != nil {
				classifications = append(classifications, classification.NewClassification(confs[i], labels[i]))
			} else {
				classifications = append(classifications, classification.NewClassification(confs[i], strconv.Itoa(i)))
			}
		}
		return classifications, nil
	}, nil
}

// In the case that the model provided is not a classifier, attemptToBuildClassifier will return a
// classifier function that function fails because the expected keys are not in the outputTensor.
// use checkIfClassifierWorks to get sample output tensors on gray image so we know if the functions
// returned from attemptToBuildClassifier will fail ahead of time.
func checkIfClassifierWorks(ctx context.Context, cf classification.Classifier) error {
	if cf == nil {
		return errors.New("nil classifier function")
	}

	// test image to check if the classifier function works
	img := image.NewGray(image.Rectangle{Min: image.Point{0, 0}, Max: image.Point{5, 5}})

	_, err := cf(ctx, img)
	if err != nil {
		return errors.New("Cannot use model as a classifier")
	}
	return nil
}
