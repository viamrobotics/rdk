// package inference contains functions to access tflite
package inference

import "C"
import (
	_ "image/jpeg"
	_ "image/png"
	"log"
	_ "path/filepath"

	"github.com/pkg/errors"

	tflite "github.com/mattn/go-tflite"
)

type TfliteModel interface {
	Delete()
}

type TfliteInterpreterOptions interface {
	Delete()
	SetNumThread(num int)
	SetErrorReporter(f func(string, interface{}), userData interface{})
}

type TfliteInterpreter interface {
	Delete()
}

type InterpreterLoader struct {
	NewModelFromFile      func(path string) *tflite.Model
	NewInterpreter        func(model TfliteModel, options TfliteInterpreterOptions) (TfliteInterpreter, error)
	NewInterpreterOptions func() (TfliteInterpreterOptions, error)
}

type FullInterpreter struct {
	Interpreter TfliteInterpreter
	Model       TfliteModel
	Options     TfliteInterpreterOptions
}

// GetDefaultInterpreterLoader returns the default loader when using tflite
func GetDefaultInterpreterLoader() *InterpreterLoader {
	loader := &InterpreterLoader{
		NewInterpreter:        GetInterpreter,
		NewModelFromFile:      tflite.NewModelFromFile,
		NewInterpreterOptions: GetInterpreterOptions,
	}

	return loader
}

// GetTfliteInterpreter returns the service a struct containing information of a tflite compatible interpreter
func (l *InterpreterLoader) GetTfliteInterpreter(modelPath string, numThreads int) (*FullInterpreter, error) {
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

	fullInterpreter := FullInterpreter{
		Interpreter: interpreter,
		Model:       model,
		Options:     options,
	}

	return &fullInterpreter, nil
}

func GetInterpreterOptions() (TfliteInterpreterOptions, error) {
	options := tflite.NewInterpreterOptions()
	if options == nil {
		return nil, errors.Errorf("no interpreter options %v", options)
	}

	return options, nil
}

func GetInterpreter(model TfliteModel, options TfliteInterpreterOptions) (TfliteInterpreter, error) {
	tfliteModel, ok := model.(*tflite.Model)
	if !ok {
		return nil, errors.New("not a tflite.Model")
	}

	tfliteOptions, ok := model.(*tflite.InterpreterOptions)
	if !ok {
		return nil, errors.New("not a tflite.InterpreterOptions")
	}

	interpreter := tflite.NewInterpreter(tfliteModel, tfliteOptions)
	if interpreter == nil {
		return nil, errors.Errorf("cannot create interpreter %v", interpreter)
	}
	return interpreter, nil
}

// DeleteInterpreter should be called at the end of using the interpreter to delete the instance and related parts
func DeleteInterpreter(i *FullInterpreter) error {
	i.Model.Delete()
	i.Options.Delete()
	i.Interpreter.Delete()
	return nil
}
