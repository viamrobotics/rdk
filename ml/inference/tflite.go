package inference

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"runtime"

	tflite "github.com/mattn/go-tflite"
	"github.com/pkg/errors"
	tfliteSchema "go.viam.com/rdk/ml/inference/tflite"
	metadata "go.viam.com/rdk/ml/inference/tflite_metadata"
)

type TFLiteStruct struct {
	model              *tflite.Model
	interpreter        Interpreter
	interpreterOptions *tflite.InterpreterOptions
	info               *TFLiteInfo
	modelPath          string
}

type TFLiteModelLoader struct {
	newModelFromFile   func(path string) *tflite.Model
	newInterpreter     func(model *tflite.Model, options *tflite.InterpreterOptions) (Interpreter, error)
	interpreterOptions *tflite.InterpreterOptions
	getInfo            func(inter Interpreter) *TFLiteInfo
}

// NewDefaultTFLiteModelLoader returns the default loader when using tflite
func NewDefaultTFLiteModelLoader() (*TFLiteModelLoader, error) {
	options, err := loadTFLiteInterpreterOptions(runtime.NumCPU())
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

// NewTFLiteModelLoader returns a loader that allows you to set threads when using tflite
func NewTFLiteModelLoader(numThreads int) (*TFLiteModelLoader, error) {
	if numThreads <= 0 {
		return nil, errors.New("numThreads must be a positive integer")
	}

	options, err := loadTFLiteInterpreterOptions(numThreads)
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

type TFLiteInterpreter interface {
	AllocateTensors() tflite.Status
}

func (loader TFLiteModelLoader) Load(modelPath string) (*TFLiteStruct, error) {
	tFLiteModel := loader.newModelFromFile(modelPath)
	if tFLiteModel == nil {
		return nil, errors.New("cannot load model")
	}

	fmt.Println("print here 0 ")
	interpreter, err := loader.newInterpreter(tFLiteModel, loader.interpreterOptions)
	if err != nil {
		return nil, errors.New("failed to create interpreter")
	}

	fmt.Println("print here 1 ")
	status := interpreter.AllocateTensors()
	if status != tflite.OK {
		return nil, errors.New("failed to allocate tensors")
	}
	fmt.Println("print here 2 ")

	info := loader.getInfo(interpreter)
	if info == nil {
		fmt.Println("base info")
		return nil, errors.New("failed to get info")
	}

	fmt.Println("print here 3 ")

	modelStruct := &TFLiteStruct{
		model:              tFLiteModel,
		interpreter:        interpreter,
		interpreterOptions: loader.interpreterOptions,
		info:               info,
		modelPath:          modelPath,
	}

	fmt.Println("print here 4 ")
	return modelStruct, nil
}

// loadTFLiteInterpreterOptions returns tflite interpreterOptions with settings
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

// closeTFLiteModel should be called at the end of using the interpreter to delete the instance and related parts
func (model *TFLiteStruct) Close() {
	model.model.Delete()
	model.interpreterOptions.Delete()
	model.interpreter.Delete()
}

type TFLiteInfo struct {
	inputHeight       int
	inputWidth        int
	inputChannels     int
	inputTensorType   string
	inputTensorCount  int
	outputTensorCount int
	outputTensorTypes []string
}

func getInfo(inter Interpreter) *TFLiteInfo {
	input := inter.GetInputTensor(0)

	numOut := inter.GetOutputTensorCount()
	var outTypes []string
	for i := 0; i < numOut; i++ {
		outTypes = append(outTypes, inter.GetOutputTensor(i).Type().String())
	}

	info := &TFLiteInfo{
		inputHeight:       input.Dim(1),
		inputWidth:        input.Dim(2),
		inputChannels:     input.Dim(3),
		inputTensorType:   input.Type().String(),
		inputTensorCount:  inter.GetInputTensorCount(),
		outputTensorCount: numOut,
		outputTensorTypes: outTypes,
	}
	return info
}

// Infer takes the input array in desired type and returns a map of the output tensors
func (model *TFLiteStruct) Infer(inputTensor interface{}) ([]interface{}, error) {
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

	// TODO: change back to config.AttributeMap because the tensors can be diff types
	var output []interface{}
	numOutputTensors := interpreter.GetOutputTensorCount()
	for i := 0; i < numOutputTensors; i++ {
		var buf interface{}
		currTensor := interpreter.GetOutputTensor(i)
		fmt.Println(i)
		switch currTensor.Type() {
		case tflite.Float32:
			buf = make([]float32, currTensor.ByteSize()/4)
		case tflite.UInt8:
			buf = make([]uint8, currTensor.ByteSize())
		case tflite.Bool:
			buf = make([]bool, currTensor.ByteSize())
		case tflite.Int8:
			buf = make([]int8, currTensor.ByteSize())
		case tflite.Int16:
			buf = make([]int16, currTensor.ByteSize()/2)
		case tflite.Int32:
			buf = make([]int32, currTensor.ByteSize()/4)
		case tflite.Complex64:
			buf = make([]complex64, currTensor.ByteSize()/8)
		case tflite.String:
			// TODO: look into what to do if it's a string since
			// strings are diff lengths and take up diff number
			// of bytes depending on the word
			buf = make([]byte, currTensor.ByteSize())
		default:
			buf = make([]byte, currTensor.ByteSize())
		}
		currTensor.CopyToBuffer(buf)
		output = append(output, buf)
	}

	return output, nil
}

// GetMetadata provides the metadata information based on the model flatbuffer file
func (model *TFLiteStruct) GetMetadata() (interface{}, error) {
	b, err := getTFLiteMetadataBytes(model.modelPath)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, errors.New("no metadata is present")
	}
	return getTFLiteMetadataAsStruct(b), nil
}

const tfLiteMetadataName string = "TFLITE_METADATA"

// getTFLiteMetadataBytes takes a model path of a tflite file and extracts the metadata buffer from the entire model.
func getTFLiteMetadataBytes(modelPath string) ([]byte, error) {
	buf, err := ioutil.ReadFile(modelPath)
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
			return nil, errors.New("failed to assign metadata")
		}

		if bytes.Equal([]byte(tfLiteMetadataName), metadata.Name()) {
			metadataBuffer := &tfliteSchema.Buffer{}
			success := model.Buffers(metadataBuffer, int(metadata.Buffer()))
			if !success {
				return nil, errors.New("failed to assign metadata buffer")
			}

			bufInBytes := metadataBuffer.DataBytes()
			return bufInBytes, nil
		}
	}

	return nil, nil
}

// getTFLiteMetadataAsStruct takes the metadata buffer returns a readable struct based on the tflite flatbuffer schema
func getTFLiteMetadataAsStruct(metaBytes []byte) *metadata.ModelMetadataT {
	meta := metadata.GetRootAsModelMetadata(metaBytes, 0)
	structMeta := meta.UnPack()
	return structMeta
}

// getInterpreter conforms a *tflite.Interpreter to the Interpreter interface
func getInterpreter(model *tflite.Model, options *tflite.InterpreterOptions) (Interpreter, error) {
	interpreter := tflite.NewInterpreter(model, options)
	if interpreter == nil {
		return nil, errors.New("failed to create interpreter")
	}

	return interpreter, nil
}
