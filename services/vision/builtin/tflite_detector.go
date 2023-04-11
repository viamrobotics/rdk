//go:build !arm && !windows

package builtin

import (
	"bufio"
	"context"
	"image"
	"os"
	fp "path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/edaniels/golog"
	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	inf "go.viam.com/rdk/ml/inference"
	"go.viam.com/rdk/ml/inference/tflite_metadata"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

// TFLiteDetectorConfig specifies the fields necessary for creating a TFLite detector.
type TFLiteDetectorConfig struct {
	// this should come from the attributes part of the detector config
	ModelPath  string  `json:"model_path"`
	NumThreads int     `json:"num_threads"`
	LabelPath  *string `json:"label_path"`
	ServiceURL *string `json:"service_url"`
}

// NewTFLiteDetector creates an RDK detector given a DetectorConfig. In other words, this
// function returns a function from image-->[]objectdetection.Detection. It does this by making calls to
// an inference package and wrapping the result.
func NewTFLiteDetector(
	ctx context.Context,
	cfg *vision.VisModelConfig,
	logger golog.Logger,
) (objectdetection.Detector, *inf.TFLiteStruct, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::NewTFLiteDetector")
	defer span.End()

	// Read those parameters into a TFLiteDetectorConfig
	var t TFLiteDetectorConfig
	tfParams, err := config.TransformAttributeMapToStruct(&t, cfg.Parameters)
	if err != nil {
		return nil, nil, errors.New("error getting parameters from config")
	}
	params, ok := tfParams.(*TFLiteDetectorConfig)
	if !ok {
		err := utils.NewUnexpectedTypeError(params, tfParams)
		return nil, nil, errors.Wrapf(err, "register tflite detector %s", cfg.Name)
	}
	// Secret but hard limit on num_threads
	if params.NumThreads > runtime.NumCPU()/4 {
		params.NumThreads = runtime.NumCPU() / 4
	}

	// Add the model
	model, err := addTFLiteModel(ctx, params.ModelPath, &params.NumThreads)
	if err != nil {
		return nil, nil, errors.Wrap(err, "something wrong with adding the model")
	}

	var inHeight, inWidth uint

	if shape := model.Info.InputShape; getIndex(shape, 3) == 1 {
		inHeight, inWidth = uint(shape[2]), uint(shape[3])
	} else {
		inHeight, inWidth = uint(shape[1]), uint(shape[2])
	}

	if params.LabelPath == nil {
		blank := ""
		params.LabelPath = &blank
	}

	labelMap, err := loadLabels(*params.LabelPath)
	if err != nil {
		logger.Warn("did not retrieve class labels")
	}

	// This function to be returned is the detector.
	return func(ctx context.Context, img image.Image) ([]objectdetection.Detection, error) {
		origW, origH := img.Bounds().Dx(), img.Bounds().Dy()
		resizedImg := resize.Resize(inHeight, inWidth, img, resize.Bilinear)
		outTensors, err := tfliteInfer(ctx, model, resizedImg)
		if err != nil {
			return nil, err
		}
		detections := unpackTensors(ctx, outTensors, model, labelMap, logger, origW, origH)
		return detections, nil
	}, model, nil
}

// addTFLiteModel uses the loader (default or otherwise) from the inference package
// to register a tflite model. Default is chosen if there's no numThreads given.
func addTFLiteModel(ctx context.Context, filepath string, numThreads *int) (*inf.TFLiteStruct, error) {
	_, span := trace.StartSpan(ctx, "service::vision::addTFLiteModel")
	defer span.End()
	var model *inf.TFLiteStruct
	var loader *inf.TFLiteModelLoader
	var err error

	if numThreads == nil {
		loader, err = inf.NewDefaultTFLiteModelLoader()
	} else {
		loader, err = inf.NewTFLiteModelLoader(*numThreads)
	}
	if err != nil {
		return nil, errors.Wrap(err, "could not get loader")
	}

	fullpath, err2 := fp.Abs(filepath)
	if err2 != nil {
		model, err = loader.Load(filepath)
	} else {
		model, err = loader.Load(fullpath)
	}

	if err != nil {
		if strings.Contains(err.Error(), "failed to load") {
			if err2 != nil {
				return nil, errors.Wrapf(err, "file not found at %s", filepath)
			}
			return nil, errors.Wrapf(err, "file not found at %s", fullpath)
		}
		return nil, errors.Wrap(err, "loader could not load model")
	}

	return model, nil
}

// tfliteInfer first converts an input image to a buffer using the imageToBuffer func
// and then uses the Infer function form the inference package to return the output tensors from the model.
func tfliteInfer(ctx context.Context, model *inf.TFLiteStruct, image image.Image) ([]interface{}, error) {
	_, span := trace.StartSpan(ctx, "service::vision::tfliteInfer")
	defer span.End()

	// Converts the image to bytes before sending it off
	switch model.Info.InputTensorType {
	case inf.UInt8:
		imgBuff := ImageToUInt8Buffer(image)
		out, err := model.Infer(imgBuff)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't infer from model")
		}
		return out, nil
	case inf.Float32:
		imgBuff := ImageToFloatBuffer(image)
		out, err := model.Infer(imgBuff)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't infer from model")
		}
		return out, nil
	default:
		return nil, errors.New("invalid input type. try uint8 or float32")
	}
}

// ImageToUInt8Buffer reads an image into a byte slice in the most common sense way.
// Left to right like a book; R, then G, then B. No funny stuff. Assumes values should be between 0-255.
func ImageToUInt8Buffer(img image.Image) []byte {
	output := make([]byte, img.Bounds().Dx()*img.Bounds().Dy()*3)
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			r, g, b, a := img.At(x, y).RGBA()
			rr, gg, bb, _ := rgbaTo8Bit(r, g, b, a)
			output[(y*img.Bounds().Dx()+x)*3+0] = rr
			output[(y*img.Bounds().Dx()+x)*3+1] = gg
			output[(y*img.Bounds().Dx()+x)*3+2] = bb
		}
	}
	return output
}

// ImageToFloatBuffer reads an image into a byte slice (buffer) the most common sense way.
// Left to right like a book; R, then G, then B. No funny stuff. Assumes values between -1 and 1.
func ImageToFloatBuffer(img image.Image) []float32 {
	output := make([]float32, img.Bounds().Dx()*img.Bounds().Dy()*3)
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			r, g, b, a := img.At(x, y).RGBA()
			rr, gg, bb := float32(r)/float32(a)*2-1, float32(g)/float32(a)*2-1, float32(b)/float32(a)*2-1
			output[(y*img.Bounds().Dx()+x)*3+0] = rr
			output[(y*img.Bounds().Dx()+x)*3+1] = gg
			output[(y*img.Bounds().Dx()+x)*3+2] = bb
		}
	}
	return output
}

// rgbaTo8Bit converts the uint32s from RGBA() to uint8s.
func rgbaTo8Bit(r, g, b, a uint32) (rr, gg, bb, aa uint8) {
	r >>= 8
	rr = uint8(r)
	g >>= 8
	gg = uint8(g)
	b >>= 8
	bb = uint8(b)
	a >>= 8
	aa = uint8(a)
	return
}

// unpackTensors takes the model's output tensors as input and reshapes them into objdet.Detections.
func unpackTensors(ctx context.Context, tensors []interface{}, model *inf.TFLiteStruct,
	labelMap []string, logger golog.Logger, origW, origH int,
) []objectdetection.Detection {
	_, span := trace.StartSpan(ctx, "service::vision::unpackTensors")
	defer span.End()
	var hasMetadata bool
	var boxOrder []int

	// Read metadata
	m, err := model.Metadata()

	if err != nil {
		hasMetadata = false
	} else {
		hasMetadata = true
	}

	// Figure out bounding box order. If we can't find it, should be empty [] (not nil)
	if hasMetadata {
		boxOrder, err = getBboxOrder(m)
		if err != nil {
			logger.Warn("assuming bounding box tensor is in the default order: [x x y y]")
		}
	}

	var labels []int
	var bboxes []float64
	var scores []float64
	var count int

	// Based on the number of output tensors and their content, make a guess about which output tensors
	// are bounding boxes, which are labels, and which are scores
	if !hasMetadata {
		switch model.Info.OutputTensorCount {
		case 1:
			// There's only one thing so assume it's bounding boxes
			T0, _ := tensors[0].([]float32)
			for _, b := range T0 {
				bboxes = append(bboxes, float64(b))
			}
			count = len(T0) / 4

		case 2:
			// See which is longer --> that's bboxes. Then check for the other's first value
			// to determine whether score/label
			var guessedBboxes []float32
			var guessedScoresLabels []float32
			T0, _ := tensors[0].([]float32)
			T1, _ := tensors[1].([]float32)
			if len(T0) > len(T1) {
				guessedBboxes = T0
				guessedScoresLabels = T1
			} else {
				guessedBboxes = T1
				guessedScoresLabels = T0
			}
			count = len(guessedScoresLabels)
			for _, b := range guessedBboxes {
				bboxes = append(bboxes, float64(b))
			}
			if guessedScoresLabels[0] >= 1 || guessedScoresLabels[0] == 0 {
				for _, l := range guessedScoresLabels {
					labels = append(labels, int(l))
				}
			} else {
				for _, s := range guessedScoresLabels {
					scores = append(scores, float64(s))
				}
			}
		default: // case 3+
			// See which is longer --> that's bboxes. Then check for the other's first value
			// to determine whether score/label. Assign the last one the last remaining thing
			var guessedBboxes []float32
			var guessedScores []float32
			var guessedLabels []float32
			T0, _ := tensors[0].([]float32)
			T1, _ := tensors[1].([]float32)
			T2, _ := tensors[2].([]float32)
			if (len(T0) > len(T1)) && (len(T0) > len(T2)) { // T0 is bboxes
				guessedBboxes = T0
				guessedScores = T1
				if guessedScores[0] >= 1 || guessedScores[0] == 0 {
					guessedLabels = T1
					guessedScores = T2
				}
			}
			if (len(T1) > len(T0)) && (len(T1) > len(T2)) { // T1 is bboxes
				guessedBboxes = T1
				guessedScores = T0
				if guessedScores[0] >= 1 || guessedScores[0] == 0 {
					guessedLabels = T0
					guessedScores = T2
				}
			}
			if (len(T2) > len(T0)) && (len(T2) > len(T1)) { // T2 is bboxes
				guessedBboxes = T2
				guessedScores = T0
				if guessedScores[0] >= 1 || guessedScores[0] == 0 {
					guessedLabels = T0
					guessedScores = T1
				}
			}
			count = len(guessedScores)
			for _, b := range guessedBboxes {
				bboxes = append(bboxes, float64(b))
			}
			for _, l := range guessedLabels {
				labels = append(labels, int(l))
			}
			for _, s := range guessedScores {
				scores = append(scores, float64(s))
			}
		}
	} else { // if we do have metadata, just read the tensor order from there.
		tensorOrder, found := getTensorOrder(m)
		if found[0] {
			for _, b := range tensors[getIndex(tensorOrder, 0)].([]float32) {
				bboxes = append(bboxes, float64(b))
			}
		}
		if found[1] {
			for _, l := range tensors[getIndex(tensorOrder, 1)].([]float32) {
				labels = append(labels, int(l))
			}
		}
		if found[2] {
			for _, s := range tensors[getIndex(tensorOrder, 2)].([]float32) {
				scores = append(scores, float64(s))
			}
		}
		count = len(tensors[getIndex(tensorOrder, 0)].([]float32)) / 4
	}

	// If we don't know the bounding box order, assume the first two are x-values
	// and the last two are y-values
	if len(boxOrder) == 0 {
		logger.Warn("assuming bounding box tensor is in the default order: [x x y y]")
		boxOrder = []int{1, 0, 3, 2}
		if bboxes[0] > bboxes[1] {
			boxOrder[0] = 1
			boxOrder[1] = 0
		} else {
			boxOrder[1] = 1
			boxOrder[0] = 0
		}
		if bboxes[2] > bboxes[3] {
			boxOrder[2] = 3
			boxOrder[3] = 2
		} else {
			boxOrder[2] = 2
			boxOrder[3] = 3
		}
	}

	// Gather detections
	detections := make([]objectdetection.Detection, count)
	for i := 0; i < count; i++ {
		// Gather box
		xmin, ymin, xmax, ymax := utils.Clamp(bboxes[4*i+getIndex(boxOrder, 0)], 0.0, 1.0)*float64(origW),
			utils.Clamp(bboxes[4*i+getIndex(boxOrder, 1)], 0.0, 1.0)*float64(origH),
			utils.Clamp(bboxes[4*i+getIndex(boxOrder, 2)], 0.0, 1.0)*float64(origW),
			utils.Clamp(bboxes[4*i+getIndex(boxOrder, 3)], 0.0, 1.0)*float64(origH)
		rect := image.Rect(int(xmin), int(ymin), int(xmax), int(ymax))

		var label string
		score := 1.0
		if len(labels) > 0 {
			if labelMap != nil {
				if labels[i] < len(labelMap) && labels[i] >= 0 {
					label = labelMap[labels[i]]
				}
			} else {
				label = strconv.Itoa(labels[i])
			}
		} // else label = ""

		if len(scores) > 0 {
			score = scores[i]
		} // else score = 1

		// Add detection
		d := objectdetection.NewDetection(rect, score, label)
		detections[i] = d
	}
	return detections
}

// loadLabels reads a labelmap.txt file from filename and returns a slice of the labels
// (stolen from https:// github.com/mattn/go-tflite).
func loadLabels(filename string) ([]string, error) {
	if filename == "" {
		return nil, errors.New("no labelpath")
	}
	labels := []string{}
	f, err := os.Open(filename) //nolint:gosec
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

	// if the labels come out as one line, try splitting that line by spaces or commas to extract labels
	if len(labels) == 1 {
		labels = strings.Split(labels[0], ",")
	}

	if len(labels) == 1 {
		labels = strings.Split(labels[0], " ")
	}

	return labels, nil
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

// getBboxOrder checks the metadata and looks for the bounding box order
// returned as []int, where 0=xmin, 1=xmax, 2=ymin, 3=ymax.
func getBboxOrder(m *tflite_metadata.ModelMetadataT) ([]int, error) {
	// tensorData should be a []TensorMetadataT from the metadata telling me about each tensor in order
	tensorData := m.SubgraphMetadata[0].OutputTensorMetadata
	for _, t := range tensorData {
		if !strings.HasPrefix(t.Name, "location") {
			continue
		}

		bboxOrder := make([]int, 4)
		order := t.Content.ContentProperties.Value.(*tflite_metadata.BoundingBoxPropertiesT).Index
		for i, o := range order {
			bboxOrder[i] = int(o)
		}
		return bboxOrder, nil
	}

	return nil, errors.New("cannot find location in getBboxOrder")
}

// getTensorOrder checks the metadata for the order of the output tensors
// returned as []int where 0=bounding box location, 1=class/category/label, 2= confidence score.
func getTensorOrder(m *tflite_metadata.ModelMetadataT) ([]int, []bool) {
	tensorOrder := make([]int, 3) // location = 0 , category = 1, score = 2

	tensorData := m.SubgraphMetadata[0].OutputTensorMetadata

	found := make([]bool, 3)

	for i, t := range tensorData {
		switch name := strings.ToLower(t.Name); name {
		case "location", "locations":
			tensorOrder[i] = 0
			found[0] = true
		case "category", "class", "classes":
			tensorOrder[i] = 1
			found[1] = true
		case "score", "scores":
			tensorOrder[i] = 2
			found[2] = true
		default:
			continue
		}
	}

	return tensorOrder, found
}
