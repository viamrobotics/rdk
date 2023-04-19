package mlvision

import (
	"bufio"
	"context"
	"image"
	"os"
	"strconv"
	"strings"

	"github.com/nfnt/resize"
	"github.com/pkg/errors"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

func attemptToBuildDetector(mlm mlmodel.Service) (objectdetection.Detector, error) {
	md, err := mlm.Metadata(context.Background())
	if err != nil {
		return nil, errors.Wrapf(err, "could not find metadata")
	}

	// Set up input type, height, width, and labels
	var inHeight, inWidth uint
	inType := md.Inputs[0].DataType
	labels, err := getLabelsFromMetadata(md)
	if err != nil {
		labels = nil
	}
	boxOrder, err := getBoxOrderFromMetadata(md)
	if err != nil || len(boxOrder) < 4 {
		boxOrder = []uint32{1, 0, 3, 2}
	}

	if shape := md.Inputs[0].Shape; getIndex(shape, 3) == 1 {
		inHeight, inWidth = uint(shape[2]), uint(shape[3])
	} else {
		inHeight, inWidth = uint(shape[1]), uint(shape[2])
	}

	return func(ctx context.Context, img image.Image) ([]objectdetection.Detection, error) {
		origW, origH := img.Bounds().Dx(), img.Bounds().Dy()
		resized := resize.Resize(inWidth, inHeight, img, resize.Bilinear)
		inMap := make(map[string]interface{})
		outMap := make(map[string]interface{})
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

		locations := unpackMe(outMap, "location", md)
		categories := unpackMe(outMap, "category", md)
		scores := unpackMe(outMap, "score", md)

		// Now reshape outMap into Detections
		detections := make([]objectdetection.Detection, 0, len(categories))
		for i := 0; i < len(scores); i++ {
			xmin, xmax, ymin, ymax := utils.Clamp(locations[4*i+int(boxOrder[0])], 0, 1)*float64(origW),
				utils.Clamp(locations[4*i+int(boxOrder[1])], 0, 1)*float64(origW),
				utils.Clamp(locations[4*i+int(boxOrder[2])], 0, 1)*float64(origH),
				utils.Clamp(locations[4*i+int(boxOrder[3])], 0, 1)*float64(origH)
			rect := image.Rect(int(xmin), int(ymin), int(xmax), int(ymax))
			labelNum := int(categories[i])

			if labels != nil {
				detections = append(detections, objectdetection.NewDetection(rect, scores[i], labels[labelNum]))
			} else {
				detections = append(detections, objectdetection.NewDetection(rect, scores[i], strconv.Itoa(labelNum)))
			}
		}
		return detections, nil
	}, nil
}

// Unpack output based on expected type and force it into a []float64.
func unpackMe(inMap map[string]interface{}, name string, md mlmodel.MLMetadata) []float64 {
	var out []float64
	me := inMap[name]
	switch getTensorTypeFromName(name, md) {
	case "uint8":
		temp := me.([]uint8)
		for _, t := range temp {
			out = append(out, float64(t))
		}
	case "float32":
		temp := me.([]float32)
		for _, p := range temp {
			out = append(out, float64(p))
		}
	default:
		return nil
	}
	return out
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
				f, err := os.Open(labelPath.(string))
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

func getBoxOrderFromMetadata(md mlmodel.MLMetadata) ([]uint32, error) {
	for _, o := range md.Outputs {
		if strings.Contains(o.Name, "location") {
			if order, ok := o.Extra["boxOrder"]; ok {
				return order.([]uint32), nil
			}
		}
	}
	return nil, errors.New("could not grab bbox order")
}

func getTensorTypeFromName(name string, md mlmodel.MLMetadata) string {
	for _, o := range md.Outputs {
		if strings.Contains(strings.ToLower(o.Name), strings.ToLower(name)) {
			return o.DataType
		}
	}
	return ""
}
