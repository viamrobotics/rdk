package motor_test

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

	"go.viam.com/rdk/component/motor"
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
	gServer1 := grpc.NewServer()

	workingMotor := &inject.Motor{}
	failingMotor := &inject.Motor{}

	workingMotor.SetPowerFunc = func(ctx context.Context, powerPct float64) error {
		return nil
	}
	workingMotor.GoForFunc = func(ctx context.Context, rpm, rotations float64) error {
		return nil
	}
	workingMotor.GoToFunc = func(ctx context.Context, rpm, position float64) error {
		return nil
	}
	workingMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64) error {
		return nil
	}
	workingMotor.PositionFunc = func(ctx context.Context) (float64, error) {
		return 42.0, nil
	}
	workingMotor.GetFeaturesFunc = func(ctx context.Context) (map[motor.Feature]bool, error) {
		return map[motor.Feature]bool{
			motor.PositionReporting: true,
		}, nil
	}
	workingMotor.StopFunc = func(ctx context.Context) error {
		return nil
	}
	workingMotor.IsPoweredFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}

	failingMotor.SetPowerFunc = func(ctx context.Context, powerPct float64) error {
		return errors.New("set power failed")
	}
	failingMotor.GoForFunc = func(ctx context.Context, rpm, rotations float64) error {
		return errors.New("go for failed")
	}
	failingMotor.GoToFunc = func(ctx context.Context, rpm, position float64) error {
		return errors.New("go to failed")
	}
	failingMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64) error {
		return errors.New("set to zero failed")
	}
	failingMotor.PositionFunc = func(ctx context.Context) (float64, error) {
		return 0, errors.New("position unavailable")
	}
	failingMotor.GetFeaturesFunc = func(ctx context.Context) (map[motor.Feature]bool, error) {
		return nil, errors.New("supported features unavailable")
	}
	failingMotor.StopFunc = func(ctx context.Context) error {
		return errors.New("stop failed")
	}
	failingMotor.IsPoweredFunc = func(ctx context.Context) (bool, error) {
		return false, errors.New("is on unavailable")
	}

	resourceMap := map[resource.Name]interface{}{
		motor.Named("workingMotor"): workingMotor,
		motor.Named("failingMotor"): failingMotor,
	}
	motorSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)

	componentpb.RegisterMotorServiceServer(gServer1, motor.NewServer(motorSvc))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = motor.NewClient(cancelCtx, "workingMotor", listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	workingMotorClient, err := motor.NewClient(context.Background(), "workingMotor", listener1.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)

	t.Run("client tests for working motor", func(t *testing.T) {
		err := workingMotorClient.SetPower(context.Background(), 42.0)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorClient.GoFor(context.Background(), 42.0, 42.0)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorClient.GoTo(context.Background(), 42.0, 42.0)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorClient.ResetZeroPosition(context.Background(), 0.5)
		test.That(t, err, test.ShouldBeNil)

		pos, err := workingMotorClient.Position(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 42.0)

		features, err := workingMotorClient.GetFeatures(context.Background())
		test.That(t, features[motor.PositionReporting], test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorClient.Stop(context.Background())
		test.That(t, err, test.ShouldBeNil)

		isOn, err := workingMotorClient.IsPowered(context.Background())
		test.That(t, isOn, test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)
	})

	failingMotorClient, err := motor.NewClient(context.Background(), "failingMotor", listener1.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)

	t.Run("client tests for failing motor", func(t *testing.T) {
		err := failingMotorClient.GoTo(context.Background(), 42.0, 42.0)
		test.That(t, err, test.ShouldNotBeNil)

		err = failingMotorClient.ResetZeroPosition(context.Background(), 0.5)
		test.That(t, err, test.ShouldNotBeNil)

		pos, err := failingMotorClient.Position(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		err = failingMotorClient.SetPower(context.Background(), 42.0)
		test.That(t, err, test.ShouldNotBeNil)

		err = failingMotorClient.GoFor(context.Background(), 42.0, 42.0)
		test.That(t, err, test.ShouldNotBeNil)

		features, err := failingMotorClient.GetFeatures(context.Background())
		test.That(t, features, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		isOn, err := failingMotorClient.IsPowered(context.Background())
		test.That(t, isOn, test.ShouldBeFalse)
		test.That(t, err, test.ShouldNotBeNil)

		err = failingMotorClient.Stop(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("dialed client tests for working motor", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		workingMotorDialedClient := motor.NewClientFromConn(context.Background(), conn, "workingMotor", logger)

		pos, err := workingMotorDialedClient.Position(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 42.0)

		features, err := workingMotorDialedClient.GetFeatures(context.Background())
		test.That(t, features[motor.PositionReporting], test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorDialedClient.GoTo(context.Background(), 42.0, 42.0)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorDialedClient.ResetZeroPosition(context.Background(), 0.5)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorDialedClient.Stop(context.Background())
		test.That(t, err, test.ShouldBeNil)

		isOn, err := workingMotorDialedClient.IsPowered(context.Background())
		test.That(t, isOn, test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("dialed client tests for failing motor", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		failingMotorDialedClient := motor.NewClientFromConn(context.Background(), conn, "failingMotor", logger)

		err = failingMotorDialedClient.SetPower(context.Background(), 39.2)
		test.That(t, err, test.ShouldNotBeNil)

		features, err := failingMotorDialedClient.GetFeatures(context.Background())
		test.That(t, features, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		isOn, err := failingMotorDialedClient.IsPowered(context.Background())
		test.That(t, isOn, test.ShouldBeFalse)
		test.That(t, err, test.ShouldNotBeNil)
	})

	test.That(t, utils.TryClose(context.Background(), workingMotorClient), test.ShouldBeNil)
	test.That(t, utils.TryClose(context.Background(), failingMotorClient), test.ShouldBeNil)
}
