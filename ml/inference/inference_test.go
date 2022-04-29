// inference makes inferences
package inference

import (
	"testing"

	"github.com/mattn/go-tflite"
	"github.com/pkg/errors"
	"go.viam.com/test"
)

const BADPATH string = "bad path"

type mockInterpreterOptions struct{}

func (mIo *mockInterpreterOptions) SetNumThread(num int) {}

func (mIo *mockInterpreterOptions) Delete() {}

func (mIo *mockInterpreterOptions) SetErrorReporter(f func(string, interface{}), userData interface{}) {
}

func TestGetInterpreter(t *testing.T) {
	goodInterpreterLoader := func(model *tflite.Model, options TfliteInterpreterOptions) (*tflite.Interpreter, error) {
		return &tflite.Interpreter{}, nil
	}

	goodOptions := func() (TfliteInterpreterOptions, error) { return &mockInterpreterOptions{}, nil }

	loader := &InterpreterLoader{
		NewModelFromFile:      modelLoader,
		NewInterpreter:        goodInterpreterLoader,
		NewInterpreterOptions: goodOptions,
		NumThreads:            4,
	}
	interpreter, err := loader.Load("random path2")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.Model, test.ShouldNotBeNil)
	test.That(t, interpreter.Interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.Options, test.ShouldNotBeNil)

	interpreter, err = loader.Load(BADPATH)
	test.That(t, err, test.ShouldBeError, errors.New("cannot load model"))
	test.That(t, interpreter, test.ShouldBeNil)
}

func TestFailedInterpreter(t *testing.T) {
	badInterpreterLoader := func(model *tflite.Model, options TfliteInterpreterOptions) (*tflite.Interpreter, error) {
		return nil, errors.New("cannot create interpreter")
	}

	goodOptions := func() (TfliteInterpreterOptions, error) { return &mockInterpreterOptions{}, nil }

	loader := &InterpreterLoader{
		NewModelFromFile:      modelLoader,
		NewInterpreter:        badInterpreterLoader,
		NewInterpreterOptions: goodOptions,
		NumThreads:            4,
	}
	interpreter, err := loader.Load("random path")
	test.That(t, err, test.ShouldBeError, errors.New("cannot create interpreter"))
	test.That(t, interpreter, test.ShouldBeNil)
}

func TestBadNumThreads(t *testing.T) {
	goodInterpreterLoader := func(model *tflite.Model, options TfliteInterpreterOptions) (*tflite.Interpreter, error) {
		return &tflite.Interpreter{}, nil
	}

	goodOptions := func() (TfliteInterpreterOptions, error) { return &mockInterpreterOptions{}, nil }

	loader := &InterpreterLoader{
		NewModelFromFile:      modelLoader,
		NewInterpreter:        goodInterpreterLoader,
		NewInterpreterOptions: goodOptions,
		NumThreads:            -5,
	}
	interpreter, err := loader.Load("random path")
	test.That(t, err, test.ShouldBeError, errors.New("NumThreads must be a positive integer"))
	test.That(t, interpreter, test.ShouldBeNil)
}

func TestNilLoader(t *testing.T) {
	loader := &InterpreterLoader{}
	interpreter, err := loader.Load("random path")
	test.That(t, err, test.ShouldBeError, errors.New("need a new model function"))
	test.That(t, interpreter, test.ShouldBeNil)
}

func TestNilNumThreads(t *testing.T) {
	goodInterpreterLoader := func(model *tflite.Model, options TfliteInterpreterOptions) (*tflite.Interpreter, error) {
		return &tflite.Interpreter{}, nil
	}

	goodOptions := func() (TfliteInterpreterOptions, error) { return &mockInterpreterOptions{}, nil }

	loader := &InterpreterLoader{
		NewModelFromFile:      modelLoader,
		NewInterpreter:        goodInterpreterLoader,
		NewInterpreterOptions: goodOptions,
	}
	interpreter, err := loader.Load("random path")
	test.That(t, err, test.ShouldBeError, errors.New("NumThreads must be a positive integer"))
	test.That(t, interpreter, test.ShouldBeNil)
}

func modelLoader(path string) *tflite.Model {
	if path == BADPATH {
		return nil
	}

	return &tflite.Model{}
}
