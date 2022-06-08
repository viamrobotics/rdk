package vision

import (
	"bufio"
	"fmt"
	"image"
	"os"
	"strconv"
	"strings"

	"github.com/edaniels/golog"
	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"go.viam.com/rdk/config"
	inf "go.viam.com/rdk/ml/inference"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

// THINK ON IT. WHEN SHOULD YOU LOAD THE MODEL? CLOSE IT?
// WHAT FUNCTIONS DO WE STILL NEED? WHAT SHOULD BE REMOVED
// REMEMBER THAT YOU INFER/CLOSE/ETC ON THE INTERNAL MODEL

// TfliteDetectorConfig specifies the fields necessary for creating a TFLite detector.
type TfliteDetectorConfig struct {
	//this should come from the attributes part of the detector config
	ModelPath  string  `json:"model_path"`
	NumThreads *int    `json:"num_threads"`
	LabelPath  *string `json:"label_path"`
	ServiceURL *string `json:"service_url"`
}

// NewTfliteDetector creates an RDK detector given a DetectorConfig. In other words, this
// function returns a function from image-->objdet.Detections. It does this by making calls to
// an inference package and wrapping the result
func NewTfliteDetector(cfg *DetectorConfig, logger golog.Logger) (objectdetection.Detector, error) {
	// Read those parameters into a TFLiteDetectorConfig
	var t TfliteDetectorConfig
	tfParams, err := config.TransformAttributeMapToStruct(&t, cfg.Parameters)
	if err != nil {
		return nil, errors.Errorf("error getting parameters from config")
	}
	params, ok := tfParams.(*TfliteDetectorConfig)
	if !ok {
		err := utils.NewUnexpectedTypeError(params, tfParams)
		return nil, errors.Wrapf(err, "register tflite detector %s", cfg.Name)
	} //params is now the TfliteDetectorConfig

	model, err := addTfliteModel(params.ModelPath, params.NumThreads)
	if err != nil {
		return nil, err
	}
	defer model.Close()

	inHeight, inWidth := model.Info.InputHeight, model.Info.InputWidth

	//This function has to be the detector
	return func(img image.Image) ([]objectdetection.Detection, error) {
		resizedImg := resize.Resize(inHeight, inWidth, img, resize.Bilinear) //resize image
		labelMap, err := loadLabels(*params.LabelPath)		//check for labelmap
		if err != nil {
			logger.Info("did not retrieve class labels")
		}
		outTensors, err := tfliteInfer(model, resizedImg)
		if err != nil {
			return nil, err
		}
		detections := unpackTensors(outTensors, model, labelMap, logger)
		return detections, nil
	}, nil
}

// addTfliteModel uses the AddModel function in the inference package to register a tflite model
func addTfliteModel(filepath string , numThreads *int) (inf.TFLiteStruct, error) {
	var model inf.TFLiteStruct

	if numThreads == nil {
		loader, err := inf.NewDefaultTFLiteModelLoader()
		if err != nil {
			return nil, errors.Wrap(err, "could not get loader")
		}
		model, err = loader.Load(filepath)
		if err != nil {
			return nil, errors.Wrap(err, "loader could not load model")
		}
	} else {
		loader, err := inf.NewTFLiteModelLoader(&numThreads)
		if err != nil {
			return nil, errors.Wrap(err, "could not get loader")
		}
		model, err = loader.Load(filepath)
		if err != nil {
			return nil, errors.Wrap(err, "loader could not load model")
		}
	}
	return model, nil
}


// tfliteInfer uses the Infer function in the inf package to return the output tensors from the model
func tfliteInfer(model *inf.TFLiteStruct, image image.Image) (config.AttributeMap, error) {
	//Converts the image to bytes before sending it off
	imgBuff := imageToBuffer(image)
	out, err := model.Infer(imgBuff)	//out is gonna be a config.AttributeMap
	if err != nil {
		return nil, errors.Wrap(err, "couldn't infer from model")
	}
	return out, nil
}

// imageToBuffer reads an image into a byte slice (buffer) the most common sense way.
// Left to right like a book; R, then G, then B. No funny stuff.
func imageToBuffer(img image.Image) []byte {
	output := make([]byte, img.Bounds().Dx()*img.Bounds().Dy()*3)

	for x := 0; x < img.Bounds().Dx(); x++ {
		for y := 0; y < img.Bounds().Dy(); y++ {
			r, g, b, a := img.At(x, y).RGBA()
			rr, gg, bb := uint8(float64(r)*255/float64(a)), uint8(float64(g)*255/float64(a)), uint8(float64(b)*255/float64(a))
			output[(y*img.Bounds().Dx())+x+0] = rr
			output[(y*img.Bounds().Dx())+x+1] = gg
			output[(y*img.Bounds().Dx())+x+2] = bb
		}
	}
	return output
}

func unpackTensors(tensors config.AttributeMap, model *inf.TFLiteStruct, labelMap []string, logger golog.Logger) []objectdetection.Detection {
	
	//Gather slices for the bboxes, scores, and labels, using TensorOrder
	var labels []int
	var bboxes []float64
	var scores []float64
	tensorOrder, err := getTensorOrder(model) 	//location = 0 , category = 1, score = 2 for tensor order
	if err != nil {
		//We couldn't get a tensor order from the metadata... 
		logger.Info("could not find tensor order. Using default order: location, category, score")
		tensorOrder = []int{0,1,2}
	} 
	//Ok but where do we get the actual tensors from? (Assuming config.AttributeMap)--------------------
	//Must  change if we get them from interface
	b := tensors[fmt.Sprint("out", getIndex(tensorOrder, 0))] //the boundingbox tensor from tensors
	bboxes = b.([]float64)
	l := tensors[fmt.Sprint("out", getIndex(tensorOrder, 1))] //the label tensor from tensors
	labels = l.([]int)
	s := tensors[fmt.Sprint("out", getIndex(tensorOrder, 2))] //the score tensor from tensors
	scores = s.([]float64)

	//---------------------------------------------------------------------------------------------

	//Get the bounding box order (try from the metadata. If not, smart-default: xx,yy )
	//xmin=0, xmax=1, ymin=2, ymax=3 for bounding box order
	boxOrder, err := getBboxOrder(model)
	if err != nil {
		logger.Info("assuming bounding box tensor is in the default order: [x x y y]")
		boxOrder = []int{1, 0, 3, 2}
		if bboxes[0] > bboxes[1] { //the first val is bigger than the second
			boxOrder[0] = 1
			boxOrder[1] = 0
		} else {
			boxOrder[1] = 1
			boxOrder[0] = 0
		}
		if bboxes[2] > bboxes[3] { //the first val is bigger than the second
			boxOrder[2] = 3
			boxOrder[3] = 2
		} else {
			boxOrder[2] = 2
			boxOrder[3] = 3
		}
	}

	w,h := model.Info.InputWidth, model.Info.InputHeight

	//Detection gathering
	detections := make([]objectdetection.Detection, len(scores))
	for i := 0; i < len(scores); i++ {
		//Gather box
		xmin, ymin, xmax, ymax := bboxes[4*i+getIndex(boxOrder, 0)]*float64(w), bboxes[4*i+getIndex(boxOrder, 1)]*float64(w),
			bboxes[4*i+getIndex(boxOrder, 2)]*float64(h), bboxes[4*i+getIndex(boxOrder, 3)]*float64(h)
		rect := image.Rect(int(xmin), int(ymin), int(xmax), int(ymax))

		//Gather label
		var label string
		if labelMap == nil {
			label = strconv.Itoa(labels[i])
		} else {
			label = labelMap[labels[i]]
		}

		//Gather score and package it
		d := objectdetection.NewDetection(rect, scores[i], label)
		detections = append(detections, d)
	}
	return detections
}

// loadLabels reads a labelmap.txt file from filename and returns a slice of the labels
// (stolen from https://github.com/mattn/go-tflite)
func loadLabels(filename string) ([]string, error) {
	labels := []string{}
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		labels = append(labels, scanner.Text())
	}
	return labels, nil
}

// getIndex just returns the index of an int in an array of ints
// Will return -1 if it's not there
func getIndex(s []int, num int) int {
	for i, v := range s {
		if v == num {
			return i
		}
	}
	return -1
}

func getStringIndex(s []string, word string) int {
	for i, v := range s {
		if v == word || strings.ToLower(v) == word {
			return i
		}
	}
	return -1
}

// getBboxOrder checks the metadata (from inf package) and looks for the bounding box order
// according to where it should be in the schema.
func getBboxOrder(model *inf.TFLiteStruct) ([]int, error) {
	bboxOrder := make([]int, 4)
	m, err := model.GetMetadata()
	if err != nil {
		return nil, errors.Wrap(err, "could not get metadata")
	}
	//tensorData should be a []TensorMetadataT from the metadata telling me about each tensor in order
	tensorData, ok := m.(ModelMetadataT).SubgraphMetadata[0].OutputTensorMetadata
	if !ok {
		return nil, errors.New("could not find bounding box order from the metadata")
	}

	//Go thru them... if name == location, check content and get to work
	for _, t := range tensorData {
		if strings.ToLower(t.Name) == "location" {
			order := t.Content.ContentProperties.Value
			bboxOrder = order.([]int)
		}
	}

	return bboxOrder, nil
}

func getTensorOrder(model *inf.TFLiteStruct) ([]int, error) {
	tensorOrder := make([]int, 4) //location = 0 , category = 1, score = 2
	m, err := model.GetMetadata()
	if err != nil {
		return nil, errors.Wrap(err, "could not get metadata")
	}
	tensorNames, ok := m.(ModelMetadataT).SubgraphMetadata[0].OutputTensorGroups.TensorNames
	if !ok {
		return nil, errors.New("could not get tensor order from metadata")
	}

	l, c, s := getStringIndex(tensorNames, "location"), getStringIndex(tensorNames, "category"), getStringIndex(tensorNames, "score")
	if l == -1 {
		return nil, errors.New("tried to find 'location' in the metadata and could not")
	}
	if c == -1 {
		return nil, errors.New("tried to find 'category' in the metadata and could not")
	}
	if s == -1 {
		return nil, errors.New("tried to find 'score' in the metadata and could not")
	}
	tensorOrder[0] = l
	tensorOrder[1] = c
	tensorOrder[2] = s

	/*
		This is another way to do it via each specific input. Less robust maybe
		tensorData := m.(ModelMetadataT).SubgraphMetadata[0].OutputTensorMetadata
		//tensorData should be a []TensorMetadataT from the metadata telling me about each tensor in order

		for _, t := range(tensorData){
			switch name := strings.ToLower(t.Name); name {
			case "location":
				tensorOrder = append(tensorOrder, 0)
			case "category":
				tensorOrder = append(tensorOrder, 1)
			case "score":
				tensorOrder = append(tensorOrder, 2)
			default:
				continue
			}
		}
	*/

	return tensorOrder, nil

}
