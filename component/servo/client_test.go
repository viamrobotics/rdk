package servo_test

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

	"go.viam.com/rdk/component/servo"
	viamgrpc "go.viam.com/rdk/grpc"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()

	workingServo := &inject.Servo{}
	failingServo := &inject.Servo{}

	workingServo.MoveFunc = func(ctx context.Context, angle uint8) error {
		return nil
	}
	workingServo.CurrentFunc = func(ctx context.Context) (uint8, error) {
		return 20, nil
	}

	failingServo.MoveFunc = func(ctx context.Context, angle uint8) error {
		return errors.New("move failed")
	}
	failingServo.CurrentFunc = func(ctx context.Context) (uint8, error) {
		return 0, errors.New("current angle not readable")
	}

	resourceMap := map[resource.Name]interface{}{
		servo.Named("workingServo"): workingServo,
		servo.Named("failingServo"): failingServo,
	}
	servoSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)

	componentpb.RegisterServoServiceServer(gServer1, servo.NewServer(servoSvc))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = servo.NewClient(cancelCtx, "workingServo", listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	workingServoClient, err := servo.NewClient(context.Background(), "workingServo", listener1.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)

	t.Run("client tests for working servo", func(t *testing.T) {
		err := workingServoClient.Move(context.Background(), 20)
		test.That(t, err, test.ShouldBeNil)

		currentDeg, err := workingServoClient.GetPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, currentDeg, test.ShouldEqual, 20)
	})

	failingServoClient, err := servo.NewClient(context.Background(), "failingServo", listener1.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)

	t.Run("client tests for failing servo", func(t *testing.T) {
		err = failingServoClient.Move(context.Background(), 20)
		test.That(t, err, test.ShouldNotBeNil)

		_, err := failingServoClient.GetPosition(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("dialed client tests for working servo", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		workingServoDialedClient := servo.NewClientFromConn(context.Background(), conn, "workingServo", logger)

		err = workingServoDialedClient.Move(context.Background(), 20)
		test.That(t, err, test.ShouldBeNil)

		currentDeg, err := workingServoDialedClient.GetPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, currentDeg, test.ShouldEqual, 20)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	test.That(t, utils.TryClose(context.Background(), workingServoClient), test.ShouldBeNil)
	test.That(t, utils.TryClose(context.Background(), failingServoClient), test.ShouldBeNil)
}
