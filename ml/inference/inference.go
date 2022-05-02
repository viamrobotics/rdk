// package inference contains functions to access tflite
package inference

import "C"
import (
	"log"

	"github.com/pkg/errors"

	tflite "github.com/mattn/go-tflite"
)

type InterpreterLoader struct {
	newModelFromFile   func(path string) *tflite.Model
	newInterpreter     func(model *tflite.Model, options *tflite.InterpreterOptions) *tflite.Interpreter
	interpreterOptions *tflite.InterpreterOptions
}

type TfliteInterpreter struct {
	Interpreter *tflite.Interpreter
	model       *tflite.Model
	options     *tflite.InterpreterOptions
}

// NewDefaultInterpreterLoader returns the default loader when using tflite
func NewDefaultInterpreterLoader() (*InterpreterLoader, error) {
	options, err := loadOptions(4)
	if err != nil {
		return nil, err
	}

	loader := &InterpreterLoader{
		newModelFromFile:   tflite.NewModelFromFile,
		newInterpreter:     tflite.NewInterpreter,
		interpreterOptions: options,
	}

	return loader, nil
}

// NewInterpreterLoader returns a loader that allows you to set threads when using tflite
func NewInterpreterLoader(numThreads int) (*InterpreterLoader, error) {
	if numThreads <= 0 {
		return nil, errors.New("numThreads must be a positive integer")
	}

	options, err := loadOptions(numThreads)
	if err != nil {
		return nil, err
	}

	loader := &InterpreterLoader{
		newModelFromFile:   tflite.NewModelFromFile,
		newInterpreter:     tflite.NewInterpreter,
		interpreterOptions: options,
	}

	return loader, nil
}

// loadOptions function sets up tflite.InterpreterOptions based on thread count
func loadOptions(numThreads int) (*tflite.InterpreterOptions, error) {
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

// Load returns the service a struct containing information of a tflite compatible interpreter
func (l *InterpreterLoader) Load(modelPath string) (*TfliteInterpreter, error) {
	if err := l.validate(); err != nil {
		return nil, err
	}

	model := l.newModelFromFile(modelPath)
	if model == nil {
		return nil, errors.New("cannot load model")
	}

	interpreter := l.newInterpreter(model, l.interpreterOptions)
	if interpreter == nil {
		return nil, errors.New("cannot create interpreter")
	}

	tfliteObjs := TfliteInterpreter{
		Interpreter: interpreter,
		model:       model,
		options:     l.interpreterOptions,
	}

	return &tfliteObjs, nil
}

// validate if this InterpreterLoader is valid.
func (l *InterpreterLoader) validate() error {
	if l.newModelFromFile == nil {
		return errors.New("need a new model function")
	}

	if l.newInterpreter == nil {
		return errors.New("need a new interpreter function")
	}

	return nil
}

// Delete should be called at the end of using the interpreter to delete the instance and related parts
func (t *TfliteInterpreter) Delete() error {
	t.model.Delete()
	t.options.Delete()
	t.Interpreter.Delete()
	return nil
}
