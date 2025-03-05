package ml

import (
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"gorgonia.org/tensor"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/utils"
)

const (
	detectorLocationName = "location"
	detectorCategoryName = "category"
	detectorScoreName    = "score"
)

// FormatDetectionOutputs formats the output tensors from a model into detections.
func FormatDetectionOutputs(outNameMap *sync.Map, outMap Tensors, origW, origH int,
	boxOrder []int, labels []string,
) ([]data.BoundingBox, error) {
	// use the outNameMap to find the tensor names, or guess and cache the names
	locationName, categoryName, scoreName, err := findDetectionTensorNames(outMap, outNameMap)
	if err != nil {
		return nil, err
	}
	locations, err := ConvertToFloat64Slice(outMap[locationName].Data())
	if err != nil {
		return nil, err
	}
	scores, err := ConvertToFloat64Slice(outMap[scoreName].Data())
	if err != nil {
		return nil, err
	}
	hasCategoryTensor := false
	categories := make([]float64, len(scores)) // default 0 category if no category output
	if categoryName != "" {
		hasCategoryTensor = true
		categories, err = ConvertToFloat64Slice(outMap[categoryName].Data())
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
	detections := make([]data.BoundingBox, 0, len(scores))
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
			xmin = utils.Clamp(locations[4*i+GetIndex(boxOrder, 0)], 0, 1)
			ymin = utils.Clamp(locations[4*i+GetIndex(boxOrder, 1)], 0, 1)
			xmax = utils.Clamp(locations[4*i+GetIndex(boxOrder, 2)], 0, 1)
			ymax = utils.Clamp(locations[4*i+GetIndex(boxOrder, 3)], 0, 1)
		} else {
			xmin = utils.Clamp(locations[4*i+GetIndex(boxOrder, 0)], 0, float64(origW-1)) / float64(origW-1)
			ymin = utils.Clamp(locations[4*i+GetIndex(boxOrder, 1)], 0, float64(origH-1)) / float64(origH-1)
			xmax = utils.Clamp(locations[4*i+GetIndex(boxOrder, 2)], 0, float64(origW-1)) / float64(origW-1)
			ymax = utils.Clamp(locations[4*i+GetIndex(boxOrder, 3)], 0, float64(origH-1)) / float64(origH-1)
		}
		labelNum := int(utils.Clamp(categories[i], 0, math.MaxInt))

		if labels == nil {
			detections = append(detections, data.BoundingBox{
				Confidence:     &scores[i],
				Label:          strconv.Itoa(labelNum),
				XMinNormalized: xmin,
				YMinNormalized: ymin,
				XMaxNormalized: xmax,
				YMaxNormalized: ymax,
			})
		} else {
			if labelNum >= len(labels) {
				return nil, errors.Errorf("cannot access label number %v from label file with %v labels", labelNum, len(labels))
			}
			detections = append(detections, data.BoundingBox{
				Confidence:     &scores[i],
				Label:          labels[labelNum],
				XMinNormalized: xmin,
				YMinNormalized: ymin,
				XMaxNormalized: xmax,
				YMaxNormalized: ymax,
			})
		}
	}
	return detections, nil
}

// findDetectionTensors finds the tensors that are necessary for object detection
// the returned tensor order is location, category, score. It caches results.
// category is optional, and will return "" if not present.
func findDetectionTensorNames(outMap Tensors, nameMap *sync.Map) (string, string, string, error) {
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
func guessDetectionTensorNames(outMap Tensors) (string, string, string, error) {
	foundTensor := map[string]bool{}
	mappedNames := map[string]string{}
	outNames := TensorNames(outMap)
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
			val64, err := ConvertToFloat64Slice(val)
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
				val, err := ConvertToFloat64Slice(whole.Data())
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
