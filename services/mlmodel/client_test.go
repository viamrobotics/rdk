package mlmodel_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	pb "go.viam.com/api/service/mlmodel/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	fakeModel := &mockDetector{}
	omMap := map[resource.Name]interface{}{
		mlmodel.Named(testMLModelServiceName): fakeModel,
	}
	svc, err := subtype.New(omMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(mlmodel.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)
	inputData := map[string]interface{}{
		"image": [][]uint8{{10, 10, 255, 0, 0, 255, 255, 0, 100}},
	}
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
		client := mlmodel.NewClientFromConn(context.Background(), conn, testMLModelServiceName, logger)
		// Infer Command
		result, err := client.Infer(context.Background(), inputData)
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
		result, err = client.Infer(context.Background(), nil)
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
		outInfo := meta.Outputs
		test.That(t, outInfo[0].Name, test.ShouldEqual, "n_detections")
		test.That(t, len(outInfo[0].AssociatedFiles), test.ShouldEqual, 0)
		test.That(t, outInfo[2].Name, test.ShouldEqual, "labels")
		test.That(t, len(outInfo[2].AssociatedFiles), test.ShouldEqual, 1)
		test.That(t, outInfo[2].AssociatedFiles[0].Name, test.ShouldEqual, "category_labels.txt")
		test.That(t, outInfo[2].AssociatedFiles[0].LabelType, test.ShouldEqual, mlmodel.LabelTypeTensorValue)
		test.That(t, outInfo[3].Name, test.ShouldEqual, "locations")
		test.That(t, len(outInfo[3].Shape), test.ShouldEqual, 3)
		test.That(t, outInfo[3].Shape, test.ShouldResemble, []int{4, 3, 1})

		// close the client
		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	fakeDetector := &mockDetector{}
	omMap := map[resource.Name]interface{}{
		mlmodel.Named(testMLModelServiceName): fakeDetector,
	}
	server, err := newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterMLModelServiceServer(gServer, server)

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := mlmodel.NewClientFromConn(ctx, conn1, testMLModelServiceName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := mlmodel.NewClientFromConn(ctx, conn2, testMLModelServiceName, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}
