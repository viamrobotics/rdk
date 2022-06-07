package inference

import (
	"path/filepath"
	"runtime"
	"testing"

	tflite "github.com/mattn/go-tflite"
	"github.com/pkg/errors"
	"go.viam.com/test"
)

const badPath string = "bad path"

var (
	_, b, _, _ = runtime.Caller(0)
	basePath   = filepath.Dir(b)
)

type fakeInterpreter struct {
	outTensorNum int
	inTensorNum  int
}

func goodInterpreterLoader(model *tflite.Model, options *tflite.InterpreterOptions) (Interpreter, error) {
	return &fakeInterpreter{}, nil
}

func (fI fakeInterpreter) AllocateTensors() tflite.Status {
	return tflite.OK
}

func (fI fakeInterpreter) Invoke() tflite.Status {
	return tflite.OK
}

func (fI fakeInterpreter) GetOutputTensorCount() int {
	return fI.outTensorNum
}

func (fI fakeInterpreter) GetInputTensorCount() int {
	return fI.inTensorNum
}

func (fI fakeInterpreter) GetOutputTensor(i int) *tflite.Tensor {
	return &tflite.Tensor{}
}

func (fI fakeInterpreter) GetInputTensor(i int) *tflite.Tensor {
	return &tflite.Tensor{}
}

func (fI fakeInterpreter) Delete() {}

var goodOptions *tflite.InterpreterOptions = &tflite.InterpreterOptions{}

func goodGetInfo(i Interpreter) *TFLiteInfo {
	return &TFLiteInfo{}
}

func TestLoadModel(t *testing.T) {
	tfliteModelPath := basePath + "/testing_files/model_with_metadata.tflite"
	loader, err := NewDefaultTFLiteModelLoader()
	test.That(t, err, test.ShouldBeNil)
	tfliteStruct, err := loader.Load(tfliteModelPath)
	test.That(t, tfliteStruct, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	structInfo := tfliteStruct.Info
	test.That(t, structInfo, test.ShouldNotBeNil)

	h := structInfo.InputHeight
	w := structInfo.InputWidth
	c := structInfo.InputChannels
	test.That(t, h, test.ShouldEqual, 640)
	test.That(t, w, test.ShouldEqual, 640)
	test.That(t, c, test.ShouldEqual, 3)
	test.That(t, structInfo.InputTensorType, test.ShouldEqual, "Float32")
	test.That(t, structInfo.InputTensorCount, test.ShouldEqual, 1)
	test.That(t, structInfo.OutputTensorCount, test.ShouldEqual, 4)
	test.That(t, structInfo.OutputTensorTypes, test.ShouldResemble, []string{"Float32", "Float32", "Float32", "Float32"})

	buf := make([]float32, c*h*w)
	outTensors, err := tfliteStruct.Infer(buf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outTensors, test.ShouldNotBeNil)
	test.That(t, len(outTensors), test.ShouldEqual, 4)

	tfliteStruct.Close()
}

func TestLoadRealBadPath(t *testing.T) {
	tfliteModelPath := basePath + "/testing_files/does_not_exist.tflite"
	loader, err := NewDefaultTFLiteModelLoader()
	test.That(t, err, test.ShouldBeNil)
	tfliteStruct, err := loader.Load(tfliteModelPath)
	test.That(t, tfliteStruct, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeError, "cannot load model")
}

func TestLoadTFLiteStruct(t *testing.T) {
	loader := &TFLiteModelLoader{
		newModelFromFile:   modelLoader,
		newInterpreter:     goodInterpreterLoader,
		interpreterOptions: goodOptions,
		getInfo:            goodGetInfo,
	}

	tfStruct, err := loader.Load("random path")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, tfStruct, test.ShouldNotBeNil)
	test.That(t, tfStruct.model, test.ShouldNotBeNil)
	test.That(t, tfStruct.interpreter, test.ShouldNotBeNil)
	test.That(t, tfStruct.interpreterOptions, test.ShouldNotBeNil)

	tfStruct, err = loader.Load(badPath)
	test.That(t, err, test.ShouldBeError, errors.New("cannot load model"))
	test.That(t, tfStruct, test.ShouldBeNil)
}

func modelLoader(path string) *tflite.Model {
	if path == badPath {
		return nil
	}

	return &tflite.Model{}
}
