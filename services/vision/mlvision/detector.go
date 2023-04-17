package mlvision

import (
	"bufio"
	"context"
	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
	"image"
	"os"
	"strings"
)

func attemptToBuildDetector(mlm mlmodel.Service) (objectdetection.Detector, error) {
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

	return func(ctx context.Context, img image.Image) ([]objectdetection.Detection, error) {
		origW, origH := img.Bounds().Dx(), img.Bounds().Dy()
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

		locations := outMap["location"].([]float32)
		categories := outMap["location"].([]float32)
		scores := outMap["location"].([]float32)

		// Now reshape outMap into Detections
		// ASSUMING [1 0 3 2] FOR NOW bounding box order
		detections := make([]objectdetection.Detection, 0, len(categories))
		for i := 0; i < len(detections); i++ {
			xmin, xmax, ymin, ymax := utils.Clamp(float64(locations[4*i+1]), 0, 1)*float64(origW),
				utils.Clamp(float64(locations[4*i+0]), 0, 1)*float64(origW),
				utils.Clamp(float64(locations[4*i+3]), 0, 1)*float64(origH),
				utils.Clamp(float64(locations[4*i+2]), 0, 1)*float64(origH)
			rect := image.Rect(int(xmin), int(ymin), int(xmax), int(ymax))

			detections = append(detections, objectdetection.NewDetection(rect, float64(scores[i]), labels[i]))
		}
		return detections, nil
	}, nil

}

// getIndex just returns the index of an int in an array of ints
// Will return -1 if it's not there.
func getIndex(s []int, num int) int {
	for i, v := range s {
		if v == num {
			return i
		}
	}
	return -1
}

func getLabelsFromMetadata(md mlmodel.MLMetadata) ([]string, error) {
	for _, o := range md.Outputs {
		if strings.Contains(o.Name, "category") || strings.Contains(o.Name, "probability") {
			if labelPath, ok := o.Extra["labels"]; ok {
				labels := []string{}
				f, err := os.Open(labelPath.(string)) //nolint:gosec
				if err != nil {
					return nil, err
				}
				defer func() {
					if err := f.Close(); err != nil {
						panic(err)
					}
				}()
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					labels = append(labels, scanner.Text())
				}
				return labels, nil
			}
		}
	}
	return nil, errors.New("could not find labels")
}
