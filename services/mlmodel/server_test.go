package mlmodel_test

import (
	"context"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	pb "go.viam.com/api/service/mlmodel/v1"
	"go.viam.com/test"
	vprotoutils "go.viam.com/utils/protoutils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/subtype"
)

func newServer(omMap map[resource.Name]interface{}) (pb.MLModelServiceServer, error) {
	omSvc, err := subtype.New(omMap)
	if err != nil {
		return nil, err
	}
	return mlmodel.NewServer(omSvc), nil
}

func TestServerNotFound(t *testing.T) {
	metadataRequest := &pb.MetadataRequest{
		Name: testMLModelServiceName,
	}
	omMap := map[resource.Name]interface{}{}
	server, err := newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.Metadata(context.Background(), metadataRequest)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:mlmodel/mlmodel1\" not found"))

	// set up the robot with something that is not an ml Model service
	omMap = map[resource.Name]interface{}{mlmodel.Named(testMLModelServiceName): "not ml model"}
	server, err = newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.Metadata(context.Background(), metadataRequest)
	test.That(t, err, test.ShouldBeError, mlmodel.NewUnimplementedInterfaceError("string"))
}

func TestServerMetadata(t *testing.T) {
	metadataRequest := &pb.MetadataRequest{
		Name: testMLModelServiceName,
	}

	mockSrv := &mockDetector{}
	omMap := map[resource.Name]interface{}{
		mlmodel.Named(testMLModelServiceName): mockSrv,
	}

	server, err := newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	resp, err := server.Metadata(context.Background(), metadataRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Metadata.GetName(), test.ShouldEqual, "fake_detector")
	test.That(t, resp.Metadata.GetType(), test.ShouldEqual, "object_detector")
	test.That(t, resp.Metadata.GetDescription(), test.ShouldEqual, "desc")
	test.That(t, len(resp.Metadata.GetInputInfo()), test.ShouldEqual, 1)
	test.That(t, len(resp.Metadata.GetOutputInfo()), test.ShouldEqual, 4)
	outInfo := resp.Metadata.GetOutputInfo()
	test.That(t, outInfo[0].GetName(), test.ShouldEqual, "n_detections")
	test.That(t, len(outInfo[0].GetAssociatedFiles()), test.ShouldEqual, 0)
	test.That(t, outInfo[2].GetName(), test.ShouldEqual, "labels")
	test.That(t, len(outInfo[2].GetAssociatedFiles()), test.ShouldEqual, 1)
	test.That(t, outInfo[2].GetAssociatedFiles()[0].GetName(), test.ShouldEqual, "category_labels.txt")
	test.That(t, outInfo[2].GetAssociatedFiles()[0].GetLabelType(), test.ShouldEqual, 1)
	test.That(t, outInfo[3].GetName(), test.ShouldEqual, "locations")
	test.That(t, outInfo[3].GetShape(), test.ShouldResemble, []int32{4, 3, 1})

	// Multiple Services names Valid
	omMap = map[resource.Name]interface{}{
		mlmodel.Named(testMLModelServiceName):  mockSrv,
		mlmodel.Named(testMLModelServiceName2): mockSrv,
	}
	server, err = newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	resp, err = server.Metadata(context.Background(), metadataRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Metadata.GetName(), test.ShouldEqual, "fake_detector")
	test.That(t, resp.Metadata.GetType(), test.ShouldEqual, "object_detector")
	test.That(t, resp.Metadata.GetDescription(), test.ShouldEqual, "desc")

	metadataRequest2 := &pb.MetadataRequest{
		Name: testMLModelServiceName2,
	}
	resp, err = server.Metadata(context.Background(), metadataRequest2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Metadata.GetName(), test.ShouldEqual, "fake_detector")
	test.That(t, resp.Metadata.GetType(), test.ShouldEqual, "object_detector")
	test.That(t, resp.Metadata.GetDescription(), test.ShouldEqual, "desc")
}

func TestServerInfer(t *testing.T) {
	inputData := map[string]interface{}{
		"image": [][]uint8{{10, 10, 255, 0, 0, 255, 255, 0, 100}},
	}
	inputProto, err := vprotoutils.StructToStructPb(inputData)
	test.That(t, err, test.ShouldBeNil)
	inferRequest := &pb.InferRequest{
		Name:      testMLModelServiceName,
		InputData: inputProto,
	}

	mockSrv := &mockDetector{}
	omMap := map[resource.Name]interface{}{
		mlmodel.Named(testMLModelServiceName): mockSrv,
	}

	server, err := newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	resp, err := server.Infer(context.Background(), inferRequest)
	test.That(t, err, test.ShouldBeNil)
	outMap := resp.OutputData.AsMap()
	test.That(t, len(outMap), test.ShouldEqual, 4)
	// decode the map[string]interface{} into a struct
	temp := struct {
		NDetections      []int32       `mapstructure:"n_detections"`
		ConfidenceScores [][]float32   `mapstructure:"confidence_scores"`
		Labels           [][]int32     `mapstructure:"labels"`
		Locations        [][][]float32 `mapstructure:"locations"`
	}{}
	err = mapstructure.Decode(outMap, &temp)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, temp.NDetections[0], test.ShouldEqual, 3)
	test.That(t, len(temp.ConfidenceScores[0]), test.ShouldEqual, 3)
	test.That(t, len(temp.Labels[0]), test.ShouldEqual, 3)
	test.That(t, temp.Locations[0][0], test.ShouldResemble, []float32{0.1, 0.4, 0.22, 0.4})
}
