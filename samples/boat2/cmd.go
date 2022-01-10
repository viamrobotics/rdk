// Package main is the work-in-progress robotic boat from Viam.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/edaniels/golog"
	slib "github.com/jacobsa/go-serial/serial"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/serial"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/services/web"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
	webserver "go.viam.com/rdk/web/server"
)

var logger = golog.NewDevelopmentLogger("boat2")

type remoteControl interface {
	Signal(ctx context.Context, name string) (int64, error)
	Signals(ctx context.Context, name []string) (map[string]int64, error)
}

type rcRemoteControl struct {
	theBoard board.Board
}

func (rc *rcRemoteControl) Signal(ctx context.Context, name string) (int64, error) {
	r, ok := rc.theBoard.DigitalInterruptByName(name)
	if !ok {
		return 0, fmt.Errorf("no signal named %s", name)
	}
	return r.Value(ctx)
}

func (rc *rcRemoteControl) Signals(ctx context.Context, names []string) (map[string]int64, error) {
	m := map[string]int64{}

	for _, n := range names {
		val, err := rc.Signal(ctx, n)
		if err != nil {
			return nil, fmt.Errorf("cannot read value of %s %w", n, err)
		}
		m[n] = val
	}

	return m, nil
}

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

type boat struct {
	myRobot robot.Robot
	rc      remoteControl
	myImu   imu.IMU

	squirt, steering, thrust motor.Motor
	starboard, port          motor.Motor
	middle                   float64
	steeringRange            float64

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
	thrustSpeed float64,
	portSpeed float64,
	starboardSpeed float64) error {
	thrustSpeed = math.Max(0, thrustSpeed)

	thrustSpeed = math.Min(1, thrustSpeed)
	portSpeed = math.Min(1, portSpeed)
	starboardSpeed = math.Min(1, starboardSpeed)

	if false {
		logger.Infof("SteerAndMoveHelp %0.2f %0.2f %0.2f\n", thrustSpeed, portSpeed, starboardSpeed)
	}
	return multierr.Combine(
		b.thrust.Go(ctx, thrustSpeed),
		b.port.Go(ctx, portSpeed),
		b.starboard.Go(ctx, starboardSpeed),
	)
}

// dir -1 -> 1 : -1 = hard left 1 = hard right
// speed -1 -> 1 : 0 means stop, 1 is forward, -1 is backwards.
func (b *boat) SteerAndMove(ctx context.Context, dir, speed float64) error {
	if false { // using column
		return b.steerColumn(ctx, dir)
	}

	if false {
		logger.Infof("SteerAndMove %0.2f %0.2f \n", dir, speed)
	}

	if speed > 0.4 {
		// forwards
		if dir < 0 {
			return b.SteerAndMoveHelp(ctx, speed-dir/3, speed, math.Max(0, speed-dir*1.5))
		}
		dir *= -1
		return b.SteerAndMoveHelp(ctx, speed-dir/3, math.Max(0, speed-dir*1.5), speed)
	}

	if speed < -0.4 {
		speed *= -1
		// backwards
		if dir < 0 {
			dir *= -1
			return b.SteerAndMoveHelp(ctx, speed, speed, math.Max(0, speed-dir))
		}

		return b.SteerAndMoveHelp(ctx, speed, math.Max(0, speed-dir), speed)
	}

	// we really want to spin with a little straight movement

	if dir > 0 {
		return multierr.Combine(
			b.thrust.Stop(ctx),
			b.port.Go(ctx, dir),
			b.starboard.Go(ctx, dir),
		)
	}

	dir *= -1
	return multierr.Combine(
		b.thrust.Stop(ctx),
		b.port.Go(ctx, dir),
		b.starboard.Go(ctx, dir),
	)
}

func newBoat(ctx context.Context, r robot.Robot, logger golog.Logger) (base.LocalBase, error) {
	var err error
	b := &boat{myRobot: r}

	bb, ok := r.BoardByName("local")
	if !ok {
		return nil, errors.New("no local board")
	}
	b.rc = &rcRemoteControl{bb}

	b.myImu, ok = imu.FromRobot(r, "imu")
	if !ok {
		return nil, errors.New("'imu' not found or not an IMU")
	}

	// get all motors

	b.squirt, ok = r.MotorByName("squirt")
	if !ok {
		return nil, errors.New("no squirt motor")
	}

	b.steering, ok = r.MotorByName("steering")
	if !ok {
		return nil, errors.New("no steering motor")
	}

	b.thrust, ok = r.MotorByName("thrust")
	if !ok {
		return nil, errors.New("no thrust motor")
	}

	b.starboard, ok = r.MotorByName("starboard")
	if !ok {
		return nil, errors.New("no starboard motor")
	}

	b.port, ok = r.MotorByName("port")
	if !ok {
		return nil, errors.New("no port motor")
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

		bwdLimit, err := b.steering.Position(ctx)
		if err != nil {
			return nil, err
		}

		err = b.steering.GoTillStop(ctx, 50, nil)
		if err != nil {
			return nil, err
		}

		fwdLimit, err := b.steering.Position(ctx)
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

func runRC(ctx context.Context, myBoat *boat) {
	previousPushMode := false
	pushDirection := 0.0

	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return
		}

		vals, err := myBoat.rc.Signals(ctx, []string{"throttle", "direction", "speed", "mode", "left-horizontal", "a"})
		if err != nil {
			logger.Errorw("error getting rc signal %w", err)
			continue
		}
		// logger.Debugf("vals: %v", vals)

		if vals["mode"] <= 1300 {
			err = myBoat.navService.SetMode(ctx, navigation.ModeWaypoint)
			if err != nil {
				logger.Errorw("error setting mode: %w", err)
			}
			continue
		}
		err = myBoat.navService.SetMode(ctx, navigation.ModeManual)
		if err != nil {
			logger.Errorw("error setting mode: %w", err)
		}

		if vals["mode"] <= 1800 {
			continue
		}

		if vals["a"] < 1500 {
			// push mode

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

			err = multierr.Combine(
				myBoat.SteerAndMove(ctx, steer, 1.0),
				myBoat.squirt.SetPower(ctx, 1.0),
			)
			if err != nil {
				logger.Errorw("error in push mode: %w", err)
			}
			continue
		}
		previousPushMode = false

		squirtPower := float64(vals["throttle"]) / 100.0
		err = myBoat.squirt.SetPower(ctx, squirtPower)
		if err != nil {
			logger.Errorw("error turning on squirt: %w", err)
			continue
		}

		direction := float64(vals["direction"]) / 100.0
		speed := float64(vals["speed"]) / 100.0

		err = myBoat.SteerAndMove(ctx, direction, speed)
		if err != nil {
			logger.Errorw("error moving: %w", err)
			continue
		}
	}
}

func newArduinoIMU(ctx context.Context) (sensor.Sensor, error) {
	options := slib.OpenOptions{
		BaudRate:        115200,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	ds := serial.Search(serial.SearchFilter{serial.TypeArduino})
	if len(ds) != 1 {
		return nil, fmt.Errorf("found %d arduinos", len(ds))
	}
	options.PortName = ds[0].Path

	port, err := slib.Open(options)
	if err != nil {
		return nil, err
	}

	portReader := bufio.NewReader(port)

	i := &myIMU{}

	go func() {
		defer port.Close()

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := portReader.ReadString('\n')
			if err != nil {
				i.lastError = err
			} else {
				i.lastError = i.parse(line)
			}
		}
	}()

	return i, nil
}

type myIMU struct {
	angularVelocity spatialmath.AngularVelocity
	orientation     spatialmath.EulerAngles
	lastError       error
}

func (i *myIMU) parse(line string) error {
	line = strings.TrimSpace(line)
	line = strings.ReplaceAll(line, " ", "")
	line = strings.ReplaceAll(line, "\t", "")

	pcs := strings.Split(line, ":")
	if len(pcs) != 2 {
		// probably init
		return nil
	}

	pcs = strings.Split(pcs[1], "|")
	if len(pcs) != 3 {
		return fmt.Errorf("bad line %s", line)
	}

	x, err := strconv.ParseFloat(pcs[0][2:], 64)
	if err != nil {
		return fmt.Errorf("bad line %s", line)
	}

	y, err := strconv.ParseFloat(pcs[1][2:], 64)
	if err != nil {
		return fmt.Errorf("bad line %s", line)
	}

	z, err := strconv.ParseFloat(pcs[2][2:], 64)
	if err != nil {
		return fmt.Errorf("bad line %s", line)
	}

	if name := pcs[0]; name == "Orient" {
		// TODO: not sure if units are right, but docs say the raw data is euler
		i.orientation.Roll = x
		i.orientation.Pitch = y
		i.orientation.Yaw = z
	} else if name == "Gyro" {
		// TODO: not sure if units are right
		i.angularVelocity.X = x
		i.angularVelocity.Y = y
		i.angularVelocity.Z = z
	}

	return nil
}

func (i *myIMU) ReadAngularVelocity(_ context.Context) (spatialmath.AngularVelocity, error) {
	return i.angularVelocity, i.lastError
}

func (i *myIMU) Orientation(_ context.Context) (spatialmath.Orientation, error) {
	return &i.orientation, i.lastError
}

func (i *myIMU) GetReadings(_ context.Context) ([]interface{}, error) {
	return []interface{}{i.angularVelocity, i.orientation}, i.lastError
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

	cfg, err := config.Read(ctx, flag.Arg(0), logger)
	if err != nil {
		return err
	}

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

	registry.RegisterComponent(imu.Subtype, "temp-imu", registry.Component{
		Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newArduinoIMU(ctx)
		},
	})

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close(ctx)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	b, ok := myRobot.BaseByName("base1")
	if !ok {
		return errors.New("no base")
	}
	myB, ok := rdkutils.UnwrapProxy(b).(*boat)
	if !ok {
		return rdkutils.NewUnexpectedTypeError(myB, rdkutils.UnwrapProxy(b))
	}

	navServiceTemp, ok := myRobot.ResourceByName(navigation.Name)
	if !ok {
		return errors.New("no navigation service")
	}
	myB.navService, ok = navServiceTemp.(navigation.Service)
	if !ok {
		return errors.New("navigation service isn't a nav service")
	}

	go runRC(ctx, myB)
	go runAngularVelocityKeeper(ctx, myB)

	return webserver.RunWeb(ctx, myRobot, web.NewOptions(), logger)
}
