// inference makes inferences
package inference

import (
	"testing"

	tflite "github.com/mattn/go-tflite"
	"github.com/pkg/errors"
	"go.viam.com/test"
)

const badPath string = "bad path"

func goodInterpreterLoader(model *tflite.Model, options *tflite.InterpreterOptions) *tflite.Interpreter {
	return &tflite.Interpreter{}
}

var goodOptions *tflite.InterpreterOptions = &tflite.InterpreterOptions{}

func TestGetInterpreter(t *testing.T) {
	loader := &InterpreterLoader{
		newModelFromFile:   modelLoader,
		newInterpreter:     goodInterpreterLoader,
		interpreterOptions: goodOptions,
	}
	interpreter, err := loader.Load("random path2")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.model, test.ShouldNotBeNil)
	test.That(t, interpreter.Interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.options, test.ShouldNotBeNil)

	interpreter, err = loader.Load(badPath)
	test.That(t, err, test.ShouldBeError, errors.New("cannot load model"))
	test.That(t, interpreter, test.ShouldBeNil)
}

func TestFailedInterpreter(t *testing.T) {
	badInterpreterLoader := func(model *tflite.Model, options *tflite.InterpreterOptions) *tflite.Interpreter {
		return nil
	}

	loader := &InterpreterLoader{
		newModelFromFile:   modelLoader,
		newInterpreter:     badInterpreterLoader,
		interpreterOptions: nil,
	}
	interpreter, err := loader.Load("random path")
	test.That(t, err, test.ShouldBeError, errors.New("cannot create interpreter"))
	test.That(t, interpreter, test.ShouldBeNil)
}

func TestBadNumThreads(t *testing.T) {
	loader, err := NewInterpreterLoader(-1)
	test.That(t, err, test.ShouldBeError, errors.New("numThreads must be a positive integer"))
	test.That(t, loader, test.ShouldBeNil)

	loader, err = NewInterpreterLoader(0)
	test.That(t, err, test.ShouldBeError, errors.New("numThreads must be a positive integer"))
	test.That(t, loader, test.ShouldBeNil)
}

func TestNilLoader(t *testing.T) {
	loader := &InterpreterLoader{}
	interpreter, err := loader.Load("random path")
	test.That(t, err, test.ShouldBeError, errors.New("need a new model function"))
	test.That(t, interpreter, test.ShouldBeNil)
}

func modelLoader(path string) *tflite.Model {
	if path == badPath {
		return nil
	}

	return &tflite.Model{}
}
