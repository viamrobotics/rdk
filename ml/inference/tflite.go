//go:build !arm && !windows

package inference

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"

	tflite "github.com/mattn/go-tflite"
	"github.com/pkg/errors"
	"gorgonia.org/tensor"

	"go.viam.com/rdk/ml"
	tfliteSchema "go.viam.com/rdk/ml/inference/tflite"
	metadata "go.viam.com/rdk/ml/inference/tflite_metadata"
)

const tfLiteMetadataName string = "TFLITE_METADATA"

// TFLiteStruct holds information, model and interpreter of a tflite model in go.
type TFLiteStruct struct {
	model              *tflite.Model
	interpreter        Interpreter
	interpreterOptions *tflite.InterpreterOptions
	Info               *TFLiteInfo
	modelPath          string
	mu                 sync.Mutex
}

// Interpreter interface holds methods used by a tflite interpreter.
type Interpreter interface {
	AllocateTensors() tflite.Status
	Invoke() tflite.Status
	GetOutputTensorCount() int
	GetInputTensorCount() int
	GetInputTensor(i int) *tflite.Tensor
	GetOutputTensor(i int) *tflite.Tensor
	Delete()
}

// TFLiteModelLoader holds functions that sets up a tflite model to be used.
type TFLiteModelLoader struct {
	newModelFromFile   func(path string) *tflite.Model
	newInterpreter     func(model *tflite.Model, options *tflite.InterpreterOptions) (Interpreter, error)
	interpreterOptions *tflite.InterpreterOptions
	getInfo            func(inter Interpreter) *TFLiteInfo
}

// NewDefaultTFLiteModelLoader returns the default loader when using tflite.
func NewDefaultTFLiteModelLoader() (*TFLiteModelLoader, error) {
	options, err := createTFLiteInterpreterOptions(runtime.NumCPU())
	if err != nil {
		return nil, err
	}

	loader := &TFLiteModelLoader{
		newModelFromFile:   tflite.NewModelFromFile,
		newInterpreter:     getInterpreter,
		interpreterOptions: options,
		getInfo:            getInfo,
	}

	return loader, nil
}

// NewTFLiteModelLoader returns a loader that allows you to set threads when using tflite.
func NewTFLiteModelLoader(numThreads int) (*TFLiteModelLoader, error) {
	if numThreads <= 0 {
		return nil, errors.New("numThreads must be a positive integer")
	}

	options, err := createTFLiteInterpreterOptions(numThreads)
	if err != nil {
		return nil, err
	}

	loader := &TFLiteModelLoader{
		newModelFromFile:   tflite.NewModelFromFile,
		newInterpreter:     getInterpreter,
		interpreterOptions: options,
		getInfo:            getInfo,
	}

	return loader, nil
}

// createTFLiteInterpreterOptions returns tflite interpreterOptions with settings.
func createTFLiteInterpreterOptions(numThreads int) (*tflite.InterpreterOptions, error) {
	options := tflite.NewInterpreterOptions()
	if options == nil {
		return nil, FailedToLoadError("interpreter options")
	}

	options.SetNumThread(numThreads)

	options.SetErrorReporter(func(msg string, userData interface{}) {
		log.Println(msg)
	}, nil)

	return options, nil
}

// Load returns a TFLite struct that is ready to be used for inferences.
func (loader TFLiteModelLoader) Load(modelPath string) (*TFLiteStruct, error) {
	tfLiteModel := loader.newModelFromFile(modelPath)
	if tfLiteModel == nil {
		return nil, FailedToLoadError("model")
	}

	interpreter, err := loader.newInterpreter(tfLiteModel, loader.interpreterOptions)
	if err != nil {
		return nil, err
	}

	status := interpreter.AllocateTensors()
	if status != tflite.OK {
		return nil, errors.New("failed to allocate tensors")
	}

	info := loader.getInfo(interpreter)

	modelStruct := &TFLiteStruct{
		model:              tfLiteModel,
		interpreter:        interpreter,
		interpreterOptions: loader.interpreterOptions,
		Info:               info,
		modelPath:          modelPath,
	}

	return modelStruct, nil
}

// InTensorType is a wrapper around a string that details the allowed input tensor types.
type InTensorType string

// UInt8 and Float32 are the currently supported input tensor types.
const (
	UInt8   = InTensorType("UInt8")
	Float32 = InTensorType("Float32")
)

// TFLiteInfo holds information about a model that are useful for creating input tensors bytes.
type TFLiteInfo struct {
	InputHeight       int
	InputWidth        int
	InputChannels     int
	InputShape        []int
	InputTensorType   InTensorType
	InputTensorCount  int
	OutputTensorCount int
	OutputTensorTypes []string
}

// getInfo provides some input and output tensor information based on a tflite interpreter.
func getInfo(inter Interpreter) *TFLiteInfo {
	input := inter.GetInputTensor(0)

	numOut := inter.GetOutputTensorCount()
	var outTypes []string
	for i := 0; i < numOut; i++ {
		outTypes = append(outTypes, inter.GetOutputTensor(i).Type().String())
	}

	info := &TFLiteInfo{
		InputHeight:       input.Dim(1),
		InputWidth:        input.Dim(2),
		InputChannels:     input.Dim(3),
		InputShape:        input.Shape(),
		InputTensorType:   InTensorType(input.Type().String()),
		InputTensorCount:  inter.GetInputTensorCount(),
		OutputTensorCount: numOut,
		OutputTensorTypes: outTypes,
	}
	return info
}

// Infer takes an input map of tensors and returns an output map of tensors.
func (model *TFLiteStruct) Infer(inputTensors ml.Tensors) (ml.Tensors, error) {
	model.mu.Lock()
	defer model.mu.Unlock()

	interpreter := model.interpreter
	inputCount := interpreter.GetInputTensorCount()
	if inputCount == 1 && len(inputTensors) == 1 { // convenience function for underspecified names
		input := interpreter.GetInputTensor(0)
		for _, inpTensor := range inputTensors { // there is only one element in this map
			status := input.CopyFromBuffer(inpTensor.Data())
			if status != tflite.OK {
				return nil, errors.Errorf("copying from tensor buffer %q failed", input.Name())
			}
		}
	} else {
		for i := 0; i < inputCount; i++ {
			input := interpreter.GetInputTensor(i)
			inpTensor, ok := inputTensors[input.Name()]
			if !ok {
				return nil, errors.Errorf("tflite model expected a tensor named %q, but no such input tensor found", input.Name())
			}
			if inpTensor == nil {
				continue
			}
			status := input.CopyFromBuffer(inpTensor.Data())
			if status != tflite.OK {
				return nil, errors.Errorf("copying from tensor buffer named %q failed", input.Name())
			}
		}
	}

	status := interpreter.Invoke()
	if status != tflite.OK {
		return nil, errors.New("tflite invoke failed")
	}

	output := ml.Tensors{}
	for i := 0; i < interpreter.GetOutputTensorCount(); i++ {
		t := interpreter.GetOutputTensor(i)
		if t == nil {
			continue
		}
		tType := TFliteTensorToGorgoniaTensor(t.Type())
		outputTensor := tensor.New(
			tensor.WithShape(t.Shape()...),
			tensor.Of(tType),
			tensor.FromMemory(uintptr(t.Data()), uintptr(t.ByteSize())),
		)
		outName := fmt.Sprintf("%s:%v", t.Name(), i)
		output[outName] = outputTensor
	}
	return output, nil
}

// TFliteTensorToGorgoniaTensor converts the constants from one tensor library to another.
func TFliteTensorToGorgoniaTensor(t tflite.TensorType) tensor.Dtype {
	switch t {
	case tflite.NoType:
		return tensor.Uintptr // just return is as a general pointer type...
	case tflite.Float32:
		return tensor.Float32
	case tflite.Int32:
		return tensor.Int32
	case tflite.UInt8:
		return tensor.Uint8
	case tflite.Int64:
		return tensor.Int64
	case tflite.String:
		return tensor.String
	case tflite.Bool:
		return tensor.Bool
	case tflite.Int16:
		return tensor.Int16
	case tflite.Complex64:
		return tensor.Complex64
	case tflite.Int8:
		return tensor.Int8
	default: // shouldn't reach here unless tflite adds more types
		return tensor.Uintptr
	}
}

// Metadata provides the metadata information based on the model flatbuffer file.
func (model *TFLiteStruct) Metadata() (*metadata.ModelMetadataT, error) {
	b, err := getTFLiteMetadataBytes(model.modelPath)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, MetadataDoesNotExistError()
	}
	return getTFLiteMetadataAsStruct(b), nil
}

// getTFLiteMetadataBytes takes a model path of a tflite file and extracts the metadata buffer from the entire model.
func getTFLiteMetadataBytes(modelPath string) ([]byte, error) {
	//nolint:gosec
	buf, err := os.ReadFile(modelPath)
	if err != nil {
		return nil, err
	}

	model := tfliteSchema.GetRootAsModel(buf, 0)
	metadataLen := model.MetadataLength()
	if metadataLen == 0 {
		return []byte{}, nil
	}

	for i := 0; i < metadataLen; i++ {
		metadata := &tfliteSchema.Metadata{}

		if success := model.Metadata(metadata, i); !success {
			return nil, FailedToLoadError("metadata")
		}

		if tfLiteMetadataName == string(metadata.Name()) {
			metadataBuffer := &tfliteSchema.Buffer{}
			success := model.Buffers(metadataBuffer, int(metadata.Buffer()))
			if !success {
				return nil, FailedToLoadError("metadata buffer")
			}

			bufInBytes := metadataBuffer.DataBytes()
			return bufInBytes, nil
		}
	}

	return []byte{}, nil
}

// getTFLiteMetadataAsStruct takes the metadata buffer returns a readable struct based on the tflite flatbuffer schema.
func getTFLiteMetadataAsStruct(metaBytes []byte) *metadata.ModelMetadataT {
	meta := metadata.GetRootAsModelMetadata(metaBytes, 0)
	structMeta := meta.UnPack()
	return structMeta
}

// Close should be called at the end of using the interpreter to delete related models and interpreters.
func (model *TFLiteStruct) Close() error {
	model.model.Delete()
	model.interpreterOptions.Delete()
	model.interpreter.Delete()
	return nil
}

// getInterpreter conforms a *tflite.Interpreter to the Interpreter interface.
func getInterpreter(model *tflite.Model, options *tflite.InterpreterOptions) (Interpreter, error) {
	interpreter := tflite.NewInterpreter(model, options)
	if interpreter == nil {
		return nil, FailedToLoadError("interpreter")
	}

	return interpreter, nil
}
