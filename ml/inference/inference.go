// package inference contains functions to access tflite
package inference

// #cgo LDFLAGS: -L/Users/alexiswei/Documents/repos/tensorflow/bazel-bin/tensorflow/lite/c
// #cgo CFLAGS: -I/Users/alexiswei/Documents/repos/tensorflow/
import "C"
import (
	"fmt"

	"go.viam.com/rdk/config"

	tflite "github.com/mattn/go-tflite"
	"github.com/pkg/errors"
)

var (
	validTypes     = [...]string{"tflite"}
	existingModels = map[string]modelStruct{}
)

type MLModel interface {
}

type modelStruct struct {
	modelType        string
	typeSpecificInfo interface{}
}

type tfliteStruct struct {
	model              *tflite.Model
	interpreter        *tflite.Interpreter
	interpreterOptions *tflite.InterpreterOptions
	info               TFLiteInfo
}

// LoadModel prepares an existing local model for inference
func LoadModel(modelPath string, modelType string, moreInfo config.AttributeMap) error {
	if isValidType(modelType) == false {
		return errors.New("only model type tflite is supported")
	}

	model, ok := existingModels[modelPath]
	if ok {
		if model.typeSpecificInfo != nil {
			return nil
		}
	}

	var modelInfo interface{}

	switch modelType {
	case "tflite":
		tfliteModel, err := loadTFLiteModel(modelPath, moreInfo)
		if err != nil {
			return err
		}
		modelInfo = tfliteModel
	default:
		return errors.New("model not supported")
	}

	existingModels[modelPath] = modelStruct{
		modelType:        modelType,
		typeSpecificInfo: modelInfo,
	}
	return nil
}

// CloseModel removes existing usable model and interpreters based on path
func CloseModel(modelPath string) error {
	model, ok := existingModels[modelPath]
	if !ok {
		return nil
	}

	if model.modelType == "tflite" {
		tfliteModelStruct, ok := model.typeSpecificInfo.(tfliteStruct)
		if !ok {
			return errors.New("not tflite model type when expected")
		}
		closeTFLiteModel(tfliteModelStruct)
		model.typeSpecificInfo = nil
	} else {
		return errors.New("no models were closed")
	}

	return nil
}

func GetAvailableModels() []string {
	keys := make([]string, 0, len(existingModels))
	for k := range existingModels {
		keys = append(keys, k)
	}
	fmt.Println(keys)
	return keys
}

// GetModelInfo returns
func GetModelInfo(modelPath string) (interface{}, error) {
	// 2. Create a model/interpreter based on this name
	model, ok := existingModels[modelPath]
	if !ok {
		return nil, nil
	}

	if model.modelType == "tflite" {
		tfliteModel, ok := model.typeSpecificInfo.(tfliteStruct)
		if !ok {
			return nil, nil
		}
		modelInfo, err := getTfliteModelInfo(tfliteModel)
		if err != nil {
			return nil, err
		}
		return modelInfo, nil

	}
	return nil, nil
}

// Infer takes a modelPath and the inputTensor and performs an inference, returning output tensors
func Infer(modelPath string, input interface{}) (config.AttributeMap, error) {
	model, ok := existingModels[modelPath]
	if !ok {
		return nil, errors.New("model does not exist")
	}

	var out config.AttributeMap
	var err error
	if model.modelType == "tflite" {
		tfliteModel, ok := model.typeSpecificInfo.(tfliteStruct)
		if !ok {
			return nil, errors.New("not tflite struct type")
		}
		out, err = tfliteInfer(tfliteModel, input)
		if err != nil {
			return nil, errors.Wrap(err, "failed to infer")
		}
	}

	return out, nil
}

// GetMetadata will return a metadata struct
func GetMetadata(modelPath string) (interface{}, error) {
	model := existingModels[modelPath]
	var metaStruct interface{}
	if model.modelType == "tflite" {
		b, err := getTFLiteMetadataBytes(modelPath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse tflite as bytes")
		}
		metaStruct = getTFLiteMetadataAsStruct(b)
	} else {
		return nil, errors.New("cannot get metadata")
	}
	return metaStruct, nil

}

// internal helper functions

// isValidType checks if the input is a type of model that the inference library supports
func isValidType(modelType string) bool {
	for _, val := range validTypes {
		if val == modelType {
			return true
		}
	}
	return false
}
