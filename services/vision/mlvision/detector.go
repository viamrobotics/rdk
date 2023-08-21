package mlvision

import (
	"context"
	"image"
	"math"
	"strconv"
	"strings"

	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"gorgonia.org/tensor"

	"go.viam.com/rdk/ml"
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
		inMap := ml.Tensors{}
		switch inType {
		case UInt8:
			inMap["image"] = tensor.New(
				tensor.WithShape(1, int(inHeight), int(inWidth), 3),
				tensor.WithBacking(rimage.ImageToUInt8Buffer(resized)),
			)
		case Float32:
			inMap["image"] = tensor.New(
				tensor.WithShape(1, int(inHeight), int(inWidth), 3),
				tensor.WithBacking(rimage.ImageToFloatBuffer(resized)),
			)
		default:
			return nil, errors.New("invalid input type. try uint8 or float32")
		}
		outMap, _, err := mlm.Infer(ctx, inMap, nil)
		if err != nil {
			return nil, err
		}

		locationsT, categoriesT, scoresT, err := findDetectionTensors(outMap)
		if err != nil {
			return nil, err
		}
		locations, err := convertToFloat64Slice(locationsT.Data())
		if err != nil {
			return nil, err
		}
		categories, err := convertToFloat64Slice(categoriesT.Data())
		if err != nil {
			return nil, err
		}
		scores, err := convertToFloat64Slice(scoresT.Data())
		if err != nil {
			return nil, err
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

// findDetectionTensors finds the tensors that are necessary for object detection
// the returned tensor order is location, category, score.
func findDetectionTensors(outMap ml.Tensors) (*tensor.Dense, *tensor.Dense, *tensor.Dense, error) {
	locations, okLoc := outMap["location"]
	categories, okCat := outMap["category"]
	scores, okScores := outMap["score"]
	if okLoc && okCat && okScores { // names are as expected
		return locations, categories, scores, nil
	}
	return guessDetectionTensors(outMap)
}

// guessDetectionTensors is a hack-y function meant to find the correct detection tensors if the tensors
// were not given the expected names, or have no metadata. This function should succeed
// for models built with the viam platform.
func guessDetectionTensors(outMap ml.Tensors) (*tensor.Dense, *tensor.Dense, *tensor.Dense, error) {
	foundTensor := map[string]bool{}
	outNames := tensorNames(outMap)
	locations, okLoc := outMap["location"]
	if okLoc {
		foundTensor["location"] = true
	}
	categories, okCat := outMap["category"]
	if okCat {
		foundTensor["category"] = true
	}
	scores, okScores := outMap["score"]
	if okScores {
		foundTensor["score"] = true
	}
	// first find how many detections there were
	// this will be used to find the other tensors
	nDetections := 0
	for name, t := range outMap {
		if _, alreadyFound := foundTensor[name]; alreadyFound {
			continue
		}
		if t.Dims() == 1 { // usually n-detections has its own tensor
			val, err := t.At(0)
			if err != nil {
				return nil, nil, nil, err
			}
			val64, err := convertToFloat64Slice(val)
			if err != nil {
				return nil, nil, nil, err
			}
			nDetections = int(val64[0])
			foundTensor[name] = true
			break
		}
	}
	if !okLoc { // guess the name of the location tensor
		// location tensor should have 3 dimensions usually
		for name, t := range outMap {
			if _, alreadyFound := foundTensor[name]; alreadyFound {
				continue
			}
			if t.Dims() == 3 {
				locations = t
				foundTensor[name] = true
				break
			}
		}
		if locations == nil {
			return nil, nil, nil, errors.Errorf("could not find an output tensor named 'location' among [%s]", strings.Join(outNames, ", "))
		}
	}
	if !okCat { // guess the name of the category tensor
		// a category usually has a whole number in its elements, so either look for
		// int data types in the tensor, or sum the elements and make sure they dont have any decimals
		for name, t := range outMap {
			if _, alreadyFound := foundTensor[name]; alreadyFound {
				continue
			}
			dt := t.Dtype()
			if t.Dims() == 2 {
				if dt == tensor.Int || dt == tensor.Int32 || dt == tensor.Int64 ||
					dt == tensor.Uint32 || dt == tensor.Uint64 || dt == tensor.Int8 || dt == tensor.Uint8 {
					categories = t
					foundTensor[name] = true
					break
				}
				// check if fully whole number
				var whole tensor.Tensor
				var err error
				if nDetections == 0 {
					whole, err = tensor.Sum(t)
					if err != nil {
						return nil, nil, nil, err
					}
				} else {
					s, err := t.Slice(nil, tensor.S(0, nDetections))
					if err != nil {
						return nil, nil, nil, err
					}
					whole, err = tensor.Sum(s)
					if err != nil {
						return nil, nil, nil, err
					}
				}
				val, err := convertToFloat64Slice(whole.Data())
				if err != nil {
					return nil, nil, nil, err
				}
				if math.Mod(val[0], 1) == 0 {
					categories = t
					foundTensor[name] = true
					break
				}
			}
		}
		if categories == nil {
			return nil, nil, nil, errors.Errorf("could not find an output tensor named 'category' among [%s]", strings.Join(outNames, ", "))
		}
	}
	if !okScores { // guess the name of the scores tensor
		// a score usually has a float data type
		for name, t := range outMap {
			if _, alreadyFound := foundTensor[name]; alreadyFound {
				continue
			}
			dt := t.Dtype()
			if t.Dims() == 2 && (dt == tensor.Float32 || dt == tensor.Float64) {
				scores = t
				foundTensor[name] = true
				break
			}
		}
		if scores == nil {
			return nil, nil, nil, errors.Errorf("could not find an output tensor named 'score' among [%s]", strings.Join(outNames, ", "))
		}
	}
	return locations, categories, scores, nil
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
		return errors.Wrap(err, "Cannot use model as a detector")
	}
	return nil
}
