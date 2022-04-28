// package inference makes inferences
package inference

import (
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
	numThreads int
}

func TestGoodModel(t *testing.T) {
	goodModelLoader := func(path string) (TfliteModel, error) { return &tflite.Model{}, nil }
	goodInterpreterLoader := func(model TfliteModel, options mockInterpreterOptions) (TfliteInterpreter, error) {
		return &tflite.Interpreter{}, nil
	}

	goodOptions := func() (mockInterpreterOptions, error) { return mockInterpreterOptions{}, nil }

	loader := &InterpreterLoader{
		NewModelFromFile:      goodModelLoader,
		NewInterpreter:        goodInterpreterLoader,
		NewInterpreterOptions: goodOptions,
	}
	interpreter, err := loader.GetTfliteInterpreter("random path", 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, interpreter, test.ShouldNotBeNil)

}

func (mIo *mockInterpreterOptions) SetNumThreads(num int) {
	return
}

func (mIo *mockInterpreterOptions) Delete() {
	return
}

func (mIo *mockInterpreterOptions) SetErrorReporter(f func(string, interface{}), user_data interface{}) {
	return
}

// func TestCheckDefaultInterpreter(t *testing.T) {
// 	loader := GetDefaultInterpreterLoader()
// 	test.That(t, reflect.TypeOf(loader.modelLoader) == reflect.TypeOf(tflite.NewModelFromFile), test.ShouldBeTrue)
// 	test.That(t, reflect.TypeOf(loader.optionsLoader) == reflect.TypeOf(tflite.NewInterpreterOptions), test.ShouldBeTrue)
// 	test.That(t, reflect.TypeOf(loader.interpreterLoader) == reflect.TypeOf(tflite.NewInterpreter), test.ShouldBeTrue)
// }

// func TestCheckRightType(t *testing.T) {
// 	loader := getMockLoader()
// 	interpreter, err := loader.GetTfliteInterpreter("/hello", 4)
// 	test.That(t, err, test.ShouldBeNil)
// 	test.That(t, interpreter, test.ShouldNotBeNil)
// 	interpreter.Model.Delete()
// 	interpreter.Options.Delete()
// 	interpreter.Interpreter.Delete()
// }

// func TestCanDelete(t *testing.T) {
// 	i := getMockInterpreterStruct()
// 	err := DeleteInterpreter(i)
// 	test.That(t, err, test.ShouldBeNil)
// }

// func getMockLoader() *InterpreterLoader {
// 	loader := &InterpreterLoader{
// 		modelLoader:       mockGetModel,
// 		optionsLoader:     mockOptions,
// 		interpreterLoader: mockInterpreter,
// 	}
// 	return loader
// }

func getMockInterpreterStruct() *FullInterpreter {
	fullInterpreter := &FullInterpreter{
		Model:       model,
		Options:     options,
		Interpreter: interpreter,
	}
	return fullInterpreter
}

func goodModelLoader(path string) *tflite.Model {
	model = &tflite.Model{}
	return model
}

func mockOptions() *tflite.InterpreterOptions {
	return options
}
func mockInterpreter(model *tflite.Model, options mockInterpreterOptions) *tflite.Interpreter {
	return interpreter
}

// func (option *tflite.InterpreterOptions) SetNumThreads(num int) {
// 	//do Nothing
// 	return
// }

// func (m *mockModel) Delete() {
// 	return
// }
