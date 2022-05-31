package gantry_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/component/generic"
	viamgrpc "go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	componentpb "go.viam.com/rdk/proto/api/component/gantry/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
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

	var gantryPos []float64

	pos1 := []float64{1.0, 2.0, 3.0}
	len1 := []float64{2.0, 3.0, 4.0}
	injectGantry := &inject.Gantry{}
	injectGantry.GetPositionFunc = func(ctx context.Context) ([]float64, error) {
		return pos1, nil
	}
	injectGantry.MoveToPositionFunc = func(ctx context.Context, pos []float64, worldState *commonpb.WorldState) error {
		gantryPos = pos
		return nil
	}
	injectGantry.GetLengthsFunc = func(ctx context.Context) ([]float64, error) {
		return len1, nil
	}
	injectGantry.StopFunc = func(ctx context.Context) error {
		return errors.New("no stop")
	}

	pos2 := []float64{4.0, 5.0, 6.0}
	len2 := []float64{5.0, 6.0, 7.0}
	injectGantry2 := &inject.Gantry{}
	injectGantry2.GetPositionFunc = func(ctx context.Context) ([]float64, error) {
		return pos2, nil
	}
	injectGantry2.MoveToPositionFunc = func(ctx context.Context, pos []float64, worldState *commonpb.WorldState) error {
		gantryPos = pos
		return nil
	}
	injectGantry2.GetLengthsFunc = func(ctx context.Context) ([]float64, error) {
		return len2, nil
	}
	injectGantry2.StopFunc = func(ctx context.Context) error {
		return nil
	}

	gantrySvc, err := subtype.New(
		(map[resource.Name]interface{}{gantry.Named(testGantryName): injectGantry, gantry.Named(testGantryName2): injectGantry2}),
	)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(gantry.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, gantrySvc)

	injectGantry.DoFunc = generic.EchoFunc
	generic.RegisterService(rpcServer, gantrySvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = gantry.NewClient(cancelCtx, testGantryName, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working
	t.Run("gantry client 1", func(t *testing.T) {
		gantry1Client, err := gantry.NewClient(context.Background(), testGantryName, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		// Do
		resp, err := gantry1Client.Do(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		pos, err := gantry1Client.GetPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldResemble, pos1)

		err = gantry1Client.MoveToPosition(context.Background(), pos2, &commonpb.WorldState{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gantryPos, test.ShouldResemble, pos2)

		lens, err := gantry1Client.GetLengths(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, lens, test.ShouldResemble, len1)

		err = gantry1Client.Stop(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no stop")

		test.That(t, utils.TryClose(context.Background(), gantry1Client), test.ShouldBeNil)
	})

	t.Run("gantry client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, testGantryName2, logger)
		gantry2Client, ok := client.(gantry.Gantry)
		test.That(t, ok, test.ShouldBeTrue)

		pos, err := gantry2Client.GetPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldResemble, pos2)

		err = gantry2Client.Stop(context.Background())
		test.That(t, err, test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectGantry := &inject.Gantry{}

	gantrySvc, err := subtype.New(map[resource.Name]interface{}{gantry.Named(testGantryName): injectGantry})
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterGantryServiceServer(gServer, gantry.NewServer(gantrySvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := gantry.NewClient(ctx, testGantryName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := gantry.NewClient(ctx, testGantryName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}
