package inference

import (
	"path/filepath"
	"runtime"
	"testing"

	tflite "github.com/mattn/go-tflite"
	"go.viam.com/test"
)

const badPath string = "bad path"

var (
	_, b, _, _ = runtime.Caller(0)
	basePath   = filepath.Dir(b)
)

type fakeInterpreter struct {
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
	return 1
}

func (fI fakeInterpreter) GetInputTensorCount() int {
	return 1
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

// TestLoadModel uses a real tflite model to test loading.
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
	arr0 := []float32{
		0.37555635, 0.09173261, 0.055961493, 0.053198617, 0.052616555, 0.044857822,
		0.04230374, 0.03936374, 0.036786992, 0.0354401,
	}
	arr1 := []float32{
		0.051015913, -0.0089687705, 0.98429424, 1.0008209, 0.18583027, 0.16296768,
		0.88490105, 0.8825035, 0.45570838, 0.07853904, 0.97085583, 0.9384842, 0.12304267, 0.12137526,
		0.65284467, 0.9064091, 0.051521003, 0.25793242, 0.76292396, 0.79288113, 0.6640951, 0.033264905,
		0.9957016, 0.9674016, -0.050027713, 0.018562555, 0.44127244, 1.0558276, 0.21212798, 0.74206173,
		0.8837902, 0.9681196, 0.018948674, 0.13732445, 0.30327052, 0.74848455, 0.12821937, -0.12527809,
		0.92030644, 0.6850277,
	}
	arr2 := []float32{27, 27, 27, 27, 27, 64, 27, 0, 37, 27}
	outTensors, err := tfliteStruct.Infer(buf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outTensors, test.ShouldNotBeNil)
	test.That(t, len(outTensors), test.ShouldEqual, 4)
	test.That(t, outTensors[0].([]float32), test.ShouldResemble, arr0)
	test.That(t, outTensors[1].([]float32), test.ShouldResemble, arr1)
	test.That(t, outTensors[2].([]float32), test.ShouldResemble, []float32{10})
	test.That(t, outTensors[3].([]float32), test.ShouldResemble, arr2)

	meta, err := tfliteStruct.GetMetadata()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, meta, test.ShouldNotBeNil)
	test.That(t, meta.SubgraphMetadata[0].OutputTensorGroups[0].TensorNames, test.ShouldResemble, []string{"location", "category", "score"})
	tfliteStruct.Close()
}

func TestLoadRealBadPath(t *testing.T) {
	tfliteModelPath := basePath + "/testing_files/does_not_exist.tflite"
	loader, err := NewDefaultTFLiteModelLoader()
	test.That(t, err, test.ShouldBeNil)
	tfliteStruct, err := loader.Load(tfliteModelPath)
	test.That(t, tfliteStruct, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeError, FailedToLoadError("model"))
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
	test.That(t, err, test.ShouldBeError, FailedToLoadError("model"))
	test.That(t, tfStruct, test.ShouldBeNil)
}

func modelLoader(path string) *tflite.Model {
	if path == badPath {
		return nil
	}

	return &tflite.Model{}
}
