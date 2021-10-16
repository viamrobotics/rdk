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

	"github.com/go-errors/errors"
	slib "github.com/jacobsa/go-serial/serial"
	"go.uber.org/multierr"

	"go.viam.com/utils"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/imu"
	"go.viam.com/core/serial"
	"go.viam.com/core/services/navigation"
	"go.viam.com/core/spatialmath"
	coreutils "go.viam.com/core/utils"
	"go.viam.com/core/web"
	webserver "go.viam.com/core/web/server"

	_ "go.viam.com/core/board/detector"

	"github.com/edaniels/golog"
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

	lastDir  float64
	lastSpin float64
}

func (b *boat) Stop(ctx context.Context) error {
	return multierr.Combine(
		b.starboard.Off(ctx),
		b.port.Off(ctx),
		b.thrust.Off(ctx),
		b.squirt.Off(ctx),
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
	thrustDir pb.DirectionRelative,
	thrustSpeed float32,
	portDir pb.DirectionRelative,
	portSpeed float32,
	starboardDir pb.DirectionRelative,
	starboardSpeed float32) error {
	if false {
		fmt.Printf("SteerAndMoveHelp %v %0.2f %v %0.2f %v %0.2f\n",
			thrustDir,
			thrustSpeed,
			portDir,
			portSpeed,
			starboardDir,
			starboardSpeed)
	}
	return multierr.Combine(
		b.thrust.Go(ctx, thrustDir, thrustSpeed),
		b.port.Go(ctx, portDir, portSpeed),
		b.starboard.Go(ctx, starboardDir, starboardSpeed),
	)

}

// dir -1 -> 1 : -1 = hard left 1 = hard right
// speed -1 -> 1 : 0 means stop, 1 is forward, -1 is backwards
func (b *boat) SteerAndMove(ctx context.Context, dir, speed float64) error {
	if false { // using column
		return b.steerColumn(ctx, dir)
	}

	if false {
		fmt.Printf("SteerAndMove %0.2f %0.2f \n", dir, speed)
	}

	if speed > .4 {
		// forwards

		if dir > 0 {
			return b.SteerAndMoveHelp(ctx,
				pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, float32(speed),
				pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, float32(speed),
				pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, float32(math.Max(0, speed-dir)))
		}
		dir *= -1
		return b.SteerAndMoveHelp(ctx,
			pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, float32(speed),
			pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, float32(math.Max(0, speed-dir)),
			pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, float32(speed),
		)
	}

	if speed < -.4 {
		speed *= -1
		// backwards
		if dir < 0 {
			dir *= -1
			return b.SteerAndMoveHelp(ctx,
				pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, float32(speed),
				pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, float32(speed),
				pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, float32(math.Max(0, speed-dir)),
			)
		}

		return b.SteerAndMoveHelp(ctx,
			pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, float32(speed),
			pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, float32(math.Max(0, speed-dir)),
			pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, float32(speed),
		)
	}

	// we really want to spin with a little straight movement

	//fmt.Printf("spinning\n")
	if dir > 0 {
		return multierr.Combine(
			b.thrust.Off(ctx),
			b.port.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, float32(dir)),
			b.starboard.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, float32(dir)),
		)
	}

	dir *= -1
	return multierr.Combine(
		b.thrust.Off(ctx),
		b.port.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, float32(dir)),
		b.starboard.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, float32(dir)),
	)
}

func newBoat(ctx context.Context, r robot.Robot, c config.Component, logger golog.Logger) (base.Base, error) {
	var err error
	b := &boat{myRobot: r}

	bb, ok := r.BoardByName("local")
	if !ok {
		return nil, errors.New("no local board")
	}
	b.rc = &rcRemoteControl{bb}

	tempIMU, ok := r.SensorByName("imu")
	if !ok {
		return nil, errors.New("need imu")
	}
	b.myImu, ok = tempIMU.(imu.IMU)
	if !ok {
		return nil, fmt.Errorf("wanted an imu but got an %T %#v", tempIMU, tempIMU)
	}

	if true {
		go func() {
			for {
				if !utils.SelectContextOrWait(ctx, 1000*time.Millisecond) {
					return
				}

				r, err := b.myImu.Readings(ctx)
				if err != nil {
					fmt.Println(err)
					continue
				}
				fmt.Printf("%#v\n", r)
			}
		}()
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
		err = b.steering.GoTillStop(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 50, nil)
		if err != nil {
			return nil, err
		}

		bwdLimit, err := b.steering.Position(ctx)
		if err != nil {
			return nil, err
		}

		err = b.steering.GoTillStop(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 50, nil)
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

		err = multierr.Combine(b.thrust.Off(ctx), b.steering.GoTo(ctx, 50, b.middle))
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}

func (b *boat) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
	speed := 0.7
	if distanceMillis >= 9*1000 {
		speed = 1.0
	}

	dir := 0.0
	if true {
		dir = b.lastSpin / 180.0
		if math.Abs(b.lastSpin) > 90 {
			speed = 0.45
		}

		dir *= .2 // trying more aggresssive and it'll go back and forth
	}

	//fmt.Printf("MoveStraight steeringDir: %0.2f speed: %v distanceMillis: %v lastSpin: %v\n", steeringDir, speed, distanceMillis, b.lastSpin)
	return 0, b.SteerAndMove(ctx, dir, speed)
}

func (b *boat) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
	b.lastSpin = angleDeg

	if false {
		steeringDir := angleDeg / 180.0
		if math.Abs(angleDeg) < 90 {
			steeringDir *= .5
		}

		logger.Debugf("steeringDir: %0.2f", steeringDir)

		err := b.SteerAndMove(ctx, steeringDir, .5) // TODO(erh) .5 is totally wrong
		if err != nil {
			return 0, err
		}
	}

	if angleDeg < 3 && angleDeg > -3 {
		return 0, nil
	}

	if false {
		// dumb spin
		dir := 1.0
		if angleDeg < 0 {
			dir *= -1
		}

		err := b.SteerAndMove(ctx, dir, 0)
		if err != nil {
			return 0, err
		}

		duration := 5 * time.Millisecond * time.Duration(math.Abs(angleDeg))
		fmt.Printf("duration: %v\n", duration)
		if !utils.SelectContextOrWait(ctx, duration) {
			return 0, nil
		}

		err = b.SteerAndMove(ctx, dir*-1, 0)
		if err != nil {
			return 0, err
		}

		if !utils.SelectContextOrWait(ctx, 20*time.Millisecond) {
			return 0, nil
		}

		return 0, b.Stop(ctx)
	}

	if false { // try to spin now
		fmt.Printf("want to turn %v\n", angleDeg)
		start, err := b.myImu.Orientation(ctx)
		if err != nil {
			return 0, err
		}
		for i := 0; i < 100; i++ {
			dir := .1
			if angleDeg < 0 {
				dir *= -1
			}
			err := b.SteerAndMove(ctx, dir, 0)
			if err != nil {
				return 0, err
			}

			if !utils.SelectContextOrWait(ctx, 100*time.Millisecond) {
				return 0, nil
			}

			now, err := b.myImu.Orientation(ctx)
			if err != nil {
				return 0, err
			}

			left := math.Abs(coreutils.AngleDiffDeg(start.EulerAngles().Roll, now.EulerAngles().Roll) - angleDeg)
			fmt.Printf("\t left %v\n", left)
			if left < 5 {
				return 0, b.Stop(ctx)
			}
		}
	}

	return 0, nil
}

func (b *boat) WidthMillis(ctx context.Context) (int, error) {
	return 600, nil
}

func (b *boat) Close() error {
	return b.Stop(context.Background())
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
		//logger.Debugf("vals: %v", vals)

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

			now, err := myBoat.myImu.Orientation(ctx)
			if err != nil {
				logger.Errorw("error getting orientation: %w", err)
				continue
			}

			if !previousPushMode {
				pushDirection = now.EulerAngles().Roll
			}
			previousPushMode = true

			delta := pushDirection - now.EulerAngles().Roll

			steer := .5 * (delta / 180)
			fmt.Printf("pushDirection: %0.1f now: %0.1f delta: %0.2f steer: %.2f\n",
				pushDirection, now.EulerAngles().Roll, delta, steer)

			err = multierr.Combine(
				myBoat.SteerAndMove(ctx, steer, 1.0),
				myBoat.squirt.Power(ctx, 1.0),
			)
			if err != nil {
				logger.Errorw("error in push mode: %w", err)
			}
			continue
		}
		previousPushMode = false

		squirtPower := float32(vals["throttle"]) / 100.0
		err = myBoat.squirt.Power(ctx, squirtPower)
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

func newArduinoIMU(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
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

	name := pcs[0]
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

	if name == "Orient" {
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

func (i *myIMU) AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	return i.angularVelocity, i.lastError
}
func (i *myIMU) Orientation(ctx context.Context) (spatialmath.Orientation, error) {
	return &i.orientation, i.lastError
}

func (i *myIMU) Readings(ctx context.Context) ([]interface{}, error) {
	return []interface{}{i.angularVelocity, i.orientation}, i.lastError
}

func (i *myIMU) Desc() sensor.Description {
	return sensor.Description{imu.Type, ""}
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()

	cfg, err := config.Read(flag.Arg(0))
	if err != nil {
		return err
	}

	// register boat as base properly
	registry.RegisterBase("viam-boat2", registry.Base{Constructor: newBoat})

	registry.RegisterSensor(imu.Type, "temp-imu", registry.Sensor{Constructor: newArduinoIMU})

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	b, ok := myRobot.BaseByName("base1")
	if !ok {
		return errors.New("no base")
	}
	myB := coreutils.UnwrapProxy(b).(*boat)

	navServiceTemp, ok := myRobot.ServiceByName("navigation")
	if !ok {
		return errors.New("no navigation service")
	}
	myB.navService, ok = navServiceTemp.(navigation.Service)
	if !ok {
		return errors.New("navigation service isn't a nav service")
	}

	go runRC(ctx, myB)
	//go runAngularVelocityKeeper(ctx, myB)

	if err := webserver.RunWeb(ctx, myRobot, web.NewOptions(), logger); err != nil && !errors.Is(err, context.Canceled) {
		logger.Errorw("error running web", "error", err)
		cancel()
		return err
	}
	return nil
}
