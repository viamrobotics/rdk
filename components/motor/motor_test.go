package motor_test

import (
	"context"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	pb "go.viam.com/api/component/motor/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testMotorName    = "motor1"
	testMotorName2   = "motor2"
	failMotorName    = "motor3"
	fakeMotorName    = "motor4"
	missingMotorName = "motor5"
)

func TestStatusValid(t *testing.T) {
	status := &pb.Status{IsPowered: true, Position: 7.7, IsMoving: true}
	newStruct, err := protoutils.StructToStructPb(status)
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		newStruct.AsMap(),
		test.ShouldResemble,
		map[string]interface{}{"is_powered": true, "position": 7.7, "is_moving": true},
	)

	convMap := &pb.Status{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(newStruct.AsMap())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, status)

	status = &pb.Status{Position: 7.7}
	newStruct, err = protoutils.StructToStructPb(status)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newStruct.AsMap(), test.ShouldResemble, map[string]interface{}{"position": 7.7})

	convMap = &pb.Status{}
	decoder, err = mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(newStruct.AsMap())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, status)
}

func TestCreateStatus(t *testing.T) {
	status := &pb.Status{IsPowered: true, Position: 7.7, IsMoving: true}

	injectMotor := &inject.LocalMotor{}
	injectMotor.IsPoweredFunc = func(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
		return status.IsPowered, 1.0, nil
	}
	injectMotor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
		return map[motor.Feature]bool{motor.PositionReporting: true}, nil
	}
	injectMotor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return status.Position, nil
	}
	injectMotor.IsMovingFunc = func(context.Context) (bool, error) {
		return true, nil
	}

	t.Run("working", func(t *testing.T) {
		status1, err := motor.CreateStatus(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)

		resourceSubtype, ok, err := resource.LookupSubtypeRegistration[motor.Motor](motor.Subtype)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ok, test.ShouldBeTrue)
		status2, err := resourceSubtype.Status(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status2, test.ShouldResemble, status)
	})

	t.Run("not moving", func(t *testing.T) {
		injectMotor.IsMovingFunc = func(context.Context) (bool, error) {
			return false, nil
		}

		status2 := &pb.Status{IsPowered: true, Position: 7.7, IsMoving: false}
		status1, err := motor.CreateStatus(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status2)
	})

	t.Run("fail on Position", func(t *testing.T) {
		errFail := errors.New("can't get position")
		injectMotor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return 0, errFail
		}
		_, err := motor.CreateStatus(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeError, errFail)
	})

	t.Run("position not supported", func(t *testing.T) {
		injectMotor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{motor.PositionReporting: false}, nil
		}

		status1, err := motor.CreateStatus(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, &pb.Status{IsPowered: true})
	})

	t.Run("fail on Properties", func(t *testing.T) {
		errFail := errors.New("can't get features")
		injectMotor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return nil, errFail
		}
		_, err := motor.CreateStatus(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeError, errFail)
	})

	t.Run("fail on IsPowered", func(t *testing.T) {
		errFail := errors.New("can't get is powered")
		injectMotor.IsPoweredFunc = func(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
			return false, 0.0, errFail
		}
		_, err := motor.CreateStatus(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeError, errFail)
	})
}
