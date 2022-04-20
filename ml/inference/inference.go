// package inference contains functions to access tflite
package inference

import "C"
import (
	"errors"
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	_ "path/filepath"

	"github.com/mattn/go-tflite"
)

type InterpreterLoader struct {
	modelLoader       func(path string) *tflite.Model
	optionsLoader     func() *tflite.InterpreterOptions
	interpreterLoader func(model *tflite.Model, options *tflite.InterpreterOptions) *tflite.Interpreter
}

type FullInterpreter struct {
	Interpreter *tflite.Interpreter
	Model       *tflite.Model
	Options     *tflite.InterpreterOptions
}

// GetDefaultInterpreterLoader returns the default loader when using tflite
func GetDefaultInterpreterLoader() *InterpreterLoader {
	loader := &InterpreterLoader{
		modelLoader:       tflite.NewModelFromFile,
		optionsLoader:     tflite.NewInterpreterOptions,
		interpreterLoader: tflite.NewInterpreter,
	}
	return loader
}

// GetTfliteInterpreter returns the service a struct containing information of a tflite compatible interpreter
func (l *InterpreterLoader) GetTfliteInterpreter(modelPath string, numThreads int) (*FullInterpreter, error) {
	model := l.modelLoader(modelPath)
	if model == nil {
		return nil, errors.New("cannot load model")
	}

	options := l.optionsLoader()
	if numThreads == 0 {
		options.SetNumThread(4)
	} else {
		options.SetNumThread(numThreads)
	}

	options.SetErrorReporter(func(msg string, user_data interface{}) {
		fmt.Println(msg)
	}, nil)

	interpreter := l.interpreterLoader(model, options)
	if interpreter == nil {
		return nil, errors.New("cannot create interpreter")
	}

	fullInterpreter := FullInterpreter{
		Interpreter: interpreter,
		Model:       model,
		Options:     options,
	}

	return &fullInterpreter, nil

}

// DeleteInterpreter should be called at the end of using the interpreter to delete the instance and related parts
func DeleteInterpreter(i *FullInterpreter) error {
	i.Model.Delete()
	i.Options.Delete()
	i.Interpreter.Delete()
	return nil
}
