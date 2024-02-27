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

const (
	detectorLocationName = "location"
	detectorCategoryName = "category"
	detectorScoreName    = "score"
	detectorInputName    = "image"
)

func attemptToBuildDetector(mlm mlmodel.Service,
	inNameMap, outNameMap *sync.Map,
	params *MLModelConfig,
) (objectdetection.Detector, error) {
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
	var boxOrder []int
	if len(params.BoxOrder) == 4 {
		boxOrder = params.BoxOrder
	} else {
		boxOrder, err = getBoxOrderFromMetadata(md)
		if err != nil || len(boxOrder) < 4 {
			boxOrder = []int{1, 0, 3, 2}
		}
	}

	if shapeLen := len(md.Inputs[0].Shape); shapeLen < 4 {
		return nil, errors.Errorf("invalid length of shape array (expected 4, got %d)", shapeLen)
	}

	channelsFirst := false // if channelFirst is true, then shape is (1, 3, height, width)
	if shape := md.Inputs[0].Shape; getIndex(shape, 3) == 1 {
		channelsFirst = true
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
		inputName := detectorInputName
		if mapName, ok := inNameMap.Load(inputName); ok {
			if name, ok := mapName.(string); ok {
				inputName = name
			}
		}
		inMap := ml.Tensors{}
		switch inType {
		case UInt8:
			inMap[inputName] = tensor.New(
				tensor.WithShape(1, resized.Bounds().Dy(), resized.Bounds().Dx(), 3),
				tensor.WithBacking(rimage.ImageToUInt8Buffer(resized, params.IsBGR)),
			)
		case Float32:
			inMap[inputName] = tensor.New(
				tensor.WithShape(1, resized.Bounds().Dy(), resized.Bounds().Dx(), 3),
				tensor.WithBacking(rimage.ImageToFloatBuffer(resized, params.IsBGR, params.MeanValue, params.StdDev)),
			)
		default:
			return nil, errors.Errorf("invalid input type of %s. try uint8 or float32", inType)
		}
		if channelsFirst {
			err := inMap[inputName].T(0, 3, 1, 2)
			if err != nil {
				return nil, errors.New("could not transponse tensor of input image")
			}
			err = inMap[inputName].Transpose()
			if err != nil {
				return nil, errors.New("could not transponse the data of the tensor of input image")
			}
		}
		outMap, err := mlm.Infer(ctx, inMap)
		if err != nil {
			return nil, err
		}

		// use the outNameMap to find the tensor names, or guess and cache the names
		locationName, categoryName, scoreName, err := findDetectionTensorNames(outMap, outNameMap)
		if err != nil {
			return nil, err
		}
		locations, err := convertToFloat64Slice(outMap[locationName].Data())
		if err != nil {
			return nil, err
		}
		scores, err := convertToFloat64Slice(outMap[scoreName].Data())
		if err != nil {
			return nil, err
		}
		hasCategoryTensor := false
		categories := make([]float64, len(scores)) // default 0 category if no category output
		if categoryName != "" {
			hasCategoryTensor = true
			categories, err = convertToFloat64Slice(outMap[categoryName].Data())
			if err != nil {
				return nil, err
			}
		}
		// sometimes categories are stuffed into the score output. separate them out.
		if !hasCategoryTensor {
			shape := outMap[scoreName].Shape()
			if len(shape) == 3 { // cartegories are stored in 3rd dimension
				nCategories := shape[2]              // nCategories usually in 3rd dim, but sometimes in 2nd
				if 4*nCategories == len(locations) { // it's actually in 2nd dim
					nCategories = shape[1]
				}
				scores, categories, err = extractCategoriesFromScores(scores, nCategories)
				if err != nil {
					return nil, errors.Wrap(err, "could not extract categories from score tensor")
				}
			}
		}

		// Now reshape outMap into Detections
		if len(categories) != len(scores) || 4*len(scores) != len(locations) {
			return nil, errors.Errorf(
				"output tensor sizes did not match each other as expected. score: %v, category: %v, location: %v",
				len(scores),
				len(categories),
				len(locations),
			)
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

			if labels == nil {
				detections = append(detections, objectdetection.NewDetection(rect, scores[i], strconv.Itoa(labelNum)))
			} else {
				if labelNum >= len(labels) {
					return nil, errors.Errorf("cannot access label number %v from label file with %v labels", labelNum, len(labels))
				}
				detections = append(detections, objectdetection.NewDetection(rect, scores[i], labels[labelNum]))
			}
		}
		return detections, nil
	}, nil
}

func extractCategoriesFromScores(scores []float64, nCategories int) ([]float64, []float64, error) {
	if nCategories == 1 { // trivially every category has the same label
		categories := make([]float64, len(scores))
		return scores, categories, nil
	}
	// ensure even division of data into categories
	if len(scores)%nCategories != 0 {
		return nil, nil, errors.Errorf("nCategories %v does not divide evenly into score tensor of length %v", nCategories, len(scores))
	}
	nEntries := len(scores) / nCategories
	newCategories := make([]float64, 0, nEntries)
	newScores := make([]float64, 0, nEntries)
	for i := 0; i < nEntries; i++ {
		argMax, floatMax, err := argMaxAndMax(scores[nCategories*i : nCategories*i+nCategories])
		if err != nil {
			return nil, nil, err
		}
		newCategories = append(newCategories, float64(argMax))
		newScores = append(newScores, floatMax)
	}
	return newScores, newCategories, nil
}

func argMaxAndMax(slice []float64) (int, float64, error) {
	if len(slice) == 0 {
		return 0, 0.0, errors.New("slice cannot be nil or empty")
	}
	argMax := 0
	floatMax := -math.MaxFloat64
	for i, v := range slice {
		if v > floatMax {
			floatMax = v
			argMax = i
		}
	}
	return argMax, floatMax, nil
}

// findDetectionTensors finds the tensors that are necessary for object detection
// the returned tensor order is location, category, score. It caches results.
// category is optional, and will return "" if not present.
func findDetectionTensorNames(outMap ml.Tensors, nameMap *sync.Map) (string, string, string, error) {
	// first try the nameMap
	loc, okLoc := nameMap.Load(detectorLocationName)
	score, okScores := nameMap.Load(detectorScoreName)
	cat, okCat := nameMap.Load(detectorCategoryName)
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
	if okLoc && okScores { // names are known, just no categories
		locString, ok := loc.(string)
		if !ok {
			return "", "", "", errors.Errorf("name map was not storing string, but a type %T", loc)
		}
		scoreString, ok := score.(string)
		if !ok {
			return "", "", "", errors.Errorf("name map was not storing string, but a type %T", score)
		}
		if len(outMap[scoreString].Shape()) == 3 || len(outMap) == 2 { // the categories are in the score
			return locString, "", scoreString, nil
		}
	}
	// next, if nameMap is not set, just see if the outMap has expected names
	// if the outMap only has two outputs, it might just be locations and scores.
	_, okLoc = outMap[detectorLocationName]
	_, okCat = outMap[detectorCategoryName]
	_, okScores = outMap[detectorScoreName]
	if okLoc && okCat && okScores { // names are as expected
		nameMap.Store(detectorLocationName, detectorLocationName)
		nameMap.Store(detectorCategoryName, detectorCategoryName)
		nameMap.Store(detectorScoreName, detectorScoreName)
		return detectorLocationName, detectorCategoryName, detectorScoreName, nil
	}
	// last, do a hack-y thing to try to guess the tensor names for the detection output tensors
	locationName, categoryName, scoreName, err := guessDetectionTensorNames(outMap)
	if err != nil {
		return "", "", "", err
	}
	nameMap.Store(detectorLocationName, locationName)
	nameMap.Store(detectorCategoryName, categoryName)
	nameMap.Store(detectorScoreName, scoreName)
	return locationName, categoryName, scoreName, nil
}

// guessDetectionTensors is a hack-y function meant to find the correct detection tensors if the tensors
// were not given the expected names, or have no metadata. This function should succeed
// for models built with the viam platform.
func guessDetectionTensorNames(outMap ml.Tensors) (string, string, string, error) {
	foundTensor := map[string]bool{}
	mappedNames := map[string]string{}
	outNames := tensorNames(outMap)
	_, okLoc := outMap[detectorLocationName]
	if okLoc {
		foundTensor[detectorLocationName] = true
		mappedNames[detectorLocationName] = detectorLocationName
	}
	_, okCat := outMap[detectorCategoryName]
	if okCat {
		foundTensor[detectorCategoryName] = true
		mappedNames[detectorCategoryName] = detectorCategoryName
	}
	_, okScores := outMap[detectorScoreName]
	if okScores {
		foundTensor[detectorScoreName] = true
		mappedNames[detectorScoreName] = detectorScoreName
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
				mappedNames[detectorLocationName] = name
				foundTensor[name] = true
				break
			}
		}
		if _, ok := mappedNames[detectorLocationName]; !ok {
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
					mappedNames[detectorCategoryName] = name
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
					mappedNames[detectorCategoryName] = name
					foundTensor[name] = true
					break
				}
			}
		}
		if _, ok := mappedNames[detectorCategoryName]; !ok {
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
				mappedNames[detectorScoreName] = name
				foundTensor[name] = true
				break
			}
		}
		if _, ok := mappedNames[detectorScoreName]; !ok {
			return "", "", "", errors.Errorf("could not find an output tensor named 'score' among [%s]", strings.Join(outNames, ", "))
		}
	}
	return mappedNames[detectorLocationName], mappedNames[detectorCategoryName], mappedNames[detectorScoreName], nil
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
		return errors.Wrap(err, "cannot use model as a detector")
	}
	return nil
}
