package mlvision

import (
	"context"
	"image"
	"sync"

	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"gorgonia.org/tensor"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/vision/objectdetection"
)

const (
	detectorInputName = "image"
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
	labels := getLabelsFromMetadata(md, params.LabelPath)
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
	if shape := md.Inputs[0].Shape; ml.GetIndex(shape, 3) == 1 {
		channelsFirst = true
		inHeight, inWidth = shape[2], shape[3]
	} else {
		inHeight, inWidth = shape[1], shape[2]
	}
	// creates postprocessor to filter on labels and confidences
	postprocessor := createDetectionFilter(params.DefaultConfidence, params.LabelConfidenceMap)

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
		boundingBoxes, err := ml.FormatDetectionOutputs(outNameMap, outMap, resizeW, resizeH, boxOrder, labels)
		if err != nil {
			return nil, err
		}
		detections := convertBoundingBoxesToDetections(boundingBoxes, origW, origH)
		if postprocessor != nil {
			detections = postprocessor(detections)
		}
		return detections, nil
	}, nil
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

// createDetectionFilter creates a post processor function that filters on the outputs of the model.
func createDetectionFilter(minConf float64, labelMap map[string]float64) objectdetection.Postprocessor {
	if len(labelMap) != 0 {
		return objectdetection.NewLabelConfidenceFilter(labelMap)
	}
	if minConf != 0.0 {
		return objectdetection.NewScoreFilter(minConf)
	}
	return nil
}

func convertBoundingBoxesToDetections(boundingBoxes []data.BoundingBox, origW, origH int) []objectdetection.Detection {
	var detections []objectdetection.Detection
	for _, bbox := range boundingBoxes {
		xmin := bbox.XMinNormalized * float64(origW-1)
		ymin := bbox.YMinNormalized * float64(origH-1)
		xmax := bbox.XMaxNormalized * float64(origW-1)
		ymax := bbox.YMaxNormalized * float64(origH-1)
		rect := image.Rect(int(xmin), int(ymin), int(xmax), int(ymax))
		detections = append(detections, objectdetection.NewDetection(image.Rect(0, 0, origW, origH), rect, *bbox.Confidence, bbox.Label))
	}
	return detections
}
