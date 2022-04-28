// package inference contains functions to access tflite
package inference

import "C"
import (
	"log"
	_ "path/filepath"

	"github.com/pkg/errors"

	tflite "github.com/mattn/go-tflite"
)

type TfliteInterpreterOptions interface {
	Delete()
	SetNumThread(num int)
	SetErrorReporter(f func(string, interface{}), userData interface{})
}

type InterpreterLoader struct {
	NewModelFromFile      func(path string) *tflite.Model
	NewInterpreter        func(model *tflite.Model, options TfliteInterpreterOptions) (*tflite.Interpreter, error)
	NewInterpreterOptions func() (TfliteInterpreterOptions, error)
}

type TfliteX struct {
	Interpreter *tflite.Interpreter
	Model       *tflite.Model
	Options     TfliteInterpreterOptions
}

// NewDefaultInterpreterLoader returns the default loader when using tflite
func NewDefaultInterpreterLoader() *InterpreterLoader {
	loader := &InterpreterLoader{
		NewInterpreter:        getInterpreter,
		NewModelFromFile:      tflite.NewModelFromFile,
		NewInterpreterOptions: getInterpreterOptions,
	}

	return loader
}

// GetTfliteInterpreter returns the service a struct containing information of a tflite compatible interpreter
func (l *InterpreterLoader) Load(modelPath string, numThreads int) (*TfliteX, error) {
	model := l.NewModelFromFile(modelPath)
	if model == nil {
		return nil, errors.New("cannot load model")
	}

	options, err := l.NewInterpreterOptions()
	if err != nil {
		return nil, err
	}

	if numThreads == 0 {
		options.SetNumThread(4)
	} else {
		options.SetNumThread(numThreads)
	}

	options.SetErrorReporter(func(msg string, userData interface{}) {
		log.Println(msg)
	}, nil)

	interpreter, err := l.NewInterpreter(model, options)
	if err != nil {
		return nil, err
	}

	tfliteX := TfliteX{
		Interpreter: interpreter,
		Model:       model,
		Options:     options,
	}

	return &tfliteX, nil
}

func getInterpreterOptions() (TfliteInterpreterOptions, error) {
	options := tflite.NewInterpreterOptions()
	if options == nil {
		return nil, errors.Errorf("no interpreter options %v", options)
	}

	return options, nil
}

func getInterpreter(model *tflite.Model, options TfliteInterpreterOptions) (*tflite.Interpreter, error) {
	tfliteOptions, ok := options.(*tflite.InterpreterOptions)
	if !ok {
		return nil, errors.New("not a tflite.InterpreterOptions")
	}

	interpreter := tflite.NewInterpreter(model, tfliteOptions)
	if interpreter == nil {
		return nil, errors.Errorf("cannot create interpreter %v", interpreter)
	}
	return interpreter, nil
}

// DeleteInterpreter should be called at the end of using the interpreter to delete the instance and related parts
func DeleteInterpreter(i *TfliteX) error {
	i.Model.Delete()
	i.Options.Delete()
	i.Interpreter.Delete()
	return nil
}
