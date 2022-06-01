package inference

import (
	"log"
	"runtime"
	"strconv"

	tflite "github.com/mattn/go-tflite"
	"github.com/pkg/errors"
	"go.viam.com/rdk/config"
)

// tflite specific functions

func loadTFLiteModel(modelPath string, moreInfo config.AttributeMap) (interface{}, error) {
	tFLiteModel := tflite.NewModelFromFile(modelPath)
	if tFLiteModel == nil {
		return nil, errors.New("failed to create model")
	}

	// parse attribute map for additional info
	threads := moreInfo["numThreads"]
	var numThreads int
	intThreads, ok := threads.(int)
	if !ok {
		numThreads = runtime.NumCPU()
	} else {
		numThreads = intThreads
	}

	options, err := loadTFLiteInterpreterOptions(numThreads)
	if err != nil {
		return nil, err
	}

	interpreter := tflite.NewInterpreter(tFLiteModel, options)
	if interpreter == nil {
		return nil, errors.New("failed to create interpreter")
	}

	status := interpreter.AllocateTensors()
	if status != tflite.OK {
		return nil, errors.New("failed to allocate tensors")
	}

	modelInfo := tfliteStruct{
		model:              tFLiteModel,
		interpreter:        interpreter,
		interpreterOptions: options,
	}

	return modelInfo, nil
}

// loadTFLiteInterpreterOptions returns preset tflite interpreterOptions
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
func closeTFLiteModel(tfliteStruct tfliteStruct) {
	tfliteStruct.model.Delete()
	tfliteStruct.interpreterOptions.Delete()
	tfliteStruct.interpreter.Delete()
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

func getTfliteModelInfo(tfliteStruct tfliteStruct) (interface{}, error) {
	inter := tfliteStruct.interpreter
	if inter == nil {
		return nil, errors.New("there is no tflite interpreter")
	}
	input := inter.GetInputTensor(0)

	numOut := inter.GetOutputTensorCount()
	var outTypes []string
	for i := 0; i < numOut; i++ {
		outTypes = append(outTypes, inter.GetOutputTensor(i).Type().String())
	}

	info := TFLiteInfo{
		inputHeight:       input.Dim(1),
		inputWidth:        input.Dim(2),
		inputChannels:     input.Dim(3),
		inputTensorType:   input.Type().String(),
		outputTensorTypes: outTypes,
	}
	return info, nil
}

func tfliteInfer(model tfliteStruct, inputTensor interface{}) (config.AttributeMap, error) {
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

	var output config.AttributeMap
	numOutputTensors := interpreter.GetOutputTensorCount()
	for i := 0; i < numOutputTensors; i++ {
		var buf interface{}
		currTensor := interpreter.GetOutputTensor(i)
		switch currTensor.Type() {
		case tflite.Float32:
			buf = make([]float32, currTensor.ByteSize())
		case tflite.UInt8:
			buf = make([]uint8, currTensor.ByteSize())
		case tflite.Bool:
			buf = make([]bool, currTensor.ByteSize())
		case tflite.Int8:
			buf = make([]int8, currTensor.ByteSize())
		case tflite.Int16:
			buf = make([]int16, currTensor.ByteSize())
		case tflite.Int32:
			buf = make([]int32, currTensor.ByteSize())
		case tflite.Complex64:
			buf = make([]complex64, currTensor.ByteSize())
		case tflite.String:
			buf = make([]string, currTensor.ByteSize())
		default:
			buf = make([]byte, currTensor.ByteSize())
		}
		currTensor.CopyToBuffer(buf)
		output["out"+strconv.Itoa(i)] = buf
	}

	return output, nil
}
