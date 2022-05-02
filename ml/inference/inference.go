// package inference contains functions to access tflite
package inference

import "C"
import (
	tflite "github.com/mattn/go-tflite"
	"github.com/pkg/errors"
	"log"
)

type InterpreterLoader struct {
	newModelFromFile      func(path string) *tflite.Model
	newInterpreterOptions func(numThreads int) (*tflite.InterpreterOptions, error)
	newInterpreter        func(model tflite.Model, options tflite.InterpreterOptions) (*tflite.Interpreter, error)
	numThreads            int
}

type TfliteObjects struct {
	Interpreter *tflite.Interpreter
	model       *tflite.Model
	options     *tflite.InterpreterOptions
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
	if err := l.validate(); err != nil {
		return nil, err
	}

	model := l.newModelFromFile(modelPath)
	if model == nil {
		return nil, errors.New("cannot load model")
	}

	options, err := l.newInterpreterOptions(l.numThreads)
	if err != nil {
		return nil, err
	}

	interpreter, err := l.newInterpreter(*model, *options)
	if err != nil {
		return nil, err
	}

	tfliteObjs := TfliteObjects{
		Interpreter: interpreter,
		model:       model,
		options:     options,
	}

	return &tfliteObjs, nil
}

// getInterpreterOptions returns a TfliteInterpreterOptions
func getInterpreterOptions(numThreads int) (*tflite.InterpreterOptions, error) {
	options := tflite.NewInterpreterOptions()
	if options == nil {
		return nil, errors.New("no interpreter options")
	}

	options.SetNumThread(numThreads)

	options.SetErrorReporter(func(msg string, userData interface{}) {
		log.Println(msg)
	}, nil)

	return options, nil
}

// Validate if this InterpreterLoader is valid.
func (l *InterpreterLoader) validate() error {
	if l.newModelFromFile == nil {
		return errors.New("need a new model function")
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
func getInterpreter(model tflite.Model, opts tflite.InterpreterOptions) (*tflite.Interpreter, error) {
	interpreter := tflite.NewInterpreter(&model, &opts)
	if interpreter == nil {
		return nil, errors.New("cannot create interpreter")
		// return nil, errors.Errorf("cannot create interpreter %v", interpreter)
	}
	return interpreter, nil
}

// Delete should be called at the end of using the interpreter to delete the instance and related parts
func (i *TfliteObjects) Delete() error {
	i.model.Delete()
	i.options.Delete()
	i.Interpreter.Delete()
	return nil
}
