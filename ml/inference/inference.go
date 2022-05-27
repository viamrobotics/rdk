// package inference contains functions to access tflite
package inference

// #cgo LDFLAGS: -L/Users/alexiswei/Documents/repos/tensorflow/bazel-bin/tensorflow/lite/c
// #cgo CFLAGS: -I/Users/alexiswei/Documents/repos/tensorflow/
import "C"
import (
	"fmt"
	"log"
	"os"

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
	file              *os.File
	modelType         string
	modelSpecificInfo interface{}
}

type tfliteModelStruct struct {
	tfliteModel       *tflite.Model
	tfliteInterpreter *tflite.Interpreter
	tfliteOptions     *tflite.InterpreterOptions
}

// AddModel adds a new model to our storage of models
func AddModel(file []byte, modelName string, modelType string, moreInfo config.AttributeMap) error {
	if isValidType(modelType) == false {
		return errors.New("only model type tflite is supported")
	}

	if _, ok := existingModels[modelName]; ok {
		return errors.New("this model name already exists, use new unique name")
	}

	fileName := modelName + "." + modelType

	tempFile, err := os.CreateTemp("viam/inference/", fileName)
	if err != nil {
		log.Fatal(err)
	}

	var modelInfo interface{}

	switch modelName {
	case "tflite":
		modelInfo, err = CreateTFLiteModel(file, moreInfo)
		if err != nil {
			return err
		}
	default:
		return errors.New("model not supported")
	}

	existingModels[modelName] = modelStruct{
		file:              tempFile,
		modelType:         modelType,
		modelSpecificInfo: modelInfo,
	}
	return nil
}

func CreateTFLiteModel(model []byte, moreInfo config.AttributeMap) (interface{}, error) {
	tFLiteModel := tflite.NewModel(model)

	// parse attribute map for additional info
	threads := moreInfo["numThreads"]
	intThreads, isTrue := threads.(int)
	if isTrue != false {
		return nil, errors.New("number of threads not an interfer")
	}

	options, err := loadTFLiteInterpreterOptions(intThreads)
	if err != nil {
		return nil, errors.New("failed to get interpreter options")
	}

	interpreter := tflite.NewInterpreter(tFLiteModel, options)
	if interpreter == nil {
		return nil, errors.New("cannot create interpreter")
	}

	modelInfo := tfliteModelStruct{
		tfliteModel:       tFLiteModel,
		tfliteInterpreter: interpreter,
		tfliteOptions:     options,
	}

	return modelInfo, nil
}

func loadTFLiteInterpreterOptions(numThreads int) (*tflite.InterpreterOptions, error) {
	options := tflite.NewInterpreterOptions()
	if options == nil {
		return nil, errors.New("interpreter options failed to be created")
	}

	options.SetNumThread(numThreads)

	options.SetErrorReporter(func(msg string, userData interface{}) {
		log.Println(msg)
	}, nil)

	return options, nil
}

func DeleteModel(modelName string) error {
	fileName := modelName + "." + existingModels[modelName].modelType
	tempDir := os.TempDir()
	err := os.Remove(tempDir + "/" + fileName)
	if err != nil {
		return err
	}
	delete(existingModels, modelName)
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

func GetModelInfo(modelName string) interface{} {
	// 2. Create a model/interpreter based on this name
	return nil
}

func Infer(name string, input []byte) config.AttributeMap {
	return nil
}

// GetMetadata
func GetMetadata(name string) interface{} {
	return nil

}

// internal helper functions

// turnToMapInterface takes output tensors and converts it into a config.AttributeMap
func turnToMapInterface(output tflite.Tensor) config.AttributeMap {
	return nil
}

// isValidType checks if the input is a type of model that the inference library supports
func isValidType(modelType string) bool {
	for _, val := range validTypes {
		if val == modelType {
			return true
		}
	}
	return false
}
