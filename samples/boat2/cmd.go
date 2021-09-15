// Package main is the work-in-progress robotic boat from Viam.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/go-errors/errors"
	"go.uber.org/multierr"

	"go.viam.com/utils"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/serial"
	"go.viam.com/core/web"
	webserver "go.viam.com/core/web/server"

	_ "go.viam.com/core/board/detector"

	"github.com/adrianmo/go-nmea"
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

	squirt, steering, thrust board.Motor
	middle                   float64
	steeringRange            float64
}

func (b *boat) Off(ctx context.Context) error {
	return multierr.Combine(
		b.thrust.Off(ctx),
		b.squirt.Off(ctx),
	)
}

func newBoat(ctx context.Context, myRobot robot.Robot) (*boat, error) {
	b := &boat{myRobot: myRobot}

	bb, ok := myRobot.BoardByName("local")
	if !ok {
		return nil, errors.New("no local board")
	}
	b.rc = &rcRemoteControl{bb}

	// get all motors

	b.squirt, ok = bb.MotorByName("squirt")
	if !ok {
		return nil, errors.New("no squirt motor")
	}

	b.steering, ok = bb.MotorByName("steering")
	if !ok {
		return nil, errors.New("no steering motor")
	}

	b.thrust, ok = bb.MotorByName("thrust")
	if !ok {
		return nil, errors.New("no thrust motor")
	}

	err := b.Off(ctx)
	if err != nil {
		return nil, err
	}

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

	return b, multierr.Combine(b.thrust.Off(ctx), b.steering.GoTo(ctx, 50, b.middle))
}

func runRC(ctx context.Context, myBoat *boat) {
	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return
		}

		vals, err := myBoat.rc.Signals(ctx, []string{"throttle", "direction", "speed", "mode"})
		if err != nil {
			logger.Errorw("error getting rc signal %w", err)
			continue
		}
		//logger.Debugf("vals: %v", vals)

		if vals["mode"] <= 1 {
			continue
		}

		squirtPower := float32(vals["throttle"]) / 100.0
		err = myBoat.squirt.Power(ctx, squirtPower)
		if err != nil {
			logger.Errorw("error turning on squirt: %w", err)
			continue
		}

		dir := (myBoat.steeringRange * float64(vals["direction"]) / 100.0)
		dir *= .5 // was too aggressive
		dir += myBoat.middle
		//logger.Debugf("dir: %v", dir)
		err = myBoat.steering.GoTo(ctx, 50, dir)
		if err != nil {
			logger.Errorw("error turning: %w", err)
			continue
		}

		speed := float32(vals["speed"]) / 100.0
		speedDir := pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
		if speed < 0 {
			speed *= -1
			speedDir = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
		}
		//fmt.Printf("speedDir: %v speed: %v\n", speedDir, speed)
		err = myBoat.thrust.Go(ctx, speedDir, speed)
		if err != nil {
			logger.Errorw("error thrusting: %w", err)
			continue
		}

	}
}

var currentLocation nmea.GLL

func trackGPS(ctx context.Context) {
	dev, err := serial.Open("/dev/ttyAMA1")
	if err != nil {
		logger.Debugf("canot open gps serial %s", err)
		return
	}
	defer dev.Close()

	r := bufio.NewReader(dev)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := r.ReadString('\n')
		if err != nil {
			logger.Fatalf("can't read gps serial %s", err)
		}

		s, err := nmea.Parse(line)
		if err != nil {
			logger.Debugf("can't parse nmea %s : %s", line, err)
			continue
		}

		gll, ok := s.(nmea.GLL)
		if ok {
			currentLocation = gll
			fmt.Println(currentLocation)
		}
	}
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()

	cfg, err := config.Read(flag.Arg(0))
	if err != nil {
		return err
	}

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	b, err := newBoat(ctx, myRobot)
	if err != nil {
		return err
	}

	go runRC(ctx, b)
	go trackGPS(ctx)

	if err := webserver.RunWeb(ctx, myRobot, web.NewOptions(), logger); err != nil && !errors.Is(err, context.Canceled) {
		logger.Errorw("error running web", "error", err)
		cancel()
		return err
	}
	return nil
}
