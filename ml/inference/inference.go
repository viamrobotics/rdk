// package inference contains functions to access tflite
package inference

import "C"
import (
	"log"

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
	NewInterpreterOptions func() (TfliteInterpreterOptions, error)
	NewInterpreter        func(model *tflite.Model, options TfliteInterpreterOptions) (*tflite.Interpreter, error)
	NumThreads            int
}

type TfliteX struct {
	Interpreter *tflite.Interpreter
	Model       *tflite.Model
	Options     TfliteInterpreterOptions
}

// NewDefaultInterpreterLoader returns the default loader when using tflite
func NewDefaultInterpreterLoader() *InterpreterLoader {
	loader := &InterpreterLoader{
		NewModelFromFile:      tflite.NewModelFromFile,
		NewInterpreterOptions: getInterpreterOptions,
		NewInterpreter:        getInterpreter,
		NumThreads:            4,
	}

	return loader
}

// NewInterpreterLoader returns a loader that allows you to set threads when using tflite
func NewInterpreterLoader(numThreads int) *InterpreterLoader {
	loader := &InterpreterLoader{
		NewModelFromFile:      tflite.NewModelFromFile,
		NewInterpreterOptions: getInterpreterOptions,
		NewInterpreter:        getInterpreter,
		NumThreads:            numThreads,
	}

	return loader
}

// Load returns the service a struct containing information of a tflite compatible interpreter
func (l *InterpreterLoader) Load(modelPath string) (*TfliteX, error) {
	if err := l.Validate(); err != nil {
		return nil, err
	}

	model := l.NewModelFromFile(modelPath)
	if model == nil {
		return nil, errors.New("cannot load model")
	}

	options, err := l.NewInterpreterOptions()
	if err != nil {
		return nil, err
	}

	options.SetNumThread(l.NumThreads)

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

// getInterpreterOptions returns a TfliteInterpreterOptions
func getInterpreterOptions() (TfliteInterpreterOptions, error) {
	options := tflite.NewInterpreterOptions()
	if options == nil {
		return nil, errors.Errorf("no interpreter options %v", options)
	}

	return options, nil
}

// Validate if this InterpreterLoader is valid.
func (l *InterpreterLoader) Validate() error {
	if l.NewModelFromFile == nil {
		return errors.New("need a new model function")
	}

	if l.NewInterpreterOptions == nil {
		return errors.New("need a new interpreter options function")
	}

	if l.NewInterpreter == nil {
		return errors.New("need a new interpreter function")
	}

	if l.NumThreads <= 0 {
		return errors.New("NumThreads must be a positive integer")
	}

	return nil
}

// getInterpreter returns a tflite.Interpreter
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
