package mlvision

import (
	"context"
	"image"
	"strconv"
	"strings"
	"sync"

	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"gorgonia.org/tensor"

	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/vision/classification"
)

const (
	classifierProbabilityName = "probability"
	classifierInputName       = "image"
)

func attemptToBuildClassifier(mlm mlmodel.Service,
	inNameMap, outNameMap *sync.Map,
	params *MLModelConfig,
) (classification.Classifier, error) {
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
	channelsFirst := false // if channelFirst is true, then shape is (1, 3, height, width)
	if shape := md.Inputs[0].Shape; getIndex(shape, 3) == 1 {
		channelsFirst = true
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
		inputName := classifierInputName
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

		// check if output tensor name that classifier is looking for is already present
		// in the nameMap. If not, find the probability name, and cache it in the nameMap
		pName, ok := outNameMap.Load(classifierProbabilityName)
		if !ok {
			_, ok := outMap[classifierProbabilityName]
			if !ok {
				if len(outMap) == 1 {
					for name := range outMap { //  only 1 element in map, assume its probabilities
						outNameMap.Store(classifierProbabilityName, name)
						pName = name
					}
				}
			} else {
				outNameMap.Store(classifierProbabilityName, classifierProbabilityName)
				pName = classifierProbabilityName
			}
		}
		probabilityName, ok := pName.(string)
		if !ok {
			return nil, errors.Errorf("name map did not store a string of the tensor name, but an object of type %T instead", pName)
		}
		data, ok := outMap[probabilityName]
		if !ok {
			return nil, errors.Errorf("no tensor named 'probability' among output tensors [%s]", strings.Join(tensorNames(outMap), ", "))
		}
		probs, err := convertToFloat64Slice(data.Data())
		if err != nil {
			return nil, err
		}
		confs := checkClassificationScores(probs)
		if labels != nil && len(labels) != len(confs) {
			return nil, errors.New("length of output expected to be length of label list (but is not)")
		}
		classifications := make(classification.Classifications, 0, len(confs))
		for i := 0; i < len(confs); i++ {
			if labels == nil {
				classifications = append(classifications, classification.NewClassification(confs[i], strconv.Itoa(i)))
			} else {
				if i >= len(labels) {
					return nil, errors.Errorf("cannot access label number %v from label file with %v labels", i, len(labels))
				}
				classifications = append(classifications, classification.NewClassification(confs[i], labels[i]))
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
		return errors.Wrap(err, "cannot use model as a classifier")
	}
	return nil
}
