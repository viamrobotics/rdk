package sensorcontrolled

import (
	"context"
	"math"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	increment   = 0.01 // angle fraction multiplier to check
	oneTurn     = 360.0
	slowDownAng = 15. // angle from goal for spin to begin breaking
)

// Spin commands a base to turn about its center at a angular speed and for a specific angle.
func (sb *sensorBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	sb.logger.Info("yo new spin")
	// make sure the control loop is enabled
	if sb.loop == nil {
		sb.logger.Info("yo new vel")
		if err := sb.startControlLoop(); err != nil {
			return err
		}
		// sb.stopLoop()
	}
	sb.setPolling(true)
	// startYaw, err := getCurrentYaw(sb.orientation)
	// if err != nil {
	// 	return err
	// }
	sb.logger.Info("angleDeg: ", angleDeg)
	sb.logger.Info("degsPerSec: ", degsPerSec)

	orientation, err := sb.orientation.Orientation(ctx, nil)
	if err != nil {
		return err
	}
	initYaw := rdkutils.RadToDeg(orientation.EulerAngles().Yaw)

	ticker := time.NewTicker(time.Duration(1000./sb.controlLoopConfig.Frequency) * time.Millisecond)
	defer ticker.Stop()

	// timeout duration is a multiplier times the expected time to perform a movement
	spinTimeEst := time.Duration(int(time.Second) * int(math.Abs(angleDeg/degsPerSec)))
	startTime := time.Now()
	timeOut := 5 * spinTimeEst
	if timeOut < 10*time.Second {
		timeOut = 10 * time.Second
	}
	angErr := 0.
	prevAngle := 0.
	prevMovedAng := 0.
	for {
		sb.logger.Info("!isPolling: ", !sb.isPolling())
		// check if we want to poll the sensor at all
		// other API calls set this to false so that this for loop stops
		if !sb.isPolling() {
			ticker.Stop()
		}

		if err := ctx.Err(); err != nil {
			ticker.Stop()
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			orientation, err := sb.orientation.Orientation(ctx, nil)
			if err != nil {
				return err
			}

			currYaw := rdkutils.RadToDeg(orientation.EulerAngles().Yaw)
			angErr, prevAngle, prevMovedAng = getAngError(initYaw, currYaw, prevAngle, prevMovedAng, angleDeg)
			sb.logger.Info("currYaw: ", currYaw)
			sb.logger.Info("angErr: ", angErr)
			sb.logger.Info("prevAngle: ", prevAngle)
			sb.logger.Info("prevMovedAng: ", prevMovedAng)
			if math.Abs(angErr) < boundCheckTarget {
				return sb.Stop(ctx, nil)
			}
			angVel := calcAngVel(angErr, degsPerSec)
			sb.logger.Info("angVel: ", angVel)

			if err := sb.updateControlConfig(ctx, 0, angVel); err != nil {
				return err
			}

			if time.Since(startTime) > timeOut {
				sb.logger.CWarn(ctx, "exceeded time for Spin call, stopping base")
				if err := sb.Stop(ctx, nil); err != nil {
					return nil
				}
				return nil
			}
		}
	}
}

func calcAngVel(angErr, degsPerSec float64) float64 {
	angVel := angErr * degsPerSec / slowDownAng
	if angVel > degsPerSec {
		return degsPerSec
	}
	if angVel < -degsPerSec {
		return -degsPerSec
	}
	return angVel
}

func getAngError(initAngle, currYaw, prevAngle, prevMovedAng, desiredAngle float64) (float64, float64, float64) {
	// use initial angle to get the current angle the spin has moved
	wrappedAng := getWrappedAngle360((currYaw - initAngle))
	angMoved := getMovedAng(prevAngle, wrappedAng, prevMovedAng)

	// compute the error
	errAng := (desiredAngle - angMoved)

	return errAng, wrappedAng, angMoved
}

// bounds an angle from 0-360
func getWrappedAngle360(angle float64) float64 {
	return angle - (math.Floor((angle+180.)/(2*180.)))*2*180. // [-180;180):
}

func getMovedAng(prevAngle, currAngle, angMoved float64) float64 {
	// the angle changed from 0 to 359. this means we are spinning in the negative direction
	if prevAngle-currAngle < -300 {
		return angMoved + currAngle - prevAngle - 360
	}
	// the angle changed from 359 to 0
	if prevAngle-currAngle > 300 {
		return angMoved + currAngle - prevAngle + 360
	}
	// add the change in angle to the position
	return angMoved + currAngle - prevAngle
}

// // Spin commands a base to turn about its center at a angular speed and for a specific angle.
// func (sb *sensorBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
// 	sb.logger.Info("yo new spin")
// 	// make sure the control loop is enabled
// 	if sb.loop != nil {
// 		// sb.logger.Info("yo new vel")
// 		// if err := sb.startControlLoop(); err != nil {
// 		// 	return err
// 		// }
// 		sb.stopLoop()
// 	}
// 	time.Sleep(5 * time.Second)
// 	pid := []control.PIDConfig{{Type: "", P: 0.1, I: 0.5, D: 0.001}}
// 	options := control.Options{PositionControlUsingTrapz: true, NeedsAutoTuning: pid[0].NeedsAutoTuning(), LoopFrequency: 20, ControllableType: "motor_name"}
// 	sc := spinController{pid: pid, sb: sb, opts: options}

// 	lp, err := control.SetupPIDControlConfig(sc.pid, "spin-controller", sc.opts, &sc, sb.logger)
// 	if err != nil {
// 		return err
// 	}

// 	sc.controlLoopConfig = lp.ControlConf
// 	sc.lp = lp.ControlLoop
// 	sc.blockNames = lp.BlockNames

// 	if err := sc.startControlLoop(); err != nil {
// 		return err
// 	}
// 	defer sc.lp.Stop()
// 	dependsOn := []string{sc.blockNames[control.BlockNameConstant][0], sc.blockNames[control.BlockNameEndpoint][0]}
// 	velConf := control.CreateTrapzBlock(ctx, sc.blockNames[control.BlockNameTrapezoidal][0], degsPerSec, dependsOn)
// 	sb.logger.Info("yo velocity block name: ", velConf.Name)
// 	sb.logger.Info("yo loop: ", sc.lp)
// 	sb.logger.Info("yo block name: ", sc.blockNames[control.BlockNameTrapezoidal][0])

// 	if err := sc.lp.SetConfigAt(ctx, sc.blockNames[control.BlockNameTrapezoidal][0], velConf); err != nil {
// 		return err
// 	}

// 	// // Update the Constant block with the given setPoint for position control
// 	posConf := control.CreateConstantBlock(ctx, sc.blockNames[control.BlockNameConstant][0], angleDeg)
// 	if err := sc.lp.SetConfigAt(ctx, sc.blockNames[control.BlockNameConstant][0], posConf); err != nil {
// 		return err
// 	}

// 	time.Sleep(5 * time.Second)
// 	sb.logger.Info("spin done sleeping")

// 	return nil
// }

// type spinController struct {
// 	pid               []control.PIDConfig
// 	sb                *sensorBase
// 	opts              control.Options
// 	controlLoopConfig control.Config
// 	blockNames        map[string][]string
// 	lp                *control.Loop
// }

// func (sc *spinController) startControlLoop() error {
// 	loop, err := control.NewLoop(sc.sb.logger, sc.controlLoopConfig, sc)
// 	if err != nil {
// 		return err
// 	}
// 	if err := loop.Start(); err != nil {
// 		return err
// 	}
// 	sc.lp = loop

// 	return nil
// }

// func (sc *spinController) SetState(ctx context.Context, state []*control.Signal) error {
// 	sc.sb.logger.Info("yo set state: ", state[0].GetSignalValueAt(0))
// 	return sc.sb.SetPower(ctx, r3.Vector{}, r3.Vector{Z: state[0].GetSignalValueAt(0)}, nil)
// }

// func (sc *spinController) State(ctx context.Context) ([]float64, error) {
// 	startYaw, err := getCurrentYaw(sc.sb.orientation)
// 	if err != nil {
// 		return nil, err
// 	}
// 	sc.sb.logger.Info("yo state: ", startYaw)
// 	return []float64{startYaw}, nil
// }

// oldSpin commands a base to turn about its center at a angular speed and for a specific angle.
func (sb *sensorBase) oldSpin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	sb.stopLoop()
	if math.Abs(angleDeg) >= 360.0 {
		sb.setPolling(false)
		sb.logger.CWarn(ctx, "feedback for spin calls over 360 not supported yet, spinning without sensor")
		return sb.controlledBase.Spin(ctx, angleDeg, degsPerSec, nil)
	}
	ctx, done := sb.opMgr.New(ctx)
	defer done()
	// check if a sensor context has been started
	if sb.sensorLoopDone != nil {
		sb.sensorLoopDone()
	}

	sb.setPolling(true)

	// starts a goroutine from within wheeled base's runAll function to run motors in the background
	if err := sb.startRunningMotors(ctx, angleDeg, degsPerSec); err != nil {
		return err
	}

	// this will be removed when spin is migrated to using a control loop
	if angleDeg > 0 {
		time.Sleep(1000 * time.Millisecond)
		sb.logger.Error("done sleeping")
	}

	// start a sensor context for the sensor loop based on the longstanding base
	// creator context
	var sensorCtx context.Context
	sensorCtx, sb.sensorLoopDone = context.WithCancel(context.Background())
	if err := sb.stopSpinWithSensor(sensorCtx, angleDeg, degsPerSec); err != nil {
		return err
	}

	// isPolling returns true when a Spin call is in progress, which is not a success condition for our control loop
	baseStopped := func(ctx context.Context) (bool, error) {
		polling := sb.isPolling()
		return !polling, nil
	}

	if err := sb.opMgr.WaitForSuccess(
		ctx,
		yawPollTime,
		baseStopped,
	); err != nil {
		if !errors.Is(err, context.Canceled) {
			return err
		}
	}
	return nil
}

func (sb *sensorBase) startRunningMotors(ctx context.Context, angleDeg, degsPerSec float64) error {
	if math.Signbit(angleDeg) != math.Signbit(degsPerSec) {
		degsPerSec *= -1
	}
	return sb.controlledBase.SetVelocity(ctx,
		r3.Vector{X: 0, Y: 0, Z: 0},
		r3.Vector{X: 0, Y: 0, Z: degsPerSec}, nil)
}

func (sb *sensorBase) stopSpinWithSensor(
	ctx context.Context, angleDeg, degsPerSec float64,
) error {
	// imu readings are limited from 0 -> 360
	startYaw, err := getCurrentYaw(sb.orientation)
	if err != nil {
		return err
	}

	targetYaw, dir, _ := findSpinParams(angleDeg, degsPerSec, startYaw)

	errBound := boundCheckTarget

	// reset error counter for imu reading errors
	errCounter := 0

	// timeout duration is a multiplier times the expected time to perform a movement
	spinTimeEst := time.Duration(int(time.Second) * int(math.Abs(angleDeg/degsPerSec)))
	startTime := time.Now()
	timeOut := 5 * spinTimeEst
	if timeOut < 10*time.Second {
		timeOut = 10 * time.Second
	}

	sb.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		ticker := time.NewTicker(yawPollTime)
		defer ticker.Stop()

		for {
			// check if we want to poll the sensor at all
			// other API calls set this to false so that this for loop stops
			if !sb.isPolling() {
				ticker.Stop()
			}

			if err := ctx.Err(); err != nil {
				ticker.Stop()
				return
			}

			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				// imu readings are limited from 0 -> 360
				currYaw, err := getCurrentYaw(sb.orientation)
				if err != nil {
					errCounter++
					if errCounter > 100 {
						sb.logger.CError(ctx, errors.Wrap(
							err,
							"imu sensor unreachable, 100 error counts when trying to read yaw angle, stopping base"))
						if err = sb.Stop(ctx, nil); err != nil {
							return
						}
					}
					continue
				}
				errCounter = 0 // reset reading error count to zero if we are successfully reading again

				atTarget, overShot, minTravel := getTurnState(currYaw, startYaw, targetYaw, dir, angleDeg, errBound)

				if sensorDebug {
					sb.logger.CDebugf(ctx, "minTravel %t, atTarget %t, overshot %t", minTravel, atTarget, overShot)
					sb.logger.CDebugf(ctx, "angleDeg %.2f", angleDeg)
					sb.logger.CDebugf(ctx,
						"currYaw %.2f, startYaw %.2f, targetYaw %.2f",
						currYaw, startYaw, targetYaw)
				}

				// poll the sensor for the current error in angle
				// check if we've overshot our target by the errTarget value
				// check if we've travelled at all
				if minTravel && (atTarget || overShot) {
					if sensorDebug {
						sb.logger.CDebugf(ctx,
							"stopping base with errAngle:%.2f, overshot? %t",
							math.Abs(targetYaw-currYaw), overShot)
					}

					if err := sb.Stop(ctx, nil); err != nil {
						return
					}
				}

				if time.Since(startTime) > timeOut {
					sb.logger.CWarn(ctx, "exceeded time for Spin call, stopping base")
					if err := sb.Stop(ctx, nil); err != nil {
						return
					}
					return
				}
			}
		}
	}, sb.activeBackgroundWorkers.Done)
	return nil
}

func getTurnState(currYaw, startYaw, targetYaw, dir, angleDeg, errorBound float64) (atTarget, overShot, minTravel bool) {
	atTarget = math.Abs(targetYaw-currYaw) < errorBound
	overShot = hasOverShot(currYaw, startYaw, targetYaw, dir)
	travelIncrement := math.Abs(angleDeg * increment)
	// check the case where we're asking for a 360 degree turn, this results in a zero travelIncrement
	if math.Abs(angleDeg) < 360 {
		minTravel = math.Abs(currYaw-startYaw) > travelIncrement
	} else {
		minTravel = false
	}
	return atTarget, overShot, minTravel
}

func getCurrentYaw(ms movementsensor.MovementSensor,
) (float64, error) {
	ctx, done := context.WithCancel(context.Background())
	defer done()
	orientation, err := ms.Orientation(ctx, nil)
	if err != nil {
		return math.NaN(), err
	}
	// add Pi to make the computation for overshoot simpler
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
	dir := 1.0
	if math.Signbit(degsPerSec) != math.Signbit(angleDeg) {
		// both positive or both negative -> counterclockwise spin call
		// counterclockwise spin calls add angles
		// the signs being different --> clockwise spin call
		// clockwise spin calls subtract angles
		dir = -1.0
	}
	targetYaw := addAnglesInDomain(angleDeg, currYaw)
	fullTurns := int(math.Abs(angleDeg)) / oneTurn

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
	case dir == -1 && start > target: // clockwise
		// for cases with a quadrant switch from 1 <-> 4
		// check if the current angle is in the regions before the
		// target and after the start
		over := angleBetween(angle, target, 0) || angleBetween(angle, 360, start)
		return over
	case dir == -1 && target > start:
		// the overshoot range is the inside range between the start and target
		return angleBetween(angle, target, start)
	case dir == 1 && start > target: // counterclockwise
		// for cases with a quadrant switch from 1 <-> 4
		// check if the current angle is not in the regions after the
		// target and before the start
		over := !angleBetween(angle, 0, target) && !angleBetween(angle, start, 360)
		return over
	default:
		// the overshoot range is the range of angles outside the start and target ranges
		return !angleBetween(angle, start, target)
	}
}
