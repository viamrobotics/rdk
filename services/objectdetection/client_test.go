package objectdetection_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/objectdetection"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectODS := &inject.ObjectDetectionService{}
	m := map[resource.Name]interface{}{
		objectdetection.Name: injectODS,
	}
	svc, err := subtype.New(m)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(objectdetection.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = objectdetection.NewClient(cancelCtx, "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("working client", func(t *testing.T) {
		client, err := objectdetection.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		expSlice := []string{"client_test"}
		injectODS.GetDetectorsFunc = func(ctx context.Context) ([]string, error) {
			return expSlice, nil
		}

		names, err := client.GetDetectors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldResemble, expSlice)

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})
}
