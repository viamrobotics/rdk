// Package wheeledodometry implements an odometery estimate from an encoder wheeled base
package wheeledodometry

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
	"go.viam.com/utils"
)

const (
	leftMotorName  = "left"
	rightMotorName = "right"
	baseName       = "base"
	testSensorName = "name"
)

type positions struct {
	mu       sync.Mutex
	leftPos  float64
	rightPos float64
}

var position = positions{
	leftPos:  0.0,
	rightPos: 0.0,
}

func createFakeMotor(dir bool) motor.Motor {
	return &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
			return motor.Properties{PositionReporting: true}, nil
		},
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			position.mu.Lock()
			defer position.mu.Unlock()
			if dir {
				return position.leftPos, nil
			}
			return position.rightPos, nil
		},
		IsMovingFunc: func(ctx context.Context) (bool, error) {
			position.mu.Lock()
			defer position.mu.Unlock()
			if dir && math.Abs(position.leftPos) > 0 {
				return true, nil
			}
			if !dir && math.Abs(position.rightPos) > 0 {
				return true, nil
			}
			return false, nil
		},
		ResetZeroPositionFunc: func(ctx context.Context, offset float64, extra map[string]interface{}) error {
			position.mu.Lock()
			defer position.mu.Unlock()
			position.leftPos = 0
			position.rightPos = 0
			return nil
		},
	}
}

func createFakeBase() base.Base {
	return &inject.Base{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
			return base.Properties{WheelCircumferenceMeters: 0.2, WidthMeters: 0.2, TurningRadiusMeters: 0}, nil
		},
	}
}

func setPositions(left, right float64) {
	position.mu.Lock()
	defer position.mu.Unlock()
	position.leftPos += left
	position.rightPos += right
}

func TestNewWheeledOdometry(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	deps := make(resource.Dependencies)
	deps[base.Named(baseName)] = createFakeBase()
	deps[motor.Named(leftMotorName)] = createFakeMotor(true)
	deps[motor.Named(rightMotorName)] = createFakeMotor(false)

	fakecfg := resource.Config{
		Name: testSensorName,
		ConvertedAttributes: &Config{
			LeftMotors:        []string{leftMotorName},
			RightMotors:       []string{rightMotorName},
			Base:              baseName,
			TimeIntervalMSecs: 500,
		},
	}
	fakeSensor, err := newWheeledOdometry(ctx, deps, fakecfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, ok := fakeSensor.(*odometry)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestValidateConfig(t *testing.T) {
	cfg := Config{
		LeftMotors:        []string{leftMotorName},
		RightMotors:       []string{rightMotorName},
		Base:              "",
		TimeIntervalMSecs: 500,
	}

	deps, err := cfg.Validate("path")
	expectedErr := utils.NewConfigValidationFieldRequiredError("path", "base")
	test.That(t, err, test.ShouldBeError, expectedErr)
	test.That(t, deps, test.ShouldBeEmpty)

	cfg = Config{
		LeftMotors:        []string{},
		RightMotors:       []string{rightMotorName},
		Base:              baseName,
		TimeIntervalMSecs: 500,
	}

	deps, err = cfg.Validate("path")
	expectedErr = utils.NewConfigValidationFieldRequiredError("path", "left motors")
	test.That(t, err, test.ShouldBeError, expectedErr)
	test.That(t, deps, test.ShouldBeEmpty)

	cfg = Config{
		LeftMotors:        []string{leftMotorName},
		RightMotors:       []string{},
		Base:              baseName,
		TimeIntervalMSecs: 500,
	}

	deps, err = cfg.Validate("path")
	expectedErr = utils.NewConfigValidationFieldRequiredError("path", "right motors")
	test.That(t, err, test.ShouldBeError, expectedErr)
	test.That(t, deps, test.ShouldBeEmpty)

	cfg = Config{
		LeftMotors:        []string{leftMotorName, leftMotorName},
		RightMotors:       []string{rightMotorName},
		Base:              baseName,
		TimeIntervalMSecs: 500,
	}

	deps, err = cfg.Validate("path")
	expectedErr = errors.New("mismatch number of left and right motors")
	test.That(t, err, test.ShouldBeError, expectedErr)
	test.That(t, deps, test.ShouldBeEmpty)

	cfg = Config{
		LeftMotors:        []string{leftMotorName, leftMotorName},
		RightMotors:       []string{rightMotorName, rightMotorName},
		Base:              baseName,
		TimeIntervalMSecs: 500,
	}

	deps, err = cfg.Validate("path")
	expectedErr = errors.New("wheeled odometry only supports one left and right motor each")
	test.That(t, err, test.ShouldBeError, expectedErr)
	test.That(t, deps, test.ShouldBeEmpty)
}

func TestSpin(t *testing.T) {
	left := createFakeMotor(true)
	right := createFakeMotor(false)
	ctx := context.Background()
	_ = left.ResetZeroPosition(ctx, 0, nil)
	_ = right.ResetZeroPosition(ctx, 0, nil)

	od := &odometry{
		lastLeftPos:        0,
		lastRightPos:       0,
		baseWidth:          1,
		wheelCircumference: 1,
		timeIntervalMSecs:  500,
	}
	od.motors = append(od.motors, motorPair{left, right})
	od.trackPosition(context.Background())

	// turn 90 degrees
	setPositions(-1*(math.Pi/4), 1*(math.Pi/4))
	// sleep for slightly longer than time interval to ensure trackPosition has enough time to run
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, _ := od.Position(ctx, nil)
	or, err := od.Orientation(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 90, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 0, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 0, 0.1)

	// turn negative 180 degrees
	setPositions(1*(math.Pi/2), -1*(math.Pi/2))
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, _ = od.Position(ctx, nil)
	or, err = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 270, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 0, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 0, 0.1)

	// turn another 360 degrees
	setPositions(-1*math.Pi, 1*math.Pi)
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, _ = od.Position(ctx, nil)
	or, err = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 270, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 0, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 0, 0.1)
}

func TestMoveStraight(t *testing.T) {
	left := createFakeMotor(true)
	right := createFakeMotor(false)
	ctx := context.Background()
	_ = left.ResetZeroPosition(ctx, 0, nil)
	_ = right.ResetZeroPosition(ctx, 0, nil)

	od := &odometry{
		lastLeftPos:        0,
		lastRightPos:       0,
		baseWidth:          1,
		wheelCircumference: 1,
		timeIntervalMSecs:  500,
	}
	od.motors = append(od.motors, motorPair{left, right})
	od.trackPosition(context.Background())

	// move straight 5 m
	setPositions(5, 5)
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err := od.Position(ctx, nil)
	or, _ := od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 0, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 5, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 0, 0.1)

	// move backwards 10 m
	setPositions(-10, -10)
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err = od.Position(ctx, nil)
	or, _ = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 0, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, -5, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 0, 0.1)
}

func TestComplicatedPath(t *testing.T) {
	left := createFakeMotor(true)
	right := createFakeMotor(false)
	ctx := context.Background()
	_ = left.ResetZeroPosition(ctx, 0, nil)
	_ = right.ResetZeroPosition(ctx, 0, nil)

	od := &odometry{
		lastLeftPos:        0,
		lastRightPos:       0,
		baseWidth:          1,
		wheelCircumference: 1,
		timeIntervalMSecs:  500,
	}
	od.motors = append(od.motors, motorPair{left, right})
	od.trackPosition(context.Background())

	// move straight 5 m
	setPositions(5, 5)
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err := od.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	or, _ := od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 0, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 5, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 0, 0.1)

	// spin negative 90 degrees
	setPositions(1*(math.Pi/4), -1*(math.Pi/4))
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err = od.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	or, err = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 270, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 5, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 0, 0.1)

	// move forward another 5 m
	setPositions(5, 5)
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err = od.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	or, err = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 270, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 5, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, -5, 0.1)

	// spin positive 45 degrees
	setPositions(-1*(math.Pi/8), 1*(math.Pi/8))
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err = od.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	or, err = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 315, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 5, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, -5, 0.1)

	// move forward 2 m
	setPositions(2, 2)
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err = od.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	or, err = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 315, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 6.4, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, -6.4, 0.1)

	// travel in an arc
	setPositions(1*(math.Pi/4), 2*(math.Pi/4))
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err = od.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	or, err = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 360, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 7.6, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, -6.4, 0.1)
}

func TestVelocities(t *testing.T) {
	left := createFakeMotor(true)
	right := createFakeMotor(false)
	ctx := context.Background()
	_ = left.ResetZeroPosition(ctx, 0, nil)
	_ = right.ResetZeroPosition(ctx, 0, nil)

	od := &odometry{
		lastLeftPos:        0,
		lastRightPos:       0,
		baseWidth:          1,
		wheelCircumference: 1,
		timeIntervalMSecs:  500,
	}
	od.motors = append(od.motors, motorPair{left, right})
	od.trackPosition(context.Background())

	// move forward 10 m
	setPositions(10, 10)
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	linVel, err := od.LinearVelocity(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	angVel, err := od.AngularVelocity(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, linVel.Y, test.ShouldAlmostEqual, 20, 0.1)
	test.That(t, angVel.Z, test.ShouldAlmostEqual, 0, 0.1)

	// spin 45 degrees
	setPositions(-1*(math.Pi/8), 1*(math.Pi/8))
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	linVel, err = od.LinearVelocity(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	angVel, err = od.AngularVelocity(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, linVel.Y, test.ShouldAlmostEqual, 0, 0.1)
	test.That(t, angVel.Z, test.ShouldAlmostEqual, 90, 0.1)

	// spin back 45 degrees
	setPositions(1*(math.Pi/8), -1*(math.Pi/8))
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	linVel, err = od.LinearVelocity(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	angVel, err = od.AngularVelocity(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, linVel.Y, test.ShouldAlmostEqual, 0, 0.1)
	test.That(t, angVel.Z, test.ShouldAlmostEqual, -90, 0.1)

	// move backwards 5 m
	setPositions(-5, -5)
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	linVel, err = od.LinearVelocity(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	angVel, err = od.AngularVelocity(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, linVel.Y, test.ShouldAlmostEqual, -10, 0.1)
	test.That(t, angVel.Z, test.ShouldAlmostEqual, 0, 0.1)
}
