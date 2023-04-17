package mlvision

import (
	"context"
	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/vision/classification"
	"image"
)

func attemptToBuildClassifier(mlm mlmodel.Service) (classification.Classifier, error) {
	md, err := mlm.Metadata(context.Background())
	if err != nil {
		// If the metadata isn't there
		// Don't actually do this, still try but this is a placeholder
		return nil, err
	}
	var inHeight, inWidth uint
	var inType string
	if shape := md.Inputs[0].Shape; getIndex(shape, 3) == 1 {
		inHeight, inWidth = uint(shape[2]), uint(shape[3])
	} else {
		inHeight, inWidth = uint(shape[1]), uint(shape[2])
	}
	inType = md.Inputs[0].DataType
	labels, err := getLabelsFromMetadata(md)
	if err != nil {
		// Not true, still do something if we can't get labels
		return nil, err
	}

	return func(ctx context.Context, img image.Image) (classification.Classifications, error) {
		resized := resize.Resize(inWidth, inHeight, img, resize.Bilinear)
		inMap := make(map[string]interface{}, 1)
		outMap := make(map[string]interface{}, 5)
		switch inType {
		case "uint8":
			inMap["image"] = rimage.ImageToUInt8Buffer(resized)
			outMap, err = mlm.Infer(ctx, inMap)
		case "float32":
			inMap["image"] = rimage.ImageToFloatBuffer(resized)
			outMap, err = mlm.Infer(ctx, inMap)
		default:
			return nil, errors.New("invalid input type. try uint8 or float32")
		}
		if err != nil {
			return nil, err
		}

		probs := outMap["probability"].([]uint8)

		classifications := make(classification.Classifications, 0, len(probs))
		for i := 0; i < len(probs); i++ {
			classifications = append(classifications, classification.NewClassification(float64(probs[i]), labels[i]))
		}
		return classifications, nil

	}, nil

}
