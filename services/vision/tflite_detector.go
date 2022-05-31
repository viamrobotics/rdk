package vision

import (
	"bufio"
	"image"
	"os"
	"strconv"

	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"go.viam.com/rdk/config"
	inf "go.viam.com/rdk/ml/inference"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

// TfliteDetectorConfig specifies the fields necessary for creating a TFLite detector.
type TfliteDetectorConfig struct {
	//this should come from the attributes part of the detector config
	ModelPath  string  `json:"model_path"`
	NumThreads *int    `json:"num_threads"`
	LabelPath  *string `json:"label_path"`
	ServiceURL *string `json:"service_url"`
}

// TfliteModelInfo has everything we expect back from inf.GetModelInfo()
type TfliteModelInfo struct {
	InputSize []int
	InputType string
	//BboxOrder *[]int
}


// NewTfliteDetector creates an RDK detector given a DetectorConfig. In other words, this
// function returns a function from image-->objdet.Detections. It does this by making calls to
// an inference package and wrapping the result
func NewTfliteDetector(cfg *DetectorConfig) (objectdetection.Detector, error) {
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


	//pretend this part worked and returns a client for the standalone ML inference service
	err = addTfliteModel(params.ModelPath, cfg.Name, cfg.Type, *params.NumThreads)
	if err != nil {
		return nil, err
	}
	modelInfo, err := getTfliteModelInfo(cfg.Name)
	if err != nil {
		return nil, err
	}
	
	//This function has to be the detector
	return func(img image.Image) ([]objectdetection.Detection, error) {

		//resize it... call infer... shape result into detection
		resizedImg := resize.Resize(modelInfo.InputSize[0], modelInfo.InputSize[1], img, resize.Bilinear)
		infResult, err := tfliteInfer(cfg.Name, resizedImg)
		if err != nil {
			return nil, err
		}
		labelMap, _ := loadLabels(*params.LabelPath) //should check this error with a logger saying no labels

		detections := unpackTensors(infResult, cfg.Name, modelInfo.InputSize[0], modelInfo.InputSize[1], labelMap)
		return detections, nil
	}, nil
}

//TODO:KHARI Read from the filepath given and give inf.AddModel a []bytes
// addTfliteModel uses the AddModel function in the inference package to register a tflite model
func addTfliteModel(file, modelName, modelType string, numThreads int) error {

	err := inf.AddModel(file, modelName, modelType, numThreads)
	if err != nil {
		return errors.Wrap(err, "couldn't add model")
	}
	return nil
}

// getTfliteModel uses the GetModelInfo function in the inf package to return model information
func getTfliteModelInfo(modelName string) (*TfliteModelInfo, error) {

	info, err := inf.GetModelInfo(modelName)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get your model info")
	}
	return info, nil
}

// tfliteInfer uses the Infer function in the inf package to return the output tensors from the model
func tfliteInfer(modelName string, image image.Image) (config.AttributeMap, error) {
	//Definitely convert the image to bytes before sending it off
	imgBuff := imageToBuffer(image)
	out, err := inf.Infer(modelName, imgBuff)
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

// unpackTensors takes the output tensors from the model and shapes them into RDK detections
// Which tensor is which gets determined via the length and type of the output tensor
func unpackTensors(T config.AttributeMap, name string, w, h int, labelMap []string) []objectdetection.Detection {
	// This might be a weird way to do it but.... lol here we go
	l1 := T.IntSlice("out1")
	l2 := T.IntSlice("out2")
	l3 := T.IntSlice("out3")

	var labels []int
	var bboxes []float64
	var scores []float64

	switch {
	case len(l1) > 0: //l1 has the labels
		labels = l1
	case len(l2) > 0: //l2 has the labels
		labels = l2
	case len(l3) > 0: //l3 has the labels
		labels = l3
	default: //def: l2 has the labels
		labels = l2
	}

	b1 := T.Float64Slice("out1")
	b2 := T.Float64Slice("out2")
	b3 := T.Float64Slice("out3")

	//check the cases below... bigger one is bboxes, smaller is score
	switch {
	case (len(b1) > len(b2) && len(b1) > len(b3)): //b1 is the bboxes
		bboxes = b1
		if len(b2) > 0 {
			scores = b2
		} else {
			scores = b3
		}
	case (len(b2) > len(b1) && len(b2) > len(b3)): //b2 is the bboxes
		bboxes = b2
		if len(b1) > 0 {
			scores = b1
		} else {
			scores = b3
		}
	case (len(b3) > len(b1) && len(b3) > len(b2)): //b3 is the bboxes
		bboxes = b3
		if len(b1) > 0 {
			scores = b1
		} else {
			scores = b2
		}
	default: //def: b1 is the bboxes
		bboxes = b1
		if len(b2) > 0 {
			scores = b2
		} else {
			scores = b3
		}
	} //Once that's done we have all of them (bboxes, labels, scores)

	//Now, check if we have action in the BboxOrder... if not, set to default
	boxOrder, err := getBboxOrder(name)
	if boxOrder == nil || err != nil {
		boxOrder = []int{1, 0, 3, 2}
	}

	//Detection gathering
	detections := make([]objectdetection.Detection, len(scores))
	for i := 0; i < len(scores); i++ {
		//Gather box
		xmin, ymin, xmax, ymax := bboxes[4*i+getIndex(boxOrder, 0)]*float64(w), bboxes[4*i+getIndex(boxOrder, 1)]*float64(h),
			bboxes[4*i+getIndex(boxOrder, 2)]*float64(w), bboxes[4*i+getIndex(boxOrder, 3)]*float64(h)
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

// getBboxOrder checks the metadata (from inf package) and looks for the bounding box order
// according to where it should be in the schema.
func getBboxOrder(name string) ([]int, error) {
	//The default order is [0, 1, 2]... locations, labels, scores
	m, err := inf.GetMetadata(name) //m should be a config.AttributeMap
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get metadata")
	}
	bboxOrder, ok := m["outputTensorMetadata"]["location"]["content"]["contentProperties"]["index"]
	if !ok {
		return nil, errors.New("couldn't find bounding box order within the metadata")
	}
	return bboxOrder, nil
}
