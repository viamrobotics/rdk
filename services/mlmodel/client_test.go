package mlmodel_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"gorgonia.org/tensor"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testMLModelServiceName  = "mlmodel1"
	testMLModelServiceName2 = "mlmodel2"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	fakeModel := inject.NewMLModelService(testMLModelServiceName)
	fakeModel.MetadataFunc = injectedMetadataFunc
	fakeModel.InferFunc = injectedInferFunc
	resources := map[resource.Name]mlmodel.Service{
		mlmodel.Named(testMLModelServiceName): fakeModel,
	}
	svc, err := resource.NewAPIResourceCollection(mlmodel.API, resources)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[mlmodel.Service](mlmodel.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, svc), test.ShouldBeNil)
	inputTensors := mlmodel.Tensors{}
	inputTensors["image"] = tensor.New(tensor.WithShape(3, 3), tensor.WithBacking([]uint8{10, 10, 255, 0, 0, 255, 255, 0, 100}))
	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// context canceled
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working
	t.Run("ml model client", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := mlmodel.NewClientFromConn(context.Background(), conn, "", mlmodel.Named(testMLModelServiceName), logger)
		test.That(t, err, test.ShouldBeNil)
		// Infer Command
		_, result, err := client.Infer(context.Background(), inputTensors, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(result), test.ShouldEqual, 4)
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
		// nil data should work too
		_, result, err = client.Infer(context.Background(), nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(result), test.ShouldEqual, 4)
		// Metadata Command
		meta, err := client.Metadata(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, meta.ModelName, test.ShouldEqual, "fake_detector")
		test.That(t, meta.ModelType, test.ShouldEqual, "object_detector")
		test.That(t, meta.ModelDescription, test.ShouldEqual, "desc")
		t.Logf("inputs: %v", meta.Inputs)
		test.That(t, len(meta.Inputs), test.ShouldEqual, 1)
		test.That(t, len(meta.Outputs), test.ShouldEqual, 4)
		test.That(t, meta.Inputs[0].Shape, test.ShouldResemble, []int{300, 200})
		outInfo := meta.Outputs
		test.That(t, outInfo[0].Name, test.ShouldEqual, "n_detections")
		test.That(t, len(outInfo[0].AssociatedFiles), test.ShouldEqual, 0)
		test.That(t, outInfo[2].Name, test.ShouldEqual, "labels")
		test.That(t, len(outInfo[2].AssociatedFiles), test.ShouldEqual, 1)
		test.That(t, outInfo[2].AssociatedFiles[0].Name, test.ShouldEqual, "category_labels.txt")
		test.That(t, outInfo[2].AssociatedFiles[0].LabelType, test.ShouldEqual, mlmodel.LabelTypeTensorValue)
		test.That(t, outInfo[3].Name, test.ShouldEqual, "locations")
		test.That(t, outInfo[3].Shape, test.ShouldResemble, []int{4, 3, 1})

		// close the client
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
