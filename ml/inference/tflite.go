//go:build !arm && !windows && !no_tflite

package inference

import (
	"log"
	"os"
	"runtime"
	"sync"

	tflite "github.com/mattn/go-tflite"
	"github.com/pkg/errors"

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

// Infer takes an input array in desired type and returns an array of the output tensors.
func (model *TFLiteStruct) Infer(inputTensor interface{}) ([]interface{}, error) {
	model.mu.Lock()
	defer model.mu.Unlock()

	interpreter := model.interpreter
	input := interpreter.GetInputTensor(0)
	status := input.CopyFromBuffer(inputTensor)
	if status != tflite.OK {
		return nil, errors.New("copying to buffer failed")
	}

	status = interpreter.Invoke()
	if status != tflite.OK {
		return nil, errors.New("invoke failed")
	}

	var output []interface{}
	numOutputTensors := interpreter.GetOutputTensorCount()
	for i := 0; i < numOutputTensors; i++ {
		var buf interface{}
		currTensor := interpreter.GetOutputTensor(i)
		if currTensor == nil {
			continue
		}
		switch currTensor.Type() {
		case tflite.Float32:
			buf = currTensor.Float32s()
		case tflite.UInt8:
			buf = currTensor.UInt8s()
		case tflite.Bool:
			buf = make([]bool, currTensor.ByteSize())
			currTensor.CopyToBuffer(buf)
		case tflite.Int8:
			buf = currTensor.Int8s()
		case tflite.Int16:
			buf = currTensor.Int16s()
		case tflite.Int32:
			buf = currTensor.Int32s()
		case tflite.Int64:
			buf = currTensor.Int64s()
		case tflite.Complex64:
			buf = make([]complex64, currTensor.ByteSize()/8)
			currTensor.CopyToBuffer(buf)
		case tflite.String, tflite.NoType:
			// TODO: find a model that outputs tflite.String to test
			// if there is a better solution than this
			buf = make([]byte, currTensor.ByteSize())
			currTensor.CopyToBuffer(buf)
		default:
			return nil, FailedToGetError("output tensor type")
		}
		output = append(output, buf)
	}
	return output, nil
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
