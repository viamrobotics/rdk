// inference makes inferences
package inference

import (
	"testing"

	"github.com/pkg/errors"

	"github.com/mattn/go-tflite"
	"go.viam.com/test"
)

const BAD_PATH = "bad path"

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
	}
	interpreter, err := loader.Load("random path", 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.Model, test.ShouldNotBeNil)
	test.That(t, interpreter.Interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.Options, test.ShouldNotBeNil)

	interpreter, err = loader.Load("random path2", 4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.Model, test.ShouldNotBeNil)
	test.That(t, interpreter.Interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.Options, test.ShouldNotBeNil)

	interpreter, err = loader.Load(BAD_PATH, 4)
	test.That(t, err, test.ShouldBeError)
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
	}
	interpreter, err := loader.Load("random path", 0)
	test.That(t, err, test.ShouldBeError)
	test.That(t, interpreter, test.ShouldBeNil)
}

func modelLoader(path string) *tflite.Model {
	if path == BAD_PATH {
		return nil
	}

	return &tflite.Model{}
}
