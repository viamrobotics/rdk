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
	var inHeight, inWidth uint
	inType := md.Inputs[0].DataType
	labels := getLabelsFromMetadata(md)
	if shape := md.Inputs[0].Shape; getIndex(shape, 3) == 1 {
		inHeight, inWidth = uint(shape[2]), uint(shape[3])
	} else {
		inHeight, inWidth = uint(shape[1]), uint(shape[2])
	}

	return func(ctx context.Context, img image.Image) (classification.Classifications, error) {
		resized := resize.Resize(inWidth, inHeight, img, resize.Bilinear)
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
