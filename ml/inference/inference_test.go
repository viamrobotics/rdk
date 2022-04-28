// package inference makes inferences
package inference

import (
	"errors"
	"testing"

	"github.com/mattn/go-tflite"
	"go.viam.com/test"
)

var (
	model       *tflite.Model
	options     *tflite.InterpreterOptions
	interpreter *tflite.Interpreter
)

type mockInterpreterOptions struct {
}

func (mIo *mockInterpreterOptions) SetNumThread(num int) {
	return
}

func (mIo *mockInterpreterOptions) Delete() {
	return
}

func (mIo *mockInterpreterOptions) SetErrorReporter(f func(string, interface{}), user_data interface{}) {
	return
}

func TestGetInterpreter(t *testing.T) {
	goodInterpreterLoader := func(model TfliteModel, options TfliteInterpreterOptions) (TfliteInterpreter, error) {
		return &tflite.Interpreter{}, nil
	}

	goodOptions := func() (TfliteInterpreterOptions, error) { return &mockInterpreterOptions{}, nil }

	loader := &InterpreterLoader{
		NewModelFromFile:      modelLoader,
		NewInterpreter:        goodInterpreterLoader,
		NewInterpreterOptions: goodOptions,
	}
	interpreter, err := loader.GetTfliteInterpreter("random path", 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.Model, test.ShouldNotBeNil)
	test.That(t, interpreter.Interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.Options, test.ShouldNotBeNil)

	interpreter, err = loader.GetTfliteInterpreter("random path2", 4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.Model, test.ShouldNotBeNil)
	test.That(t, interpreter.Interpreter, test.ShouldNotBeNil)
	test.That(t, interpreter.Options, test.ShouldNotBeNil)

	interpreter, err = loader.GetTfliteInterpreter("bad path", 4)
	test.That(t, err, test.ShouldBeError)
	test.That(t, interpreter, test.ShouldBeNil)
}

func TestBadModel(t *testing.T) {

}

func modelLoader(path string) (TfliteModel, error) {
	if path == "bad path" {
		return nil, errors.New("cannot load model")
	} else {
		return &tflite.Model{}, nil
	}
}
