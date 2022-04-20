// package inference makes inferences
package inference

import (
	"reflect"
	"testing"

	"github.com/mattn/go-tflite"
	"go.viam.com/test"
)

var (
	model       *tflite.Model
	options     *tflite.InterpreterOptions
	interpreter *tflite.Interpreter
)

func TestCheckDefaultInterpreter(t *testing.T) {
	loader := GetDefaultInterpreterLoader()
	test.That(t, reflect.TypeOf(loader.modelLoader) == reflect.TypeOf(tflite.NewModelFromFile), test.ShouldBeTrue)
	test.That(t, reflect.TypeOf(loader.optionsLoader) == reflect.TypeOf(tflite.NewInterpreterOptions), test.ShouldBeTrue)
	test.That(t, reflect.TypeOf(loader.interpreterLoader) == reflect.TypeOf(tflite.NewInterpreter), test.ShouldBeTrue)
}

func TestCheckRightType(t *testing.T) {
	loader := getMockLoader()
	interpreter, err := loader.GetTfliteInterpreter("/hello", 4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, interpreter, test.ShouldNotBeNil)
	interpreter.Model.Delete()
	interpreter.Options.Delete()
	interpreter.Interpreter.Delete()
}

func TestCanDelete(t *testing.T) {
	i := getMockInterpreterStruct()
	err := DeleteInterpreter(i)
	test.That(t, err, test.ShouldBeNil)
}

func getMockLoader() *InterpreterLoader {
	loader := &InterpreterLoader{
		modelLoader:       mockGetModel,
		optionsLoader:     mockOptions,
		interpreterLoader: mockInterpreter,
	}
	return loader
}

func getMockInterpreterStruct() *FullInterpreter {
	fullInterpreter := &FullInterpreter{
		Model:       model,
		Options:     options,
		Interpreter: interpreter,
	}
	return fullInterpreter
}

func mockGetModel(path string) *tflite.Model {
	model = &tflite.Model{}
	return model
}

func mockOptions() *tflite.InterpreterOptions {
	return options
}
func mockInterpreter(model *tflite.Model, options *tflite.InterpreterOptions) *tflite.Interpreter {
	return interpreter
}

func (option *tflite.InterpreterOptions) SetNumThreads(num int) {
	//do Nothing
	return
}
