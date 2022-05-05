// Package main is the work-in-progress robotic boat from Viam.
package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/services/web"
	rdkutils "go.viam.com/rdk/utils"
)

var logger = golog.NewDevelopmentLogger("boat2")

// different states used for roboat operation.
const (
	OFFMODE = iota
	MANUALMODE
	ROBOTMODE
	PUSHMODE
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

type boat struct {
	myRobot robot.Robot
	myImu   imu.IMU
	rc      input.Controller

	squirt, thrust  motor.Motor
	starboard, port motor.Motor
	steering        motor.LocalMotor
	middle          float64
	steeringRange   float64

	navService navigation.Service

	lastDir       float64
	lastSpin      float64
	previousSpins []float64
}

func (b *boat) Stop(ctx context.Context) error {
	return multierr.Combine(
		b.starboard.Stop(ctx),
		b.port.Stop(ctx),
		b.thrust.Stop(ctx),
		b.squirt.Stop(ctx),
	)
}

func (b *boat) steerColumn(ctx context.Context, dir float64) error {
	if math.Abs(dir) > .5 {
		dir *= 2
	} else {
		dir *= .7 // was too aggressive
	}

	dir += .12
	dir = b.steeringRange * dir

	dir += b.middle

	rpm := 80.0
	if math.Abs(b.lastDir-dir) < .2 {
		rpm /= 2
	}

	b.lastDir = dir

	return b.steering.GoTo(ctx, rpm, dir)
}

func (b *boat) SteerAndMoveHelp(ctx context.Context,
	thrustPowerPct float64,
	portPowerPct float64,
	starboardPowerPct float64,
) error {
	thrustPowerPct = math.Max(0, thrustPowerPct)

	thrustPowerPct = math.Min(1, thrustPowerPct)
	portPowerPct = math.Min(1, portPowerPct)
	starboardPowerPct = math.Min(1, starboardPowerPct)

	if false {
		logger.Infof("SteerAndMoveHelp %0.2f %0.2f %0.2f\n", thrustPowerPct, portPowerPct, starboardPowerPct)
	}
	return multierr.Combine(
		b.thrust.SetPower(ctx, thrustPowerPct),
		b.port.SetPower(ctx, portPowerPct),
		b.starboard.SetPower(ctx, starboardPowerPct),
	)
}

// dir -1 -> 1 : -1 = hard left 1 = hard right
// pctMaxRPM -1 -> 1 : 0 means stop, 1 is forward, -1 is backwards.
func (b *boat) SteerAndMove(ctx context.Context, dir, pctMaxRPM float64) error {
	if false { // using column
		return b.steerColumn(ctx, dir)
	}

	if false {
		logger.Infof("SteerAndMove %0.2f %0.2f \n", dir, pctMaxRPM)
	}

	if pctMaxRPM > 0.4 {
		// forwards
		if dir < 0 {
			return b.SteerAndMoveHelp(ctx, pctMaxRPM-dir/3, pctMaxRPM, math.Max(0, pctMaxRPM-dir*1.5))
		}
		dir *= -1
		return b.SteerAndMoveHelp(ctx, pctMaxRPM-dir/3, math.Max(0, pctMaxRPM-dir*1.5), pctMaxRPM)
	}

	if pctMaxRPM < -0.4 {
		pctMaxRPM *= -1
		// backwards
		if dir < 0 {
			dir *= -1
			return b.SteerAndMoveHelp(ctx, pctMaxRPM, pctMaxRPM, math.Max(0, pctMaxRPM-dir))
		}

		return b.SteerAndMoveHelp(ctx, pctMaxRPM, math.Max(0, pctMaxRPM-dir), pctMaxRPM)
	}

	// we really want to spin with a little straight movement

	if dir > 0 {
		return multierr.Combine(
			b.thrust.Stop(ctx),
			b.port.SetPower(ctx, dir),
			b.starboard.SetPower(ctx, dir),
		)
	}

	dir *= -1
	return multierr.Combine(
		b.thrust.Stop(ctx),
		b.port.SetPower(ctx, dir),
		b.starboard.SetPower(ctx, dir),
	)
}

func newBoat(ctx context.Context, r robot.Robot, logger golog.Logger) (base.LocalBase, error) {
	var err error
	var ok bool

	b := &boat{myRobot: r}

	b.myImu, err = imu.FromRobot(r, "imu")
	if err != nil {
		return nil, err
	}

	b.rc, err = input.FromRobot(r, "BoatRC")
	if err != nil {
		return nil, err
	}
	// get all motors

	b.squirt, err = motor.FromRobot(r, "squirt")
	if err != nil {
		return nil, errors.Wrap(err, "no squirt motor")
	}

	steeringMotor, err := motor.FromRobot(r, "steering")
	if err != nil {
		return nil, errors.Wrap(err, "no steering motor")
	}
	stoppableMotor, ok := steeringMotor.(motor.LocalMotor)
	if !ok {
		return nil, motor.NewGoTillStopUnsupportedError("steering")
	}
	b.steering = stoppableMotor

	b.thrust, err = motor.FromRobot(r, "thrust")
	if err != nil {
		return nil, errors.Wrap(err, "no thrust motor")
	}

	b.starboard, err = motor.FromRobot(r, "starboard")
	if err != nil {
		return nil, errors.Wrap(err, "no starboard motor")
	}

	b.port, err = motor.FromRobot(r, "port")
	if err != nil {
		return nil, errors.Wrap(err, "no port motor")
	}

	err = b.Stop(ctx)
	if err != nil {
		return nil, err
	}

	if false {
		// calibrate steering
		err = b.steering.GoTillStop(ctx, -50, nil)
		if err != nil {
			return nil, err
		}

		bwdLimit, err := b.steering.GetPosition(ctx)
		if err != nil {
			return nil, err
		}

		err = b.steering.GoTillStop(ctx, 50, nil)
		if err != nil {
			return nil, err
		}

		fwdLimit, err := b.steering.GetPosition(ctx)
		if err != nil {
			return nil, err
		}

		logger.Debugf("bwdLimit: %v fwdLimit: %v", bwdLimit, fwdLimit)

		b.steeringRange = (fwdLimit - bwdLimit) / 2
		b.middle = bwdLimit + b.steeringRange

		if b.steeringRange < 1 {
			return nil, fmt.Errorf("steeringRange only %v", b.steeringRange)
		}

		err = multierr.Combine(b.thrust.Stop(ctx), b.steering.GoTo(ctx, 50, b.middle))
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}

func (b *boat) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, block bool) error {
	speed := 0.7
	if distanceMm >= 9*1000 {
		speed = 1.0
	}

	if true {
		err := b.SteerAndMove(ctx, 0, speed)
		utils.SelectContextOrWait(ctx, 10000*time.Millisecond)
		return err
	}

	if math.Abs(b.lastSpin) > 90 {
		speed = 0.1 // this means spin in place
	}

	dir := b.lastSpin / 180.0

	// if we're not making progress turning towrads are goal, turn more
	last := len(b.previousSpins) - 1
	for i := 0; i < 5; i++ {
		if last-1-i < 0 {
			break
		}

		if b.previousSpins[last-i] >= b.previousSpins[last-1-i] {
			dir *= 1.2 // magic number
		}
	}

	return b.SteerAndMove(ctx, dir, speed)
}

// MoveArc allows the motion along an arc defined by speed, distance and angular velocity (TBD).
func (b *boat) MoveArc(ctx context.Context, distanceMm int, mmPerSec float64, angleDeg float64, block bool) error {
	return errors.New("boat can't move in arc yet")
}

func (b *boat) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
	b.lastSpin = angleDeg
	b.previousSpins = append(b.previousSpins, b.lastSpin)

	if angleDeg < 3 && angleDeg > -3 {
		return nil
	}

	if true { // try to spin now
		logger.Infof("want to turn %v\n", angleDeg)
		start, err := b.myImu.ReadOrientation(ctx)
		if err != nil {
			return err
		}
		startAngle := start.EulerAngles().Yaw

		dir := 1.0
		if angleDeg < 0 {
			dir *= -1
		}
		err = b.SteerAndMove(ctx, dir, 0)
		if err != nil {
			return err
		}

		// chek how much we've spinned till we've spin the righ amount
		for i := 0; i < 1000; i++ {
			if !utils.SelectContextOrWait(ctx, 50*time.Millisecond) {
				return nil
			}

			now, err := b.myImu.ReadOrientation(ctx)
			if err != nil {
				return err
			}

			left := math.Abs(angleDeg) - rdkutils.AngleDiffDeg(startAngle, now.EulerAngles().Yaw)
			logger.Infof("\t left %v (%#v %#v)\n", left, startAngle, now.EulerAngles().Yaw)
			if left < 5 || left > 180 {
				return b.Stop(ctx)
			}
		}
	}

	return nil
}

func (b *boat) GetWidth(ctx context.Context) (int, error) {
	return 600, nil
}

func (b *boat) Close(ctx context.Context) error {
	return b.Stop(ctx)
}

// Do is unimplemented.
func (b *boat) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, errors.New("Do() unimplemented")
}

func runRC2(ctx context.Context, myBoat *boat) {
	var err error

	pushFlag := 0
	navModeNum := OFFMODE
	speed := 0.0
	dir := 0.0
	previousPushMode := false
	pushDirection := 0.0

	err = myBoat.navService.SetMode(ctx, navigation.ModeManual)
	if err != nil {
		logger.Errorw("error setting mode: %w", err)
	}

	repFunc := func(ctx context.Context, event input.Event) {
		switch event.Control {
		case input.ButtonNorth:
			if event.Event == input.ButtonPress {
				// reset pushmode on a mode switch, and make sure we are not overwriting last push
				if navModeNum != PUSHMODE {
					previousPushMode = false
				}
				// cannot access these modes unless the robot is on
				if navModeNum != OFFMODE {
					switch pushFlag {
					case 1:
						// push mode is activated with LT+North
						navModeNum = PUSHMODE
						err = myBoat.navService.SetMode(ctx, navigation.ModeManual)
						if err != nil {
							logger.Errorw("error setting mode: %w", err)
						}
					default:
						switch navModeNum {
						case MANUALMODE:
							// only pressing North will switch between Robot and Manual
							// Can only access ROBOT mode if in manual currently
							navModeNum = ROBOTMODE
							err = myBoat.navService.SetMode(ctx, navigation.ModeWaypoint)
							if err != nil {
								logger.Errorw("error setting mode: %w", err)
							}
						default:
							navModeNum = MANUALMODE
							err = myBoat.navService.SetMode(ctx, navigation.ModeManual)
							if err != nil {
								logger.Errorw("error setting mode: %w", err)
							}
						}
					}
				}
			}
		case input.ButtonLT:
			pushFlag = int(event.Value)
		case input.AbsoluteY:
			speed = -event.Value

		case input.AbsoluteX:
			dir = event.Value

		case input.AbsoluteZ:
			// only squirt if you actually press it
			if event.Value > .75 && navModeNum != OFFMODE {
				myBoat.squirt.SetPower(ctx, event.Value)
			} else {
				myBoat.squirt.SetPower(ctx, 0)
			}

		case input.ButtonStart:
			// if the robot is "on", turn it "off", if off then go into robot mode
			if event.Event == input.ButtonPress {
				if navModeNum != OFFMODE {
					// make sure we are not using waypoints and turn off
					navModeNum = OFFMODE
					err = myBoat.navService.SetMode(ctx, navigation.ModeManual)
					if err != nil {
						logger.Errorw("error setting mode: %w", err)
					}
				} else {
					navModeNum = MANUALMODE
				}
			}
		}
	}

	// Expects auto_reconnect to be set in the config
	controls, err := myBoat.rc.GetControls(ctx)
	if err != nil {
		logger.Error(err)
		return
	}
	for _, control := range controls {
		err = myBoat.rc.RegisterControlCallback(ctx, control, []input.EventType{input.AllEvents}, repFunc)
		if err != nil {
			return
		}
	}
	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return
		}

		switch navModeNum {
		case OFFMODE:

			err = myBoat.SteerAndMove(ctx, 0.0, 0.0)
			if err != nil {
				logger.Errorw("error moving: %w", err)
				continue
			}
		case MANUALMODE:

			err = myBoat.SteerAndMove(ctx, dir, speed)
			if err != nil {
				logger.Errorw("error moving: %w", err)
				continue
			}
		case ROBOTMODE:
			// navservice(waypoint) handles everything
		case PUSHMODE:
			now, err := myBoat.myImu.ReadOrientation(ctx)
			if err != nil {
				logger.Errorw("error getting orientation: %w", err)
				continue
			}

			if !previousPushMode {
				pushDirection = now.EulerAngles().Yaw
			}
			previousPushMode = true

			delta := pushDirection - now.EulerAngles().Yaw

			steer := .5 * (delta / 180)
			logger.Infof("pushDirection: %0.1f now: %0.1f delta: %0.2f steer: %.2f\n",
				pushDirection, now.EulerAngles().Yaw, delta, steer)

			err = myBoat.SteerAndMove(ctx, steer, 1.0)
			if err != nil {
				logger.Errorw("error in push mode: %w", err)
			}
		}

		if err != nil {
			logger.Errorw("error turning on squirt: %w", err)
			continue
		}
	}
}

func runAngularVelocityKeeper(ctx context.Context, myBoat *boat) {
	go func() {
		for {
			if !utils.SelectContextOrWait(ctx, 10*1000*time.Millisecond) {
				return
			}

			r, err := myBoat.myImu.ReadAngularVelocity(ctx)
			if err != nil {
				logger.Infof("error from imu %v\n", err)
				continue
			}

			r2, err := myBoat.myImu.ReadOrientation(ctx)
			if err != nil {
				logger.Infof("error from imu %v\n", err)
				continue
			}

			logger.Infof("imu readings %#v\n\t%#v\n", r, r2)
		}
	}()
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()

	// register boat as base properly
	registry.RegisterComponent(base.Subtype, "viam-boat2", registry.Component{
		Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newBoat(ctx, r, logger)
		},
	})

	cfg, err := config.Read(ctx, flag.Arg(0), logger)
	if err != nil {
		return err
	}
	myRobot, err := robotimpl.RobotFromConfig(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close(ctx)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	b, err := base.FromRobot(myRobot, "base1")
	if err != nil {
		return err
	}
	myB, ok := rdkutils.UnwrapProxy(b).(*boat)
	if !ok {
		return rdkutils.NewUnexpectedTypeError(myB, rdkutils.UnwrapProxy(b))
	}

	navServiceTemp, err := myRobot.ResourceByName(navigation.Name)
	if err != nil {
		return errors.Wrapf(err, "no navigation service")
	}
	myB.navService, ok = navServiceTemp.(navigation.Service)
	if !ok {
		return errors.New("navigation service isn't a nav service")
	}

	go runRC2(ctx, myB)
	go runAngularVelocityKeeper(ctx, myB)

	return web.RunWebWithConfig(ctx, myRobot, cfg, logger)
}
