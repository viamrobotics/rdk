package mlmodel_test

import (
	"context"
	"testing"

	"github.com/mitchellh/mapstructure"
	"go.viam.com/test"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testMLModelServiceName  = "mlmodel1"
	testMLModelServiceName2 = "mlmodel2"
)

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
	reconfCount int
}

func (m *mockDetector) Infer(
	ctx context.Context,
	input map[string]interface{},
) (map[string]interface{}, error) {
	m.inferCount++
	// this is a possible form of what a detection tensor with 3 detection in 1 image would look like
	outputMap := make(map[string]interface{})
	outputMap["n_detections"] = []int32{3}
	outputMap["confidence_scores"] = [][]float32{{0.9084375, 0.7359375, 0.33984375}}
	outputMap["labels"] = [][]int32{{0, 0, 4}}
	outputMap["locations"] = [][][]float32{{
		{0.1, 0.4, 0.22, 0.4},
		{0.02, 0.22, 0.77, 0.90},
		{0.40, 0.50, 0.40, 0.50},
	}}
	return outputMap, nil
}

func (m *mockDetector) Metadata(ctx context.Context) (mlmodel.MLMetadata, error) {
	md := mlmodel.MLMetadata{
		ModelName:        "fake_detector",
		ModelType:        "object_detector",
		ModelDescription: "desc",
	}
	md.Inputs = []mlmodel.TensorInfo{
		{Name: "image", Description: "i0", DataType: "uint8", Shape: []int{300, 200}},
	}
	md.Outputs = []mlmodel.TensorInfo{
		{Name: "n_detections", Description: "o0", DataType: "int32", Shape: []int{1}},
		{Name: "confidence_scores", Description: "o1", DataType: "float32", Shape: []int{3, 1}},
		{
			Name:        "labels",
			Description: "o2",
			DataType:    "int32",
			Shape:       []int{3, 1},
			AssociatedFiles: []mlmodel.File{
				{
					Name:        "category_labels.txt",
					Description: "these labels represent types of plants",
					LabelType:   mlmodel.LabelTypeTensorValue,
				},
			},
		},
		{Name: "locations", Description: "o3", DataType: "float32", Shape: []int{4, 3, 1}},
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
	// decode the map[string]interface{} into a struct
	temp := struct {
		NDetections      []int32       `mapstructure:"n_detections"`
		ConfidenceScores [][]float32   `mapstructure:"confidence_scores"`
		Labels           [][]int32     `mapstructure:"labels"`
		Locations        [][][]float32 `mapstructure:"locations"`
	}{}
	err = mapstructure.Decode(result, &temp)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, temp.NDetections[0], test.ShouldEqual, 3)
	test.That(t, len(temp.ConfidenceScores[0]), test.ShouldEqual, 3)
	test.That(t, len(temp.Labels[0]), test.ShouldEqual, 3)
	test.That(t, temp.Locations[0][0], test.ShouldResemble, []float32{0.1, 0.4, 0.22, 0.4})
	// remove resource
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return "not an ml model", nil
	}
	svc, err = mlmodel.FromRobot(r, testMLModelServiceName2)
	test.That(t, err, test.ShouldBeError, mlmodel.NewUnimplementedInterfaceError("string"))
	test.That(t, svc, test.ShouldBeNil)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return nil, rutils.NewResourceNotFoundError(name)
	}

	svc, err = mlmodel.FromRobot(r, testMLModelServiceName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(mlmodel.Named(testMLModelServiceName)))
	test.That(t, svc, test.ShouldBeNil)
}
