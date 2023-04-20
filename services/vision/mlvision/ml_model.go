// Package mlvision uses an underlying model from the ML model service as a vision model,
// and wraps the ML model with the vision service methods.
package mlvision

import (
	"bufio"
	"context"
	"math"
	"os"
	"strings"

	"github.com/montanaflynn/stats"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
)

var model = resource.NewDefaultModel("mlmodel")

const (
	// UInt8 is one of the possible input/output types for tensors.
	UInt8 = "uint8"
	// Float32 is one of the possible input/output types for tensors.
	Float32 = "float32"
)

func init() {
	registry.RegisterService(vision.Subtype, model, registry.Service{
		RobotConstructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			attrs, ok := c.ConvertedAttributes.(*MLModelConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, c.ConvertedAttributes)
			}
			return registerMLModelVisionService(ctx, c.Name, attrs, r, logger)
		},
	})
	config.RegisterServiceAttributeMapConverter(
		vision.Subtype,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf MLModelConfig
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*MLModelConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&MLModelConfig{},
	)
}

// MLModelConfig specifies the parameters needed to turn an ML model into a vision Model.
type MLModelConfig struct {
	ModelName string `json:"mlmodel_name"`
}

func registerMLModelVisionService(
	ctx context.Context,
	name string,
	params *MLModelConfig,
	r robot.Robot,
	logger golog.Logger,
) (vision.Service, error) {
	_, span := trace.StartSpan(ctx, "service::vision::registerMLModelVisionService")
	defer span.End()

	mlm, err := mlmodel.FromRobot(r, params.ModelName)
	if err != nil {
		return nil, err
	}
	classifierFunc, err := attemptToBuildClassifier(mlm)
	if err != nil {
		logger.Infof("%v", errors.Wrapf(err, "was not able to turn ml model %q into a classifier", params.ModelName))
	}
	detectorFunc, err := attemptToBuildDetector(mlm)
	if err != nil {
		logger.Infof("%v", errors.Wrapf(err, "was not able to turn ml model %q into a detector", params.ModelName))
	}
	segmenter3DFunc, err := attemptToBuild3DSegmenter(mlm)
	if err != nil {
		logger.Infof("%v", errors.Wrapf(err, "was not able to turn ml model %q into a 3D segmenter", params.ModelName))
	}
	// Don't return a close function, because you don't want to close the underlying ML service
	return vision.NewService(name, r, nil, classifierFunc, detectorFunc, segmenter3DFunc)
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

// getTensorTypeFromName uses the metadata to find the expected type of the tensor
func getTensorTypeFromName(name string, md mlmodel.MLMetadata) string {
	for _, o := range md.Outputs {
		if strings.Contains(strings.ToLower(o.Name), strings.ToLower(name)) {
			return o.DataType
		}
	}
	return ""
}

// getLabelsFromMetadata returns a slice of strings--the intended labels.
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
				// if the labels come out as one line, try splitting that line by spaces or commas to extract labels
				if len(labels) == 1 {
					labels = strings.Split(labels[0], ",")
				}
				if len(labels) == 1 {
					labels = strings.Split(labels[0], " ")
				}

				return labels
			}
		}
	}
	return nil
}

// getBoxOrderFromMetadata returns a slice of ints--the bounding box
// display order, where 0=xmin, 1=ymin, 2=xmax, 3=ymax.
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

// softmax takes the input slice and applies the softmax function.
func softmax(in []float64) []float64 {
	out := make([]float64, 0, len(in))
	bigSum := 0.0
	for _, x := range in {
		bigSum += math.Exp(x)
	}
	for _, x := range in {
		out = append(out, math.Exp(x)/bigSum)
	}
	return out
}

// checkClassification scores ensures that the input scores (output of classifier)
// will represent confidence values (from 0-1).
func checkClassificationScores(in []float64) []float64 {
	if len(in) > 1 {
		for _, p := range in {
			if p < 0 || p > 1 { // is logit, needs softmax
				confs := softmax(in)
				return confs
			}
		}
		return in // no need to softmax
	}
	// otherwise, this is a binary classifier
	if in[0] < -1 || in[0] > 1 { // needs sigmoid
		out, err := stats.Sigmoid(in)
		if err != nil {
			return in
		}
		return out
	}
	return in // no need to sigmoid
}
