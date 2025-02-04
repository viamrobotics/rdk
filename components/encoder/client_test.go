package encoder_test

import (
	"context"
	"net"
	"testing"

	pb "go.viam.com/api/component/encoder/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/encoder"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testEncoderName = "encoder1"
	failEncoderName = "encoder2"
	fakeEncoderName = "encoder3"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
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
	workingEncoder.PositionFunc = func(
		ctx context.Context,
		positionType encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error) {
		actualExtra = extra
		return 42.0, encoder.PositionTypeUnspecified, nil
	}
	workingEncoder.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (encoder.Properties, error) {
		actualExtra = extra
		return encoder.Properties{
			TicksCountSupported:   true,
			AngleDegreesSupported: false,
		}, nil
	}

	failingEncoder.ResetPositionFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errSetToZeroFailed
	}
	failingEncoder.PositionFunc = func(
		ctx context.Context,
		positionType encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error) {
		return 0, encoder.PositionTypeUnspecified, errPositionUnavailable
	}
	failingEncoder.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (encoder.Properties, error) {
		return encoder.Properties{}, errGetPropertiesFailed
	}

	resourceMap := map[resource.Name]encoder.Encoder{
		encoder.Named(testEncoderName): workingEncoder,
		encoder.Named(failEncoderName): failingEncoder,
	}
	encoderSvc, err := resource.NewAPIResourceCollection(encoder.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[encoder.Encoder](encoder.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, encoderSvc), test.ShouldBeNil)

	workingEncoder.DoFunc = testutils.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	workingEncoderClient, err := encoder.NewClientFromConn(context.Background(), conn, "", encoder.Named(testEncoderName), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client tests for working encoder", func(t *testing.T) {
		// DoCommand
		resp, err := workingEncoderClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		err = workingEncoderClient.ResetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		pos, positionType, err := workingEncoderClient.Position(
			context.Background(),
			encoder.PositionTypeUnspecified,
			map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 42.0)
		test.That(t, positionType, test.ShouldEqual, pb.PositionType_POSITION_TYPE_UNSPECIFIED)

		test.That(t, actualExtra, test.ShouldResemble, map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}})

		test.That(t, workingEncoderClient.Close(context.Background()), test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	conn, err = viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	failingEncoderClient, err := encoder.NewClientFromConn(context.Background(), conn, "", encoder.Named(failEncoderName), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client tests for failing encoder", func(t *testing.T) {
		err = failingEncoderClient.ResetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errSetToZeroFailed.Error())

		pos, _, err := failingEncoderClient.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errPositionUnavailable.Error())
		test.That(t, pos, test.ShouldEqual, 0.0)

		test.That(t, failingEncoderClient.Close(context.Background()), test.ShouldBeNil)
	})

	t.Run("dialed client tests for working encoder", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingEncoderDialedClient, err := encoder.NewClientFromConn(context.Background(), conn, "", encoder.Named(testEncoderName), logger)
		test.That(t, err, test.ShouldBeNil)

		pos, _, err := workingEncoderDialedClient.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 42.0)

		err = workingEncoderDialedClient.ResetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, workingEncoderDialedClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client tests for failing encoder", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingEncoderDialedClient, err := encoder.NewClientFromConn(context.Background(), conn, "", encoder.Named(failEncoderName), logger)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, failingEncoderDialedClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	test.That(t, conn.Close(), test.ShouldBeNil)
}
