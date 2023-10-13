//go:build !no_media

package mlvision

import (
	"context"
	"image"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"gorgonia.org/tensor"

	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

func attemptToBuildDetector(mlm mlmodel.Service, nameMap *sync.Map) (objectdetection.Detector, error) {
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
				tensor.WithShape(1, resized.Bounds().Dy(), resized.Bounds().Dx(), 3),
				tensor.WithBacking(rimage.ImageToUInt8Buffer(resized)),
			)
		case Float32:
			inMap["image"] = tensor.New(
				tensor.WithShape(1, resized.Bounds().Dy(), resized.Bounds().Dx(), 3),
				tensor.WithBacking(rimage.ImageToFloatBuffer(resized)),
			)
		default:
			return nil, errors.New("invalid input type. try uint8 or float32")
		}
		outMap, err := mlm.Infer(ctx, inMap)
		if err != nil {
			return nil, err
		}

		// use the nameMap to find the tensor names, or guess and cache the names
		locationName, categoryName, scoreName, err := findDetectionTensorNames(outMap, nameMap)
		if err != nil {
			return nil, err
		}
		locations, err := convertToFloat64Slice(outMap[locationName].Data())
		if err != nil {
			return nil, err
		}
		categories, err := convertToFloat64Slice(outMap[categoryName].Data())
		if err != nil {
			return nil, err
		}
		scores, err := convertToFloat64Slice(outMap[scoreName].Data())
		if err != nil {
			return nil, err
		}

		// Now reshape outMap into Detections
		if len(categories) != len(scores) || 4*len(scores) != len(locations) {
			return nil, errors.New("output tensor sizes did not match each other as expected")
		}
		detections := make([]objectdetection.Detection, 0, len(scores))
		detectionBoxesAreProportional := false
		for i := 0; i < len(scores); i++ {
			// heuristic for knowing if bounding box coordinates are abolute pixel locations, or
			// proportional pixel locations. Absolute bounding boxes will not usually be less than a pixel
			// and purely located in the upper left corner.
			if i == 0 && (locations[0]+locations[1]+locations[2]+locations[3] < 4.) {
				detectionBoxesAreProportional = true
			}
			var xmin, ymin, xmax, ymax float64
			if detectionBoxesAreProportional {
				xmin = utils.Clamp(locations[4*i+getIndex(boxOrder, 0)], 0, 1) * float64(origW-1)
				ymin = utils.Clamp(locations[4*i+getIndex(boxOrder, 1)], 0, 1) * float64(origH-1)
				xmax = utils.Clamp(locations[4*i+getIndex(boxOrder, 2)], 0, 1) * float64(origW-1)
				ymax = utils.Clamp(locations[4*i+getIndex(boxOrder, 3)], 0, 1) * float64(origH-1)
			} else {
				xmin = utils.Clamp(locations[4*i+getIndex(boxOrder, 0)], 0, float64(origW-1))
				ymin = utils.Clamp(locations[4*i+getIndex(boxOrder, 1)], 0, float64(origH-1))
				xmax = utils.Clamp(locations[4*i+getIndex(boxOrder, 2)], 0, float64(origW-1))
				ymax = utils.Clamp(locations[4*i+getIndex(boxOrder, 3)], 0, float64(origH-1))
			}
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
// the returned tensor order is location, category, score. It caches results.
func findDetectionTensorNames(outMap ml.Tensors, nameMap *sync.Map) (string, string, string, error) {
	// first try the nameMap
	loc, okLoc := nameMap.Load("location")
	cat, okCat := nameMap.Load("category")
	score, okScores := nameMap.Load("score")
	if okLoc && okCat && okScores { // names are known
		locString, ok := loc.(string)
		if !ok {
			return "", "", "", errors.Errorf("name map was not storing string, but a type %T", loc)
		}
		catString, ok := cat.(string)
		if !ok {
			return "", "", "", errors.Errorf("name map was not storing string, but a type %T", cat)
		}
		scoreString, ok := score.(string)
		if !ok {
			return "", "", "", errors.Errorf("name map was not storing string, but a type %T", score)
		}
		return locString, catString, scoreString, nil
	}
	// next, if nameMap is not set, just see if the outMap has expected names
	_, okLoc = outMap["location"]
	_, okCat = outMap["category"]
	_, okScores = outMap["score"]
	if okLoc && okCat && okScores { // names are as expected
		nameMap.Store("location", "location")
		nameMap.Store("category", "category")
		nameMap.Store("score", "score")
		return "location", "category", "score", nil
	}
	// last, do a hack-y thing to try to guess the tensor names for the detection output tensors
	locationName, categoryName, scoreName, err := guessDetectionTensorNames(outMap)
	if err != nil {
		return "", "", "", err
	}
	nameMap.Store("location", locationName)
	nameMap.Store("category", categoryName)
	nameMap.Store("score", scoreName)
	return locationName, categoryName, scoreName, nil
}

// guessDetectionTensors is a hack-y function meant to find the correct detection tensors if the tensors
// were not given the expected names, or have no metadata. This function should succeed
// for models built with the viam platform.
func guessDetectionTensorNames(outMap ml.Tensors) (string, string, string, error) {
	foundTensor := map[string]bool{}
	mappedNames := map[string]string{}
	outNames := tensorNames(outMap)
	_, okLoc := outMap["location"]
	if okLoc {
		foundTensor["location"] = true
		mappedNames["location"] = "location"
	}
	_, okCat := outMap["category"]
	if okCat {
		foundTensor["category"] = true
		mappedNames["category"] = "category"
	}
	_, okScores := outMap["score"]
	if okScores {
		foundTensor["score"] = true
		mappedNames["score"] = "score"
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
				return "", "", "", err
			}
			val64, err := convertToFloat64Slice(val)
			if err != nil {
				return "", "", "", err
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
				mappedNames["location"] = name
				foundTensor[name] = true
				break
			}
		}
		if _, ok := mappedNames["location"]; !ok {
			return "", "", "", errors.Errorf("could not find an output tensor named 'location' among [%s]", strings.Join(outNames, ", "))
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
					mappedNames["category"] = name
					foundTensor[name] = true
					break
				}
				// check if fully whole number
				var whole tensor.Tensor
				var err error
				if nDetections == 0 {
					whole, err = tensor.Sum(t)
					if err != nil {
						return "", "", "", err
					}
				} else {
					s, err := t.Slice(nil, tensor.S(0, nDetections))
					if err != nil {
						return "", "", "", err
					}
					whole, err = tensor.Sum(s)
					if err != nil {
						return "", "", "", err
					}
				}
				val, err := convertToFloat64Slice(whole.Data())
				if err != nil {
					return "", "", "", err
				}
				if math.Mod(val[0], 1) == 0 {
					mappedNames["category"] = name
					foundTensor[name] = true
					break
				}
			}
		}
		if _, ok := mappedNames["category"]; !ok {
			return "", "", "", errors.Errorf("could not find an output tensor named 'category' among [%s]", strings.Join(outNames, ", "))
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
				mappedNames["score"] = name
				foundTensor[name] = true
				break
			}
		}
		if _, ok := mappedNames["score"]; !ok {
			return "", "", "", errors.Errorf("could not find an output tensor named 'score' among [%s]", strings.Join(outNames, ", "))
		}
	}
	return mappedNames["location"], mappedNames["category"], mappedNames["score"], nil
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
