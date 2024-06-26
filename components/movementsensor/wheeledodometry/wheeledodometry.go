// Package wheeledodometry implements an odometery estimate from an encoder wheeled base.
package wheeledodometry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// Model is the name of the wheeled odometry model of a movementsensor component.
var Model = resource.DefaultModelFamily.WithModel("wheeled-odometry")

const (
	defaultTimeIntervalMSecs = 500
	oneTurn                  = 2 * math.Pi
	mToKm                    = 1e-3
	returnRelative           = "return_relative_pos_m"
	setLong                  = "setLong"
	setLat                   = "setLat"
	useCompass               = "use_compass"
	shiftPos                 = "shift_position"
	resetShift               = "reset"
	moveX                    = "moveX"
	moveY                    = "moveY"
)

// Config is the config for a wheeledodometry MovementSensor.
type Config struct {
	LeftMotors        []string `json:"left_motors"`
	RightMotors       []string `json:"right_motors"`
	Base              string   `json:"base"`
	TimeIntervalMSecs float64  `json:"time_interval_msecs,omitempty"`
}

type motorPair struct {
	left  motor.Motor
	right motor.Motor
}

type odometry struct {
	resource.Named
	resource.AlwaysRebuild

	lastLeftPos        float64
	lastRightPos       float64
	baseWidth          float64
	wheelCircumference float64
	base               base.Base
	timeIntervalMSecs  float64

	motors []motorPair

	angularVelocity spatialmath.AngularVelocity
	linearVelocity  r3.Vector
	position        r3.Vector
	orientation     spatialmath.EulerAngles
	coordUpToDate   atomic.Bool
	coord           *geo.Point
	originCoord     *geo.Point

	useCompass bool
	shiftPos   bool

	workers utils.StoppableWorkers
	mu      sync.Mutex
	logger  logging.Logger
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		Model,
		resource.Registration[movementsensor.MovementSensor, *Config]{Constructor: newWheeledOdometry})
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string

	if cfg.Base == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "base")
	}
	deps = append(deps, cfg.Base)

	if len(cfg.LeftMotors) == 0 {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "left motors")
	}
	deps = append(deps, cfg.LeftMotors...)

	if len(cfg.RightMotors) == 0 {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "right motors")
	}
	deps = append(deps, cfg.RightMotors...)

	if len(cfg.LeftMotors) != len(cfg.RightMotors) {
		return nil, errors.New("mismatch number of left and right motors")
	}

	// Temporary validation check until support for more than one left and right motor each is added.
	if len(cfg.LeftMotors) > 1 || len(cfg.RightMotors) > 1 {
		return nil, errors.New("wheeled odometry only supports one left and right motor each")
	}

	return deps, nil
}

// Reconfigure automatically reconfigures this movement sensor based on the updated config.
func (o *odometry) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	if len(o.motors) > 0 {
		if err := o.motors[0].left.Stop(ctx, nil); err != nil {
			return err
		}
		if err := o.motors[0].right.Stop(ctx, nil); err != nil {
			return err
		}
	}

	if o.workers != nil {
		o.workers.Stop()
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	// set the new timeIntervalMSecs
	o.timeIntervalMSecs = newConf.TimeIntervalMSecs
	if o.timeIntervalMSecs == 0 {
		o.timeIntervalMSecs = defaultTimeIntervalMSecs
	}
	if o.timeIntervalMSecs > 1000 {
		o.logger.CWarn(ctx, "if the time interval is more than 1000 ms, be sure to move the base slowly for better accuracy")
	}

	// set baseWidth and wheelCircumference from the new base properties
	newBase, err := base.FromDependencies(deps, newConf.Base)
	if err != nil {
		return err
	}
	props, err := newBase.Properties(ctx, nil)
	if err != nil {
		return err
	}
	o.baseWidth = props.WidthMeters
	o.wheelCircumference = props.WheelCircumferenceMeters
	if o.baseWidth == 0 || o.wheelCircumference == 0 {
		return errors.New("base width or wheel circumference are 0, movement sensor cannot be created")
	}
	o.base = newBase
	o.logger.Debugf("using base %v for wheeled_odometry sensor", newBase.Name().ShortName())

	// check if new motors have been added, or the existing motors have been changed, and update the motorPairs accorodingly
	for i := range newConf.LeftMotors {
		var motorLeft, motorRight motor.Motor

		motorLeft, err = motor.FromDependencies(deps, newConf.LeftMotors[i])
		if err != nil {
			return err
		}
		properties, err := motorLeft.Properties(ctx, nil)
		if err != nil {
			return err
		}
		if !properties.PositionReporting {
			return motor.NewPropertyUnsupportedError(properties, newConf.LeftMotors[i])
		}

		motorRight, err = motor.FromDependencies(deps, newConf.RightMotors[i])
		if err != nil {
			return err
		}
		properties, err = motorRight.Properties(ctx, nil)
		if err != nil {
			return err
		}
		if !properties.PositionReporting {
			return motor.NewPropertyUnsupportedError(properties, newConf.LeftMotors[i])
		}

		// append if motors have been added, replace if motors have changed
		thisPair := motorPair{left: motorLeft, right: motorRight}
		if i >= len(o.motors) {
			o.motors = append(o.motors, thisPair)
		} else if (o.motors[i].left.Name().ShortName() != newConf.LeftMotors[i]) ||
			(o.motors[i].right.Name().ShortName() != newConf.RightMotors[i]) {
			o.motors[i].left = motorLeft
			o.motors[i].right = motorRight
		}
		o.logger.Debugf("using motors %v for wheeled odometery",
			[]string{motorLeft.Name().ShortName(), motorRight.Name().ShortName()})
	}

	if len(o.motors) > 1 {
		o.logger.CWarn(ctx, "odometry will not be accurate if the left and right motors that are paired are not listed in the same order")
	}

	o.orientation.Yaw = 0
	o.originCoord = geo.NewPoint(0, 0)
	o.coordUpToDate.Store(false)
	o.mu.Unlock()
	defer o.mu.Lock() // Must be unlocked for trackPosition. We put a Lock on the defer stack so the earlier deferred unlock does not hang.
	o.trackPosition() // (re-)initializes o.workers
	// Wait for trackPosition to initialize coord so we do not start with stale data
	for !o.coordUpToDate.Load() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		time.Sleep(time.Duration(o.timeIntervalMSecs) * time.Millisecond)
	}
	return nil
}

// newWheeledOdometry returns a new wheeled encoder movement sensor defined by the given config.
func newWheeledOdometry(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (movementsensor.MovementSensor, error) {
	o := &odometry{
		Named:        conf.ResourceName().AsNamed(),
		lastLeftPos:  0.0,
		lastRightPos: 0.0,
		originCoord:  geo.NewPoint(0, 0),
		logger:       logger,
	}

	if err := o.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return o, nil
}

func (o *odometry) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.angularVelocity, nil
}

func (o *odometry) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
}

func (o *odometry) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	ov := &spatialmath.OrientationVector{Theta: o.orientation.Yaw, OX: 0, OY: 0, OZ: 1}
	return ov, nil
}

func (o *odometry) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.useCompass {
		return yawToCompassHeading(o.orientation.Yaw), nil
	}

	return 0, movementsensor.ErrMethodUnimplementedCompassHeading
}

// computes the compass heading in degrees from a yaw in radians, with 0 -> 360 and Z down.
func yawToCompassHeading(yaw float64) float64 {
	yawDeg := utils.RadToDeg(yaw)
	if yawDeg < 0 {
		yawDeg = 180 - yawDeg
	}
	return 360 - yawDeg
}

func (o *odometry) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.linearVelocity, nil
}

func (o *odometry) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if relative, ok := extra[returnRelative]; ok {
		if relative.(bool) {
			return geo.NewPoint(o.position.Y, o.position.X), o.position.Z, nil
		}
	}

	return o.coord, o.position.Z, nil
}

func (o *odometry) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := movementsensor.DefaultAPIReadings(ctx, o, extra)
	if err != nil {
		return nil, err
	}

	// movementsensor.Readings calls all the APIs with their owm mutex lock in this driver
	// the lock has been released, so for the last two readings we lock again to append them to the readings map
	o.mu.Lock()
	defer o.mu.Unlock()
	readings["position_meters_X"] = o.position.X
	readings["position_meters_Y"] = o.position.Y

	return readings, nil
}

func (o *odometry) Accuracy(ctx context.Context, extra map[string]interface{}) (*movementsensor.Accuracy, error,
) {
	return movementsensor.UnimplementedOptionalAccuracies(), nil
}

func (o *odometry) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported:  true,
		AngularVelocitySupported: true,
		OrientationSupported:     true,
		PositionSupported:        true,
		CompassHeadingSupported:  o.useCompass,
	}, nil
}

func (o *odometry) Close(ctx context.Context) error {
	o.workers.Stop()
	return nil
}

func (o *odometry) checkBaseProps(ctx context.Context) {
	props, err := o.base.Properties(ctx, nil)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			o.logger.Error(err)
			return
		}
	}
	if (o.baseWidth != props.WidthMeters) || (o.wheelCircumference != props.WheelCircumferenceMeters) {
		o.baseWidth = props.WidthMeters
		o.wheelCircumference = props.WheelCircumferenceMeters
		o.logger.Warnf("Base %v's properties have changed: baseWidth = %v and wheelCirumference = %v.",
			"Odometry can optionally be reset using DoCommand",
			o.base.Name().ShortName(), o.baseWidth, o.wheelCircumference)
	}
}

// trackPosition uses the motor positions to calculate an estimation of the position, orientation,
// linear velocity, and angular velocity of the wheeled base.
// The estimations in this function are based on the math outlined in this article:
// https://stuff.mit.edu/afs/athena/course/6/6.186/OldFiles/2005/doc/odomtutorial/odomtutorial.pdf
func (o *odometry) trackPosition() {
	// Spawn a new goroutine to do all the work in the background.
	o.workers = utils.NewStoppableWorkers(func(ctx context.Context) {
		ticker := time.NewTicker(time.Duration(o.timeIntervalMSecs) * time.Millisecond)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Sleep until it's time to update the position again.
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			// Use GetInParallel to ensure the left and right motors are polled at the same time.
			positionFuncs := func() []utils.FloatFunc {
				fs := []utils.FloatFunc{}

				// Always use the first pair until more than one pair of motors is supported in this model.
				fs = append(fs, func(ctx context.Context) (float64, error) { return o.motors[0].left.Position(ctx, nil) })
				fs = append(fs, func(ctx context.Context) (float64, error) { return o.motors[0].right.Position(ctx, nil) })

				return fs
			}

			_, positions, err := utils.GetInParallel(ctx, positionFuncs())
			if err != nil {
				o.logger.CError(ctx, err)
				continue
			}

			// Current position of the left and right motors in revolutions.
			if len(positions) != len(o.motors)*2 {
				o.logger.CError(ctx, "error getting both motor positions, trying again")
				continue
			}
			left := positions[0]
			right := positions[1]

			// Base properties need to be checked every time because dependent components reconfiguring does not trigger
			// the parent component to reconfigure. In this case, that means if the base properties change, the wheeled
			// odometry movement sensor will not be aware of these changes and will continue to use the old values
			o.checkBaseProps(ctx)

			// Difference in the left and right motors since the last iteration, in mm.
			leftDist := (left - o.lastLeftPos) * o.wheelCircumference
			rightDist := (right - o.lastRightPos) * o.wheelCircumference

			// Update lastLeftPos and lastRightPos to be the current position in mm.
			o.lastLeftPos = left
			o.lastRightPos = right

			// Linear and angular distance the center point has traveled. This works based on
			// the assumption that the time interval between calulations is small enough that
			// the inner angle of the rotation will be small enough that it can be accurately
			// estimated using the below equations.
			centerDist := (leftDist + rightDist) / 2
			centerAngle := (rightDist - leftDist) / o.baseWidth

			// Update the position and orientation values accordingly.
			o.mu.Lock()
			o.orientation.Yaw += centerAngle

			// Limit the yaw to a range of positive 0 to 360 degrees.
			o.orientation.Yaw = math.Mod(o.orientation.Yaw, oneTurn)
			o.orientation.Yaw = math.Mod(o.orientation.Yaw+oneTurn, oneTurn)
			angle := o.orientation.Yaw
			xFlip := -1.0
			if o.useCompass {
				angle = utils.DegToRad(yawToCompassHeading(o.orientation.Yaw))
				xFlip = 1.0
			}
			o.position.X += xFlip * (centerDist * math.Sin(angle))
			o.position.Y += (centerDist * math.Cos(angle))

			distance := math.Hypot(o.position.X, o.position.Y)
			heading := utils.RadToDeg(math.Atan2(o.position.X, o.position.Y))
			o.coord = o.originCoord.PointAtDistanceAndBearing(distance*mToKm, heading)
			o.coordUpToDate.Store(true)

			// Update the linear and angular velocity values using the provided time interval.
			o.linearVelocity.Y = centerDist / (o.timeIntervalMSecs / 1000)
			o.angularVelocity.Z = centerAngle * (180 / math.Pi) / (o.timeIntervalMSecs / 1000)

			o.mu.Unlock()
		}
	})
}

func (o *odometry) DoCommand(ctx context.Context,
	req map[string]interface{},
) (map[string]interface{}, error) {
	resp := make(map[string]interface{})

	o.mu.Lock()
	defer o.mu.Unlock()
	cmd, ok := req[useCompass].(bool)
	if ok {
		o.useCompass = cmd
		resp[useCompass] = fmt.Sprintf("using orientation as compass heading set to %v", cmd)
	}

	reset, ok := req[resetShift].(bool)
	if ok {
		o.shiftPos = reset
		o.originCoord = geo.NewPoint(0, 0)
		o.coord = geo.NewPoint(0, 0)
		o.position.X = 0
		o.position.Y = 0
		o.orientation.Yaw = 0

		resp[resetShift] = fmt.Sprintf("resetting position and setting shift to %v", reset)
	}
	lat, okY := req[setLat].(float64)
	long, okX := req[setLong].(float64)
	if okY && okX {
		o.originCoord = geo.NewPoint(lat, long)
		o.shiftPos = true
		o.coordUpToDate.Store(false)
		o.mu.Unlock()
		// Wait for trackPosition to initialize coord so we do not start with stale data
		for !o.coordUpToDate.Load() {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			time.Sleep(time.Duration(o.timeIntervalMSecs) * time.Millisecond)
		}
		o.mu.Lock()

		resp[setLat] = fmt.Sprintf("lat shifted to %.8f", lat)
		resp[setLong] = fmt.Sprintf("lng shifted to %.8f", long)
	} else if okY || okX {
		// If only one value is given, return an error.
		// This prevents errors when neither is given.
		resp["bad shift"] = "need both lat and long shifts"
	}

	xMove, okX := req[moveX].(float64)
	yMove, okY := req[moveY].(float64)
	if okX {
		o.position.X += xMove
		resp[moveX] = fmt.Sprintf("x position moved to %.8f", o.position.X)
	}
	if okY {
		o.position.Y += yMove
		resp[moveY] = fmt.Sprintf("y position shifted to %.8f", o.position.Y)
	}

	return resp, nil
}
