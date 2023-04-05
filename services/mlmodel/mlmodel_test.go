package mlmodel_test

import (
	"context"
	"testing"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
)

const testMLModelServiceName = "mlmodel1"

func setupInjectRobot() (*inject.Robot, *mockDetector) {
	svc1 := &mockDetector{}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return svc1, nil
	}
	return r, svc1
}

type mockDetector struct {
	mlmodel.Service
	inferCount  int
	name        string
	reconfCount int
	cmd         map[string]interface{}
}

func (m *mockDetector) Infer(
	ctx context.Context,
	input map[string]interface{},
) (map[string]interface{}, error) {
	m.inferCount++
	// this is a possible form of what a detection tensor with 3 detection in 1 image would look like
	outputMap := make(map[string]interface{})
	outputMap["n_detections"] = []int32{3}
	outputMap["confidence_scores"] = [][]float32{[]float32{0.9084375, 0.7359375, 0.33984375}}
	outputMap["labels"] = [][]int32{[]int32{0, 0, 4}}
	outputMap["locations"] = [][][]float32{[][]float32{
		[]float32{0.1, 0.4, 0.22, 0.4},
		[]float32{0.02, 0.22, 0.77, 0.90},
		[]float32{0.40, 0.50, 0.40, 0.50},
	}}
	return outputMap, nil
}

func (m *mockDetector) Metadata(ctx context.Context) (mlmodel.MLMetadata, error) {
	md := mlmodel.MLMetadata{
		ModelName:        "fake_detector",
		ModelType:        "object_detector",
		ModelDescription: "this fake detector always returns the same 3 detections",
	}
	md.Inputs = []mlmodel.TensorInfo{
		{Name: "image", Description: "i0", DataType: "uint8", NDim: 2},
	}
	md.Outputs = []mlmodel.TensorInfo{
		{Name: "n_detections", Description: "o0", DataType: "int32", NDim: 1},
		{Name: "confidence_scores", Description: "o1", DataType: "float32", NDim: 2},
		{
			Name:        "labels",
			Description: "o2",
			DataType:    "int32",
			NDim:        2,
			AssociatedFiles: []mlmodel.File{
				{
					Name:        "category_labels.txt",
					Description: "these labels represent types of plants",
					LabelType:   mlmodel.LabelTypeTensorValue,
				},
			},
		},
		{Name: "locations", Description: "o3", DataType: "float32", NDim: 3},
	}
	return md, nil
}

func (m *mockDetector) Close(ctx context.Context) error {
	m.reconfCount++
	return nil
}

func TestFromRobot(t *testing.T) {
	r, svc1 := setupInjectRobot()

	svc, err := mlmodel.FromRobot(r, testMLModelServiceName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)
	testIn := make(map[string]interface{})
	result, err := svc.Infer(context.Background(), testIn)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(result), test.ShouldEqual, 4)
	test.That(t, svc1.inferCount, test.ShouldEqual, 1)
}
