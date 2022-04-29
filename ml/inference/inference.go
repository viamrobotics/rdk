// package inference contains functions to access tflite
package inference

import "C"
import (
	"log"

	"github.com/pkg/errors"

	tflite "github.com/mattn/go-tflite"
)

type tfliteInterpreterOptions interface {
	Delete()
	SetNumThread(num int)
	SetErrorReporter(f func(string, interface{}), userData interface{})
}

type InterpreterLoader struct {
	newModelFromFile      func(path string) *tflite.Model
	newInterpreterOptions func() (tfliteInterpreterOptions, error)
	newInterpreter        func(model *tflite.Model, options tfliteInterpreterOptions) (*tflite.Interpreter, error)
	numThreads            int
}

type TfliteObjects struct {
	Interpreter *tflite.Interpreter
	model       *tflite.Model
	Options     tfliteInterpreterOptions
}

// NewDefaultInterpreterLoader returns the default loader when using tflite
func NewDefaultInterpreterLoader() *InterpreterLoader {
	loader := &InterpreterLoader{
		newModelFromFile:      tflite.NewModelFromFile,
		newInterpreterOptions: getInterpreterOptions,
		newInterpreter:        getInterpreter,
		numThreads:            4,
	}

	return loader
}

// NewInterpreterLoader returns a loader that allows you to set threads when using tflite
func NewInterpreterLoader(numThreads int) *InterpreterLoader {
	loader := &InterpreterLoader{
		newModelFromFile:      tflite.NewModelFromFile,
		newInterpreterOptions: getInterpreterOptions,
		newInterpreter:        getInterpreter,
		numThreads:            numThreads,
	}

	return loader
}

// Load returns the service a struct containing information of a tflite compatible interpreter
func (l *InterpreterLoader) Load(modelPath string) (*TfliteObjects, error) {
	if err := l.Validate(); err != nil {
		return nil, err
	}

	model := l.newModelFromFile(modelPath)
	if model == nil {
		return nil, errors.New("cannot load model")
	}

	options, err := l.newInterpreterOptions()
	if err != nil {
		return nil, err
	}

	options.SetNumThread(l.numThreads)

	options.SetErrorReporter(func(msg string, userData interface{}) {
		log.Println(msg)
	}, nil)

	interpreter, err := l.newInterpreter(model, options)
	if err != nil {
		return nil, err
	}

	tfliteObjs := TfliteObjects{
		Interpreter: interpreter,
		model:       model,
		Options:     options,
	}

	return &tfliteObjs, nil
}

// getInterpreterOptions returns a TfliteInterpreterOptions
func getInterpreterOptions() (tfliteInterpreterOptions, error) {
	options := tflite.NewInterpreterOptions()
	if options == nil {
		return nil, errors.New("no interpreter options")
	}

	return options, nil
}

// Validate if this InterpreterLoader is valid.
func (l *InterpreterLoader) Validate() error {
	if l.newModelFromFile == nil {
		return errors.New("need a new model function")
	}

	if l.newInterpreterOptions == nil {
		return errors.New("need a new interpreter options function")
	}

	if l.newInterpreter == nil {
		return errors.New("need a new interpreter function")
	}

	if l.numThreads <= 0 {
		return errors.New("NumThreads must be a positive integer")
	}

	return nil
}

// getInterpreter returns a tflite.Interpreter
func getInterpreter(model *tflite.Model, options tfliteInterpreterOptions) (*tflite.Interpreter, error) {
	tfliteOptions, ok := options.(*tflite.InterpreterOptions)
	if !ok {
		return nil, errors.New("not a tflite.InterpreterOptions")
	}

	interpreter := tflite.NewInterpreter(model, tfliteOptions)
	if interpreter == nil {
		return nil, errors.New("cannot create interpreter")
		// return nil, errors.Errorf("cannot create interpreter %v", interpreter)
	}
	return interpreter, nil
}

// DeleteInterpreter should be called at the end of using the interpreter to delete the instance and related parts
func (i *TfliteObjects) DeleteInterpreter() error {
	i.model.Delete()
	i.Options.Delete()
	i.Interpreter.Delete()
	return nil
}
