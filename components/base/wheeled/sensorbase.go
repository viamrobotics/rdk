package wheeled

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	yawPollTime = 10 * time.Millisecond
	errTurn     = 2.0
	errTarget   = 5.0
	oneTurn     = 360
	increment   = 0.1
	sensorDebug = true
)

type sensorBase struct {
	generic.Unimplemented
	logger golog.Logger

	workers       sync.WaitGroup
	base          base.LocalBase
	sensorMu      sync.Mutex
	sensorCtx     context.Context
	sensorDone    func()
	pollingActive bool

	allSensors  []movementsensor.MovementSensor
	orientation movementsensor.MovementSensor
}

func makeBaseWithSensors(
	ctx context.Context,
	base base.LocalBase,
	deps registry.Dependencies,
	cfg config.Component,
	logger golog.Logger,
) (base.LocalBase, error) {
	// spawn a new context for sensors so we don't add many background workers
	attr, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attr, &AttrConfig{})
	}

	s := &sensorBase{base: base, logger: logger}

	s.sensorCtx, s.sensorDone = context.WithCancel(context.Background())

	var omsName string
	for _, msName := range attr.MovementSensor {
		ms, err := movementsensor.FromDependencies(deps, msName)
		if err != nil {
			return nil, errors.Wrapf(err, "no movement_sensor namesd (%s)", msName)
		}
		s.allSensors = append(s.allSensors, ms)
		props, err := ms.Properties(ctx, nil)
		if props.OrientationSupported && err == nil {
			s.orientation = ms
			omsName = msName
		}
	}
	s.logger.Infof("using sensor %s as orientation sensor for base", omsName)

	return s, nil
}

// setPollActive determines whether we want the sensor loop to run ad stop the base with sensor feedback
// should be set to false everywhere except when sensor feedback is need to stop movement.
func (s *sensorBase) setPollActive(isActive bool) {
	s.sensorMu.Lock()
	s.pollingActive = isActive
	s.sensorMu.Unlock()
}

// isPollActive gets whether the base is actively polling a sensor.
func (s *sensorBase) isPollActive() bool {
	s.sensorMu.Lock()
	defer s.sensorMu.Unlock()
	return s.pollingActive
}

// Spin commands a base to turn about its center at a angular speed and for a specific angle.
func (s *sensorBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	s.setPollActive(false)
	if s.orientation == nil {
		return s.base.Spin(ctx, angleDeg, degsPerSec, nil)
	}

	s.workers.Add(1)
	utils.ManagedGo(func() {
		if err := s.base.SetVelocity(ctx, r3.Vector{}, r3.Vector{Z: angleDeg}, nil); err != nil {
			s.logger.Error(err)
		}
	}, s.workers.Done)
	return s.stopSpinWithSensor(s.sensorCtx, angleDeg, degsPerSec)
}

func (s *sensorBase) stopSpinWithSensor(
	ctx context.Context, angleDeg, degsPerSec float64,
) error {
	// this is the only are
	startYaw, err := getCurrentYaw(ctx, s.orientation)
	if err != nil {
		return err
	} // from 0 -> 360

	targetYaw, dir, fullTurns := findSpinParams(angleDeg, degsPerSec, startYaw)
	errBound := errTarget
	if fullTurns > 0 {
		errBound = errTurn
	}
	turnCount := 0
	errCounter := 0

	s.workers.Add(1)
	utils.ManagedGo(func() {
		ticker := time.NewTicker(yawPollTime)
		defer ticker.Stop()
		for {
			// check if we want to poll the sensor at all
			if !s.isPollActive() {
				return
			}
			select {
			case <-ctx.Done():
				s.setPollActive(false)
				return
			case <-ticker.C:
				// imu readings are limited from 0 -> 360
				currYaw, err := getCurrentYaw(ctx, s.orientation)
				if err != nil {
					errCounter++
					if errCounter > 100 {
						s.logger.Error(errors.Wrap(
							err, "imu sensor unreachable, 100 error counts when trying to read yaw angle, stopping base"))
						if err := s.Stop(ctx, nil); err != nil {
							s.logger.Error(err)
						}
						return
					}
					return
				}
				errCounter = 0 // reset reading error count to zero if we are successfully reading again

				atTarget, overShot, minTravel := getTurnState(currYaw, startYaw, targetYaw, dir, angleDeg, errBound)

				// if the imu yaw reading is close to 360, we are nearing a full turn,
				// so we adjust the current reading by 360 * the number of turns we've done
				if atTarget && (fullTurns > 0) {
					turnCount++
					overShot = false
					minTravel = true
				}

				if sensorDebug {
					s.logger.Debugf("minTravel %t, atTarget %t, overshot %t", minTravel, atTarget, overShot)
					s.logger.Debugf("angleDeg %.2f, increment %.2f, turnCount %d, fullTurns %d",
						angleDeg, increment, turnCount, fullTurns)
					s.logger.Debugf("currYaw %.2f, startYaw %.2f, targetYaw %.2f", currYaw, startYaw, targetYaw)
				}

				// poll the sensor for the current error in angle
				// check if we've overshot our target by the errTarget value
				// check if we've travelled at all
				if fullTurns == 0 {
					if (atTarget && minTravel) || (overShot && minTravel) {
						s.setPollActive(false)
						if err := s.Stop(ctx, nil); err != nil {
							return
						}

						if sensorDebug {
							s.logger.Debugf(
								"stopping base with errAngle:%.2f, overshot? %t",
								math.Abs(targetYaw-currYaw), overShot)
						}
					}
				} else {
					if (turnCount >= fullTurns) && atTarget {
						s.setPollActive(false)
						if err := s.Stop(ctx, nil); err != nil {
							return
						}

						if sensorDebug {
							s.logger.Debugf(
								"stopping base with errAngle:%.2f, overshot? %t, fullTurns %d, turnCount %d",
								math.Abs(targetYaw-currYaw), overShot, fullTurns, turnCount)
						}
					}
				}
			}
		}
	}, s.workers.Done)
	return nil
}

func getTurnState(currYaw, startYaw, targetYaw, dir, angleDeg, errorBound float64) (atTarget, overShot, minTravel bool) {
	atTarget = math.Abs(targetYaw-currYaw) < errorBound
	overShot = hasOverShot(currYaw, startYaw, targetYaw, dir)
	minTravel = math.Abs(currYaw-startYaw) > math.Abs(angleDeg*increment)
	return atTarget, overShot, minTravel
}

func getCurrentYaw(ctx context.Context, ms movementsensor.MovementSensor,
) (float64, error) {
	ctx, done := context.WithCancel(ctx)
	defer done()
	orientation, err := ms.Orientation(ctx, nil)
	if err != nil {
		return 0, err
	}
	// Add Pi  to make the computation for overshoot simpler
	// turns imus from -180 -> 180 to a 0 -> 360 range
	return addAnglesInDomain(rdkutils.RadToDeg(orientation.EulerAngles().Yaw), 0), nil
}

func addAnglesInDomain(target, current float64) float64 {
	angle := target + current
	// reduce the angle
	angle = math.Mod(angle, oneTurn)

	// force it to be the positive remainder, so that 0 <= angle < 360
	angle = math.Mod(angle+oneTurn, oneTurn)
	return angle
}

func findSpinParams(angleDeg, degsPerSec, currYaw float64) (float64, float64, int) {
	targetYaw := addAnglesInDomain(angleDeg, currYaw)
	dir := 1.0
	if math.Signbit(degsPerSec) != math.Signbit(angleDeg) {
		// both positive or both negative -> counterclockwise spin call
		// counterclockwise spin calls add allowable angles
		// the signs being different --> clockwise spin call
		// cloxkwise spin calls subtract allowable angles
		dir = -1
	}
	fullTurns := int(math.Abs(angleDeg)) / int(oneTurn)

	return targetYaw, dir, fullTurns
}

// this function does not wrap around 360 degrees currently.
func angleBetween(current, bound1, bound2 float64) bool {
	if bound2 > bound1 {
		inBewtween := current >= bound1 && current < bound2
		return inBewtween
	}
	inBewteen := current > bound2 && current <= bound1
	return inBewteen
}

func hasOverShot(angle, start, target, dir float64) bool {
	switch {
	case dir == -1 && start > target:
		// for cases with a quadrant switch from 1 -> 4
		// check if the current angle is in the regions before the
		// target and after the start
		over := angleBetween(angle, target, 0) || angleBetween(angle, 360, start)
		return over
	case dir == -1 && target > start:
		// the overshoot range is the inside range between the start and target
		return angleBetween(angle, target, start)
	case dir == 1 && start > target:
		// for cases with a quadrant switch from 1 <-> 4
		// check if the current angle is not in the regions after the
		// target and before the start
		over := !angleBetween(angle, 0, target) && !angleBetween(angle, start, 360)
		return over
	default:
		// the overshoot range is the outside range between the start and target
		return !angleBetween(angle, start, target)
	}
}

func (s *sensorBase) MoveStraight(
	ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{},
) error {
	s.setPollActive(false)
	return s.base.MoveStraight(ctx, distanceMm, mmPerSec, extra)
}

func (s *sensorBase) SetVelocity(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	s.setPollActive(false)
	return s.base.SetVelocity(ctx, linear, angular, extra)
}

func (s *sensorBase) SetPower(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	s.setPollActive(false)
	return s.base.SetPower(ctx, linear, angular, extra)
}

func (s *sensorBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	s.setPollActive(false)
	return s.base.Stop(ctx, extra)
}

func (s *sensorBase) IsMoving(ctx context.Context) (bool, error) {
	return s.base.IsMoving(ctx)
}

func (s *sensorBase) Width(ctx context.Context) (int, error) {
	return s.base.Width(ctx)
}

func (s *sensorBase) Close(ctx context.Context) error {
	s.setPollActive(false)
	base, isWheeled := rdkutils.UnwrapProxy(s.base).(*wheeledBase)
	if isWheeled {
		return base.Close(ctx)
	}
	return nil
}
