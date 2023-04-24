// Package mlvision uses an underlying model from the ML model service as a vision model,
// and wraps the ML model with the vision service methods.
package mlvision

import (
	"bufio"
	"context"
	"math"
	"os"
	"strings"

	"github.com/edaniels/golog"
	"github.com/montanaflynn/stats"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
)

var model = resource.DefaultModelFamily.WithModel("mlmodel")

const (
	// UInt8 is one of the possible input/output types for tensors.
	UInt8 = "uint8"
	// Float32 is one of the possible input/output types for tensors.
	Float32 = "float32"
	// DefaultOutTensorName is the prefix key given to output tensors in the map
	// if there is no metadata. (output0, output1, etc.)
	DefaultOutTensorName = "output"
)

func init() {
	resource.RegisterService(vision.API, model, resource.Registration[vision.Service, *MLModelConfig]{
		DeprecatedRobotConstructor: func(ctx context.Context, r any, c resource.Config, logger golog.Logger) (vision.Service, error) {
			attrs, err := resource.NativeConfig[*MLModelConfig](c)
			if err != nil {
				return nil, err
			}
			actualR, err := utils.AssertType[robot.Robot](r)
			if err != nil {
				return nil, err
			}
			return registerMLModelVisionService(ctx, c.ResourceName().Name, attrs, actualR, logger)
		},
	})
}

// MLModelConfig specifies the parameters needed to turn an ML model into a vision Model.
type MLModelConfig struct {
	resource.TriviallyValidateConfig
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
		logger.Infow("was not able to turn ml model into classifier", "ml model", params.ModelName, "error", err)
	} else {
		classifierFunc, err = checkIfClassifierWorks(ctx, classifierFunc)
		if err != nil {
			logger.Infow("was not able to turn ml model into classifier", "ml model", params.ModelName, "error", err)
		} else {
			logger.Infof("model %q fulfills a vision service ciassifier", params.ModelName)
		}
	}

	detectorFunc, err := attemptToBuildDetector(mlm)
	if err != nil {
		logger.Infow("was not able to turn ml model into detector", "ml model", params.ModelName, "error", err)
	} else {
		detectorFunc, err = checkIfDetectorWorks(ctx, detectorFunc)
		if err != nil {
			logger.Infow("was not able to turn ml model into detector", "ml model", params.ModelName, "error", err)
		} else {
			logger.Infof("model %q fulfills a vision service detector", params.ModelName)
		}
	}

	segmenter3DFunc, err := attemptToBuild3DSegmenter(mlm)
	if err != nil {
		logger.Infow("error turning turn ml model into a 3D segmenter:", "model", params.ModelName, "error", err)
	} else {
		logger.Infof("model %q fulfills a vision service 3D segmenter", params.ModelName)
	}
	// Don't return a close function, because you don't want to close the underlying ML service
	return vision.NewService(name, r, nil, classifierFunc, detectorFunc, segmenter3DFunc)
}

// Unpack output based on expected type and force it into a []float64.
func unpack(inMap map[string]interface{}, name string) ([]float64, error) {
	var out []float64
	me := inMap[name]
	if me == nil {
		return nil, errors.Errorf("no such tensor named %q to unpack", name)
	}
	switch v := me.(type) {
	case []uint8:
		for _, t := range v {
			out = append(out, float64(t))
		}
	case []float32:
		for _, t := range v {
			out = append(out, float64(t))
		}
	}
	return out, nil
}

// getLabelsFromMetadata returns a slice of strings--the intended labels.
func getLabelsFromMetadata(md mlmodel.MLMetadata) []string {
	for _, o := range md.Outputs {
		if !strings.Contains(o.Name, "category") && !strings.Contains(o.Name, "probability") {
			continue
		}

		if labelPath, ok := o.Extra["labels"]; ok {
			var labels []string
			f, err := os.Open(labelPath.(string))
			if err != nil {
				return nil
			}
			defer func() {
				if err := f.Close(); err != nil {
					return
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
