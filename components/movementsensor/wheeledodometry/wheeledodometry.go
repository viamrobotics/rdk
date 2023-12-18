// Package wheeledodometry implements an odometery estimate from an encoder wheeled base.
package wheeledodometry

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

var model = resource.DefaultModelFamily.WithModel("wheeled-odometry")

const (
	defaultTimeIntervalMSecs = 500
	oneTurn                  = 2 * math.Pi
	mToKm                    = 1e-3
	returnRelative           = "return_relative_pos_m"
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
	timeIntervalMSecs  float64

	motors []motorPair

	angularVelocity spatialmath.AngularVelocity
	linearVelocity  r3.Vector
	position        r3.Vector
	orientation     spatialmath.EulerAngles
	coord           *geo.Point

	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
	mu                      sync.Mutex
	logger                  logging.Logger
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		model,
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

	if o.cancelFunc != nil {
		o.cancelFunc()
		o.activeBackgroundWorkers.Wait()
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
		o.logger.Warn("if the time interval is more than 1000 ms, be sure to move the base slowly for better accuracy")
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
			o.logger.Debugf("using motors %v for wheeled odometery", []string{
				thisPair.left.Name().ShortName(), thisPair.right.Name().ShortName(),
			},
			)
		} else if (o.motors[i].left.Name().ShortName() != newConf.LeftMotors[i]) ||
			(o.motors[i].right.Name().ShortName() != newConf.RightMotors[i]) {
			o.motors[i].left = motorLeft
			o.motors[i].right = motorRight
			o.logger.Debugf("using motors %v for wheeled odometery", []string{
				motorLeft.Name().ShortName(), motorRight.Name().ShortName(),
			},
			)
		}
	}

	if len(o.motors) > 1 {
		o.logger.Warn("odometry will not be accurate if the left and right motors that are paired are not listed in the same order")
	}

	o.orientation.Yaw = 0
	ctx, cancelFunc := context.WithCancel(context.Background())
	o.cancelFunc = cancelFunc
	o.trackPosition(ctx)

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
	return 0, movementsensor.ErrMethodUnimplementedCompassHeading
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

func (o *odometry) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

func (o *odometry) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported:  true,
		AngularVelocitySupported: true,
		OrientationSupported:     true,
		PositionSupported:        true,
	}, nil
}

func (o *odometry) Close(ctx context.Context) error {
	o.cancelFunc()
	o.activeBackgroundWorkers.Wait()
	return nil
}

// trackPosition uses the motor positions to calculate an estimation of the position, orientation,
// linear velocity, and angular velocity of the wheeled base.
// The estimations in this function are based on the math outlined in this article:
// https://stuff.mit.edu/afs/athena/course/6/6.186/OldFiles/2005/doc/odomtutorial/odomtutorial.pdf
func (o *odometry) trackPosition(ctx context.Context) {
	geoOrigin := geo.NewPoint(0, 0)

	o.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer o.activeBackgroundWorkers.Done()
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
			positionFuncs := func() []rdkutils.FloatFunc {
				fs := []rdkutils.FloatFunc{}

				// Always use the first pair until more than one pair of motors is supported in this model.
				fs = append(fs, func(ctx context.Context) (float64, error) { return o.motors[0].left.Position(ctx, nil) })
				fs = append(fs, func(ctx context.Context) (float64, error) { return o.motors[0].right.Position(ctx, nil) })

				return fs
			}

			_, positions, err := rdkutils.GetInParallel(ctx, positionFuncs())
			if err != nil {
				o.logger.Error(err)
				continue
			}

			// Current position of the left and right motors in revolutions.
			if len(positions) != len(o.motors)*2 {
				o.logger.Error("error getting both motor positions, trying again")
				continue
			}
			left := positions[0]
			right := positions[1]

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

			// Calculate X and Y by using centerDist and the current orientation yaw (theta).
			o.position.X += (centerDist * math.Sin(o.orientation.Yaw))
			o.position.Y += (centerDist * math.Cos(o.orientation.Yaw))

			distance := math.Hypot(o.position.X, o.position.Y)
			heading := math.Atan2(o.position.X, o.position.Y)
			o.coord = geoOrigin.PointAtDistanceAndBearing(distance*mToKm, heading)

			// Update the linear and angular velocity values using the provided time interval.
			o.linearVelocity.Y = centerDist / (o.timeIntervalMSecs / 1000)
			o.angularVelocity.Z = centerAngle * (180 / math.Pi) / (o.timeIntervalMSecs / 1000)

			o.mu.Unlock()
		}
	})
}
