package encoder_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/encoder/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/generic"
	viamgrpc "go.viam.com/rdk/grpc"
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

	workingEncoder := &inject.Encoder{}
	failingEncoder := &inject.Encoder{}

	var actualExtra map[string]interface{}

	workingEncoder.ResetPositionFunc = func(ctx context.Context, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	workingEncoder.GetPositionFunc = func(
		ctx context.Context,
		positionType *encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error) {
		actualExtra = extra
		return 42.0, encoder.PositionTypeUNSPECIFIED, nil
	}
	workingEncoder.GetPropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error) {
		actualExtra = extra
		return map[encoder.Feature]bool{
			encoder.TicksCountSupported:   true,
			encoder.AngleDegreesSupported: false,
		}, nil
	}

	failingEncoder.ResetPositionFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errors.New("set to zero failed")
	}
	failingEncoder.GetPositionFunc = func(
		ctx context.Context,
		positionType *encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error) {
		return 0, encoder.PositionTypeUNSPECIFIED, errors.New("position unavailable")
	}
	failingEncoder.GetPropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error) {
		return nil, errors.New("get properties failed")
	}

	resourceMap := map[resource.Name]interface{}{
		encoder.Named(testEncoderName): workingEncoder,
		encoder.Named(failEncoderName): failingEncoder,
	}
	encoderSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(encoder.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, encoderSvc)

	workingEncoder.DoFunc = generic.EchoFunc
	generic.RegisterService(rpcServer, encoderSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	workingEncoderClient := encoder.NewClientFromConn(context.Background(), conn, testEncoderName, logger)

	t.Run("client tests for working encoder", func(t *testing.T) {
		// DoCommand
		resp, err := workingEncoderClient.DoCommand(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		err = workingEncoderClient.ResetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		pos, positionType, err := workingEncoderClient.GetPosition(
			context.Background(),
			nil,
			map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 42.0)
		test.That(t, positionType, test.ShouldEqual, pb.PositionType_POSITION_TYPE_UNSPECIFIED)

		test.That(t, actualExtra, test.ShouldResemble, map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}})

		test.That(t, utils.TryClose(context.Background(), workingEncoderClient), test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	conn, err = viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	failingEncoderClient := encoder.NewClientFromConn(context.Background(), conn, failEncoderName, logger)

	t.Run("client tests for failing encoder", func(t *testing.T) {
		err = failingEncoderClient.ResetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)

		pos, _, err := failingEncoderClient.GetPosition(context.Background(), encoder.PositionTypeUNSPECIFIED.Enum(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		test.That(t, utils.TryClose(context.Background(), failingEncoderClient), test.ShouldBeNil)
	})

	t.Run("dialed client tests for working encoder", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingEncoderDialedClient := encoder.NewClientFromConn(context.Background(), conn, testEncoderName, logger)

		pos, _, err := workingEncoderDialedClient.GetPosition(context.Background(), encoder.PositionTypeUNSPECIFIED.Enum(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 42.0)

		err = workingEncoderDialedClient.ResetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, utils.TryClose(context.Background(), workingEncoderDialedClient), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client tests for failing encoder", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingEncoderDialedClient := encoder.NewClientFromConn(context.Background(), conn, failEncoderName, logger)

		test.That(t, utils.TryClose(context.Background(), failingEncoderDialedClient), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectEncoder := &inject.Encoder{}

	encoderSvc, err := subtype.New(map[resource.Name]interface{}{encoder.Named(testEncoderName): injectEncoder})
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterEncoderServiceServer(gServer, encoder.NewServer(encoderSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := encoder.NewClientFromConn(ctx, conn1, testEncoderName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := encoder.NewClientFromConn(ctx, conn2, testEncoderName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}
