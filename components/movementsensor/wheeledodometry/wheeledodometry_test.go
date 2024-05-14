// Package wheeledodometry implements an odometery estimate from an encoder wheeled base
package wheeledodometry

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	leftMotorName     = "left"
	rightMotorName    = "right"
	baseName          = "base"
	testSensorName    = "name"
	newLeftMotorName  = "new_left"
	newRightMotorName = "new_right"
	newBaseName       = "new_base"
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

var relativePos = map[string]interface{}{returnRelative: true}

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
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error {
			return nil
		},
	}
}

func createFakeBase(circ, width, rad float64) base.Base {
	return &inject.Base{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
			return base.Properties{WheelCircumferenceMeters: circ, WidthMeters: width, TurningRadiusMeters: rad}, nil
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
	logger := logging.NewTestLogger(t)

	deps := make(resource.Dependencies)
	deps[base.Named(baseName)] = createFakeBase(0.1, 0.1, 0.1)
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

func TestReconfigure(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	deps := make(resource.Dependencies)
	deps[base.Named(baseName)] = createFakeBase(0.1, 0.1, 0)
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
	od, ok := fakeSensor.(*odometry)
	test.That(t, ok, test.ShouldBeTrue)

	newDeps := make(resource.Dependencies)
	newDeps[base.Named(newBaseName)] = createFakeBase(0.2, 0.2, 0)
	newDeps[motor.Named(newLeftMotorName)] = createFakeMotor(true)
	newDeps[motor.Named(rightMotorName)] = createFakeMotor(false)

	newconf := resource.Config{
		Name: testSensorName,
		ConvertedAttributes: &Config{
			LeftMotors:        []string{newLeftMotorName},
			RightMotors:       []string{rightMotorName},
			Base:              newBaseName,
			TimeIntervalMSecs: 500,
		},
	}

	err = fakeSensor.Reconfigure(ctx, newDeps, newconf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, od.timeIntervalMSecs, test.ShouldEqual, 500)
	props, err := od.base.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.WidthMeters, test.ShouldEqual, 0.2)
	test.That(t, props.WheelCircumferenceMeters, test.ShouldEqual, 0.2)

	newDeps = make(resource.Dependencies)
	newDeps[base.Named(newBaseName)] = createFakeBase(0.2, 0.2, 0)
	newDeps[motor.Named(newLeftMotorName)] = createFakeMotor(true)
	newDeps[motor.Named(newRightMotorName)] = createFakeMotor(false)

	newconf = resource.Config{
		Name: testSensorName,
		ConvertedAttributes: &Config{
			LeftMotors:        []string{newLeftMotorName},
			RightMotors:       []string{newRightMotorName},
			Base:              newBaseName,
			TimeIntervalMSecs: 200,
		},
	}

	err = fakeSensor.Reconfigure(ctx, newDeps, newconf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, od.timeIntervalMSecs, test.ShouldEqual, 200)
	props, err = od.base.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.WidthMeters, test.ShouldEqual, 0.2)
	test.That(t, props.WheelCircumferenceMeters, test.ShouldEqual, 0.2)
}

func TestValidateConfig(t *testing.T) {
	cfg := Config{
		LeftMotors:        []string{leftMotorName},
		RightMotors:       []string{rightMotorName},
		Base:              "",
		TimeIntervalMSecs: 500,
	}

	deps, err := cfg.Validate("path")
	expectedErr := resource.NewConfigValidationFieldRequiredError("path", "base")
	test.That(t, err, test.ShouldBeError, expectedErr)
	test.That(t, deps, test.ShouldBeEmpty)

	cfg = Config{
		LeftMotors:        []string{},
		RightMotors:       []string{rightMotorName},
		Base:              baseName,
		TimeIntervalMSecs: 500,
	}

	deps, err = cfg.Validate("path")
	expectedErr = resource.NewConfigValidationFieldRequiredError("path", "left motors")
	test.That(t, err, test.ShouldBeError, expectedErr)
	test.That(t, deps, test.ShouldBeEmpty)

	cfg = Config{
		LeftMotors:        []string{leftMotorName},
		RightMotors:       []string{},
		Base:              baseName,
		TimeIntervalMSecs: 500,
	}

	deps, err = cfg.Validate("path")
	expectedErr = resource.NewConfigValidationFieldRequiredError("path", "right motors")
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
	base := createFakeBase(0.2, 0.2, 0.1)
	ctx := context.Background()
	_ = left.ResetZeroPosition(ctx, 0, nil)
	_ = right.ResetZeroPosition(ctx, 0, nil)

	od := &odometry{
		lastLeftPos:        0,
		lastRightPos:       0,
		wheelCircumference: 0.2,
		baseWidth:          0.2,
		base:               base,
		timeIntervalMSecs:  500,
		originCoord:        geo.NewPoint(0, 0),
	}
	od.motors = append(od.motors, motorPair{left, right})
	od.trackPosition()

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
	test.That(t, od.Close(context.Background()), test.ShouldBeNil)
}

func TestMoveStraight(t *testing.T) {
	left := createFakeMotor(true)
	right := createFakeMotor(false)
	base := createFakeBase(1, 1, 0.1)
	ctx := context.Background()
	_ = left.ResetZeroPosition(ctx, 0, nil)
	_ = right.ResetZeroPosition(ctx, 0, nil)

	od := &odometry{
		lastLeftPos:        0,
		lastRightPos:       0,
		wheelCircumference: 1,
		baseWidth:          1,
		base:               base,
		timeIntervalMSecs:  500,
		originCoord:        geo.NewPoint(0, 0),
	}
	od.motors = append(od.motors, motorPair{left, right})
	od.trackPosition()

	// move straight 5 m
	setPositions(5, 5)
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err := od.Position(ctx, relativePos)
	or, _ := od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 0, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 5, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 0, 0.1)

	// move backwards 10 m
	setPositions(-10, -10)
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err = od.Position(ctx, relativePos)
	or, _ = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 0, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, -5, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 0, 0.1)
	test.That(t, od.Close(context.Background()), test.ShouldBeNil)
}

func TestComplicatedPath(t *testing.T) {
	left := createFakeMotor(true)
	right := createFakeMotor(false)
	base := createFakeBase(1, 1, 0.1)
	ctx := context.Background()
	_ = left.ResetZeroPosition(ctx, 0, nil)
	_ = right.ResetZeroPosition(ctx, 0, nil)

	od := &odometry{
		lastLeftPos:        0,
		lastRightPos:       0,
		wheelCircumference: 1,
		baseWidth:          1,
		base:               base,
		timeIntervalMSecs:  500,
		originCoord:        geo.NewPoint(0, 0),
	}
	od.motors = append(od.motors, motorPair{left, right})
	od.trackPosition()

	// move straight 5 m
	setPositions(5, 5)
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err := od.Position(ctx, relativePos)
	test.That(t, err, test.ShouldBeNil)
	or, _ := od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 0, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 5, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 0, 0.1)

	// spin negative 90 degrees
	setPositions(1*(math.Pi/4), -1*(math.Pi/4))
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err = od.Position(ctx, relativePos)
	test.That(t, err, test.ShouldBeNil)
	or, err = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 270, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 5, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 0, 0.1)

	// move forward another 5 m
	setPositions(5, 5)
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err = od.Position(ctx, relativePos)
	test.That(t, err, test.ShouldBeNil)
	or, err = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 270, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 5, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 5, 0.1)

	// spin positive 45 degrees
	setPositions(-1*(math.Pi/8), 1*(math.Pi/8))
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err = od.Position(ctx, relativePos)
	test.That(t, err, test.ShouldBeNil)
	or, err = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 315, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 5, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 5, 0.1)

	// move forward 2 m
	setPositions(2, 2)
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err = od.Position(ctx, relativePos)
	test.That(t, err, test.ShouldBeNil)
	or, err = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 315, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 6.4, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 6.4, 0.1)

	// travel in an arc
	setPositions(1*(math.Pi/4), 2*(math.Pi/4))
	time.Sleep(time.Duration(od.timeIntervalMSecs*1.15) * time.Millisecond)

	pos, _, err = od.Position(ctx, relativePos)
	test.That(t, err, test.ShouldBeNil)
	or, err = od.Orientation(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, or.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 0, 0.1)
	test.That(t, pos.Lat(), test.ShouldAlmostEqual, 7.6, 0.1)
	test.That(t, pos.Lng(), test.ShouldAlmostEqual, 6.4, 0.1)
	test.That(t, od.Close(context.Background()), test.ShouldBeNil)
}

func TestVelocities(t *testing.T) {
	left := createFakeMotor(true)
	right := createFakeMotor(false)
	base := createFakeBase(1, 1, 0.1)
	ctx := context.Background()
	_ = left.ResetZeroPosition(ctx, 0, nil)
	_ = right.ResetZeroPosition(ctx, 0, nil)

	od := &odometry{
		lastLeftPos:  0,
		lastRightPos: 0, wheelCircumference: 1,
		baseWidth:         1,
		base:              base,
		timeIntervalMSecs: 500,
		originCoord:       geo.NewPoint(0, 0),
	}
	od.motors = append(od.motors, motorPair{left, right})
	od.trackPosition()

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
	test.That(t, od.Close(context.Background()), test.ShouldBeNil)
}
