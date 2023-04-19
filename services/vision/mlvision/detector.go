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

const (
	// UInt8 is one of the possible input/output types for tensors.
	UInt8 = "uint8"
	// Float32 is one of the possible input/output types for tensors.
	Float32 = "float32"
)

func attemptToBuildDetector(mlm mlmodel.Service) (objectdetection.Detector, error) {
	md, err := mlm.Metadata(context.Background())
	if err != nil {
		return nil, errors.Wrapf(err, "could not find metadata")
	}

	// Set up input type, height, width, and labels
	var inHeight, inWidth uint
	inType := md.Inputs[0].DataType
	labels := getLabelsFromMetadata(md)
	boxOrder, err := getBoxOrderFromMetadata(md)
	if err != nil || len(boxOrder) < 4 {
		boxOrder = []int{1, 0, 3, 2}
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

		locations := unpack(outMap, "location", md)
		categories := unpack(outMap, "category", md)
		scores := unpack(outMap, "score", md)

		// Now reshape outMap into Detections
		detections := make([]objectdetection.Detection, 0, len(categories))
		for i := 0; i < len(scores); i++ {
			xmin, ymin, xmax, ymax := utils.Clamp(locations[4*i+getIndex(boxOrder, 0)], 0, 1)*float64(origW),
				utils.Clamp(locations[4*i+getIndex(boxOrder, 1)], 0, 1)*float64(origH),
				utils.Clamp(locations[4*i+getIndex(boxOrder, 2)], 0, 1)*float64(origW),
				utils.Clamp(locations[4*i+getIndex(boxOrder, 3)], 0, 1)*float64(origH)
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
func unpack(inMap map[string]interface{}, name string, md mlmodel.MLMetadata) []float64 {
	var out []float64
	me := inMap[name]
	switch getTensorTypeFromName(name, md) {
	case UInt8:
		temp := me.([]uint8)
		for _, t := range temp {
			out = append(out, float64(t))
		}
	case Float32:
		temp := me.([]float32)
		for _, p := range temp {
			out = append(out, float64(p))
		}
	default:
		return nil
	}
	return out
}

func getTensorTypeFromName(name string, md mlmodel.MLMetadata) string {
	for _, o := range md.Outputs {
		if strings.Contains(strings.ToLower(o.Name), strings.ToLower(name)) {
			return o.DataType
		}
	}
	return ""
}

// getLabelsFromMetadata returns a slice of strings--the intended labels
func getLabelsFromMetadata(md mlmodel.MLMetadata) []string {
	for _, o := range md.Outputs {
		if strings.Contains(o.Name, "category") || strings.Contains(o.Name, "probability") {
			if labelPath, ok := o.Extra["labels"]; ok {
				labels := []string{}
				f, err := os.Open(labelPath.(string))
				if err != nil {
					return nil
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
				return labels
			}
		}
	}
	return nil
}

// getBoxOrderFromMetadata returns a slice of ints--the bounding box
// display order, where 0=xmin, 1=ymin, 2=xmax, 3=ymax
func getBoxOrderFromMetadata(md mlmodel.MLMetadata) ([]int, error) {
	for _, o := range md.Outputs {
		if strings.Contains(o.Name, "location") {
			out := make([]int, 0, 4)
			if order, ok := o.Extra["boxOrder"].([]uint32); ok {
				for _, o := range order {
					out = append(out, int(o))
				}
				return out, nil
			}
		}
	}
	return nil, errors.New("could not grab bbox order")
}

// getIndex returns the index of an int in an array of ints
// Will return -1 if it's not there.
func getIndex(s []int, num int) int {
	for i, v := range s {
		if v == num {
			return i
		}
	}
	return -1
}
