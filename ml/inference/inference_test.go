// inference makes inferences
package inference

import (
	"testing"

	"github.com/mattn/go-tflite"
	"github.com/pkg/errors"
	"go.viam.com/test"
)

const badPath string = "bad path"

type mockInterpreterOptions struct{}

func (mIo *mockInterpreterOptions) SetNumThread(num int) {}

func (mIo *mockInterpreterOptions) Delete() {}

func (mIo *mockInterpreterOptions) SetErrorReporter(f func(string, interface{}), userData interface{}) {
}

func TestGetInterpreter(t *testing.T) {
	goodInterpreterLoader := func(model *tflite.Model, options tfliteInterpreterOptions) (*tflite.Interpreter, error) {
		return &tflite.Interpreter{}, nil
	}

	goodOptions := func() (tfliteInterpreterOptions, error) { return &mockInterpreterOptions{}, nil }

	loader := &InterpreterLoader{
		newModelFromFile:      modelLoader,
		newInterpreter:        goodInterpreterLoader,
		newInterpreterOptions: goodOptions,
		numThreads:            4,
	}
	interpreter, err := loader.Load("random path2")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.model, test.ShouldNotBeNil)
	test.That(t, interpreter.Interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.Options, test.ShouldNotBeNil)

	interpreter, err = loader.Load(badPath)
	test.That(t, err, test.ShouldBeError, errors.New("cannot load model"))
	test.That(t, interpreter, test.ShouldBeNil)
}

func TestFailedInterpreter(t *testing.T) {
	badInterpreterLoader := func(model *tflite.Model, options tfliteInterpreterOptions) (*tflite.Interpreter, error) {
		return nil, errors.New("cannot create interpreter")
	}

	goodOptions := func() (tfliteInterpreterOptions, error) { return &mockInterpreterOptions{}, nil }

	loader := &InterpreterLoader{
		newModelFromFile:      modelLoader,
		newInterpreter:        badInterpreterLoader,
		newInterpreterOptions: goodOptions,
		numThreads:            4,
	}
	interpreter, err := loader.Load("random path")
	test.That(t, err, test.ShouldBeError, errors.New("cannot create interpreter"))
	test.That(t, interpreter, test.ShouldBeNil)
}

func TestBadNumThreads(t *testing.T) {
	goodInterpreterLoader := func(model *tflite.Model, options tfliteInterpreterOptions) (*tflite.Interpreter, error) {
		return &tflite.Interpreter{}, nil
	}

	goodOptions := func() (tfliteInterpreterOptions, error) { return &mockInterpreterOptions{}, nil }

	loader := &InterpreterLoader{
		newModelFromFile:      modelLoader,
		newInterpreter:        goodInterpreterLoader,
		newInterpreterOptions: goodOptions,
		numThreads:            -5,
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
	goodInterpreterLoader := func(model *tflite.Model, options tfliteInterpreterOptions) (*tflite.Interpreter, error) {
		return &tflite.Interpreter{}, nil
	}

	goodOptions := func() (tfliteInterpreterOptions, error) { return &mockInterpreterOptions{}, nil }

	loader := &InterpreterLoader{
		newModelFromFile:      modelLoader,
		newInterpreter:        goodInterpreterLoader,
		newInterpreterOptions: goodOptions,
	}
	interpreter, err := loader.Load("random path")
	test.That(t, err, test.ShouldBeError, errors.New("NumThreads must be a positive integer"))
	test.That(t, interpreter, test.ShouldBeNil)
}

func modelLoader(path string) *tflite.Model {
	if path == badPath {
		return nil
	}

	return &tflite.Model{}
}
