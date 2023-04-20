package gantry_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/gantry"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
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
	var extra1 map[string]interface{}
	injectGantry := &inject.Gantry{}
	injectGantry.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		extra1 = extra
		return pos1, nil
	}
	injectGantry.MoveToPositionFunc = func(ctx context.Context, pos []float64, extra map[string]interface{}) error {
		gantryPos = pos
		extra1 = extra
		return nil
	}
	injectGantry.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		extra1 = extra
		return len1, nil
	}
	injectGantry.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		extra1 = extra
		return errors.New("no stop")
	}

	pos2 := []float64{4.0, 5.0, 6.0}
	len2 := []float64{5.0, 6.0, 7.0}
	var extra2 map[string]interface{}
	injectGantry2 := &inject.Gantry{}
	injectGantry2.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		extra2 = extra
		return pos2, nil
	}
	injectGantry2.MoveToPositionFunc = func(ctx context.Context, pos []float64, extra map[string]interface{}) error {
		gantryPos = pos
		extra2 = extra
		return nil
	}
	injectGantry2.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		extra2 = extra
		return len2, nil
	}
	injectGantry2.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		extra2 = extra
		return nil
	}

	gantrySvc, err := resource.NewSubtypeCollection(
		gantry.Subtype,
		(map[resource.Name]gantry.Gantry{gantry.Named(testGantryName): injectGantry, gantry.Named(testGantryName2): injectGantry2}),
	)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype, ok, err := registry.ResourceSubtypeLookup[gantry.Gantry](gantry.Subtype)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceSubtype.RegisterRPCService(context.Background(), rpcServer, gantrySvc), test.ShouldBeNil)

	injectGantry.DoFunc = testutils.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working
	t.Run("gantry client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		gantry1Client, err := gantry.NewClientFromConn(context.Background(), conn, gantry.Named(testGantryName), logger)
		test.That(t, err, test.ShouldBeNil)

		// DoCommand
		resp, err := gantry1Client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		pos, err := gantry1Client.Position(context.Background(), map[string]interface{}{"foo": 123, "bar": "234"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldResemble, pos1)
		test.That(t, extra1, test.ShouldResemble, map[string]interface{}{"foo": 123., "bar": "234"})

		err = gantry1Client.MoveToPosition(context.Background(), pos2, map[string]interface{}{"foo": 234, "bar": "345"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gantryPos, test.ShouldResemble, pos2)
		test.That(t, extra1, test.ShouldResemble, map[string]interface{}{"foo": 234., "bar": "345"})

		lens, err := gantry1Client.Lengths(context.Background(), map[string]interface{}{"foo": 345, "bar": "456"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, lens, test.ShouldResemble, len1)
		test.That(t, extra1, test.ShouldResemble, map[string]interface{}{"foo": 345., "bar": "456"})

		err = gantry1Client.Stop(context.Background(), map[string]interface{}{"foo": 456, "bar": "567"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no stop")
		test.That(t, extra1, test.ShouldResemble, map[string]interface{}{"foo": 456., "bar": "567"})

		test.That(t, gantry1Client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("gantry client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := resourceSubtype.RPCClient(context.Background(), conn, gantry.Named(testGantryName2), logger)
		test.That(t, err, test.ShouldBeNil)
		gantry2Client, ok := client.(gantry.Gantry)
		test.That(t, ok, test.ShouldBeTrue)

		pos, err := gantry2Client.Position(context.Background(), map[string]interface{}{"foo": "123", "bar": 234})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldResemble, pos2)
		test.That(t, extra2, test.ShouldResemble, map[string]interface{}{"foo": "123", "bar": 234.})

		err = gantry2Client.Stop(context.Background(), map[string]interface{}{"foo": "234", "bar": 345})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extra2, test.ShouldResemble, map[string]interface{}{"foo": "234", "bar": 345.})

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
