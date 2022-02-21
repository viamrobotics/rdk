package objectmanipulation_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	viamgrpc "go.viam.com/rdk/grpc"
	servicepb "go.viam.com/rdk/proto/api/service/objectmanipulation/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/objectmanipulation"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectOMS := &inject.ObjectManipulationService{}
	omMap := map[resource.Name]interface{}{
		objectmanipulation.Name: injectOMS,
	}
	svc, err := subtype.New(omMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(objectmanipulation.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = objectmanipulation.NewClient(cancelCtx, "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working
	t.Run("object manipulation client 1", func(t *testing.T) {
		client, err := objectmanipulation.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		success := true
		injectOMS.DoGrabFunc = func(ctx context.Context, gripperName, armName, cameraName string, cameraPoint *r3.Vector) (bool, error) {
			return success, nil
		}

		result, err := client.DoGrab(context.Background(), "", "", "", &r3.Vector{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldEqual, success)

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})

	// broken
	t.Run("object manipulation client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, "", logger)
		client2, ok := client.(objectmanipulation.Service)
		test.That(t, ok, test.ShouldBeTrue)

		passedErr := errors.New("fake dograb error")
		injectOMS.DoGrabFunc = func(ctx context.Context, gripperName, armName, cameraName string, cameraPoint *r3.Vector) (bool, error) {
			return false, passedErr
		}

		resp, err := client2.DoGrab(context.Background(), "", "", "", &r3.Vector{})
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())
		test.That(t, resp, test.ShouldEqual, false)
		test.That(t, utils.TryClose(context.Background(), client2), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectOMS := &inject.ObjectManipulationService{}
	omMap := map[resource.Name]interface{}{
		objectmanipulation.Name: injectOMS,
	}
	server, err := newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	servicepb.RegisterObjectManipulationServiceServer(gServer, server)

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := objectmanipulation.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := objectmanipulation.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}
