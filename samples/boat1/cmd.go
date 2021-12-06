// Package main is the work-in-progress robotic boat from Viam.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/component/motor"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/rlog"
	"go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/sensor"
	"go.viam.com/core/serial"
	"go.viam.com/core/services/web"
	webserver "go.viam.com/core/web/server"

	"github.com/adrianmo/go-nmea"
	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

const (
	millisPerRotation = 200
	maxRPM            = 600.0
)

var logger = golog.NewDevelopmentLogger("boat1")

// Boat TODO
type Boat struct {
	theBoard        board.Board
	starboard, port motor.Motor

	throttle, direction, mode, aSwitch board.DigitalInterrupt
	rightVertical, rightHorizontal     board.DigitalInterrupt
	activeBackgroundWorkers            *sync.WaitGroup

	cancel    func()
	cancelCtx context.Context
}

// MoveStraight TODO
func (b *Boat) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
	dir := pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
	if distanceMillis < 0 {
		dir = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
		distanceMillis *= -1
	}

	if block {
		return 0, errors.New("boat can't block for move straight yet")
	}

	speed := (millisPerSec * 60.0) / float64(millisPerRotation)
	rotations := float64(distanceMillis) / millisPerRotation

	// TODO(erh): return how much it actually moved
	return distanceMillis, multierr.Combine(
		b.starboard.GoFor(ctx, dir, speed, rotations),
		b.port.GoFor(ctx, dir, speed, rotations),
	)

}

// MoveArc allows the motion along an arc defined by speed, distance and angular velocity (TBD)
func (b *Boat) MoveArc(ctx context.Context, distanceMillis int, millisPerSec float64, angleDeg float64, block bool) (int, error) {
	return 1, errors.New("boat can't move in arc yet")
}

// Spin TODO
func (b *Boat) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
	return math.NaN(), errors.New("boat can't spin yet")
}

// WidthMillis TODO
func (b *Boat) WidthMillis(ctx context.Context) (int, error) {
	return 1, nil
}

// Stop TODO
func (b *Boat) Stop(ctx context.Context) error {
	return multierr.Combine(b.starboard.Off(ctx), b.port.Off(ctx))
}

// Close TODO
func (b *Boat) Close() error {
	defer b.activeBackgroundWorkers.Wait()
	b.cancel()
	return b.Stop(context.Background())
}

// StartRC TODO
func (b *Boat) StartRC(ctx context.Context) {
	b.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			select {
			case <-b.cancelCtx.Done():
				return
			default:
			}
			if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
				return
			}

			mode, err := b.mode.Value(ctx)
			if err != nil {
				log.Print(err)
				continue
			}
			if mode == 0 {
				continue
			}

			var port, starboard float64

			var portDirection = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
			var starboardDirection = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD

			direction := 0.0

			aSwitchVal, err := b.aSwitch.Value(ctx)
			if err != nil {
				log.Print(err)
				continue
			}
			if aSwitchVal >= 1600 {
				rightVerticalVal, err := b.rightVertical.Value(ctx)
				if err != nil {
					log.Print(err)
					continue
				}
				rightHorizontalVal, err := b.rightHorizontal.Value(ctx)
				if err != nil {
					log.Print(err)
					continue
				}

				port = maxRPM * float64(rightVerticalVal) / 100.0
				starboard = port

				if math.Abs(port) < 10 {
					// either not moving or spin mode

					port = maxRPM * float64(rightHorizontalVal) / 100.0
					starboard = -1 * port
				} else {
					// moving mostly forward or back, but turning a bit
					direction = float64(rightHorizontalVal)
				}

				if port < 0 {
					portDirection = board.FlipDirection(portDirection)
					port = -1 * port
				}
				if starboard < 0 {
					starboardDirection = board.FlipDirection(starboardDirection)
					starboard = -1 * starboard
				}

			} else {
				if mode == 2 {
					portDirection = board.FlipDirection(portDirection)
					starboardDirection = board.FlipDirection(starboardDirection)
				}

				throttleVal, err := b.throttle.Value(ctx)
				if err != nil {
					log.Print(err)
					continue
				}
				directionVal, err := b.direction.Value(ctx)
				if err != nil {
					log.Print(err)
					continue
				}

				port = maxRPM * (float64(throttleVal) / 90)
				starboard = port

				direction = float64(directionVal)

			}

			if direction > 0 {
				// we want to turn towards starboard
				// so we slow down the starboard motor
				starboard *= 1 - (direction / 100.0)
			} else if direction < 0 {
				port *= 1 - (direction / -100.0)
			}

			if port < 8 && starboard < 8 {
				err = b.Stop(ctx)
			} else {
				err = multierr.Combine(
					b.starboard.GoFor(ctx, starboardDirection, starboard, 0),
					b.port.GoFor(ctx, portDirection, port, 0),
				)
			}

			if err != nil {
				log.Print(err)
			}

		}
	}, b.activeBackgroundWorkers.Done)
}

// SavedDepth TODO
type SavedDepth struct {
	Longitude float64
	Latitude  float64
	Depth     float64
	Extra     interface{}
}

func storeAll(docs []SavedDepth) error {
	for _, doc := range docs {
		data, err := json.Marshal(doc)
		if err != nil {
			return err
		}

		_, err = http.Post(
			"https://us-east-1.aws.webhooks.mongodb-realm.com/api/client/v2.0/app/boat1-lwcji/service/http1/incoming_webhook/depthRecord",
			"application/json",
			bytes.NewReader(data))
		if err != nil {
			return err
		}
	}

	return nil
}

var currentLocation nmea.GLL

func trackGPS(ctx context.Context) {
	dev, err := serial.Open("/dev/ttyAMA1")
	if err != nil {
		rlog.Logger.Fatalf("canot open gps serial %s", err)
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
			rlog.Logger.Fatalf("can't read gps serial %s", err)
		}

		s, err := nmea.Parse(line)
		if err != nil {
			rlog.Logger.Debugf("can't parse nmea %s : %s", line, err)
			continue
		}

		gll, ok := s.(nmea.GLL)
		if ok {
			currentLocation = gll
		}
	}
}

var toStore []SavedDepth

func doRecordDepth(ctx context.Context, depthSensor sensor.Sensor) error {
	if currentLocation.Longitude == 0 {
		return errors.New("currentLocation is 0")
	}

	readings, err := depthSensor.Readings(ctx)
	if err != nil {
		return err
	}
	if len(readings) != 1 {
		return errors.Errorf("readings is unexpected %v", readings)
	}

	m := readings[0].(map[string]interface{})

	confidence := m["confidence"].(float64)
	depth := m["distance"].(float64)

	if confidence < 90 {
		rlog.Logger.Debugf("confidence too low, skipping confidence: %v depth: %v", confidence, depth)
		return nil
	}

	d := SavedDepth{currentLocation.Longitude, currentLocation.Latitude, depth, m}

	toStore = append(toStore, d)

	err = storeAll(toStore)
	if err == nil {
		toStore = []SavedDepth{}
	}
	return err
}

func recordDepthWorker(ctx context.Context, depthSensor sensor.Sensor) {
	if depthSensor == nil {
		rlog.Logger.Fatalf("depthSensor cannot be nil")
	}

	for {
		if !utils.SelectContextOrWait(ctx, 5*time.Second) {
			return
		}

		err := doRecordDepth(ctx, depthSensor)
		if err != nil {
			rlog.Logger.Debugf("erorr recording depth %s", err)
		}
	}
}

// newBoat TODO
func newBoat(ctx context.Context, r robot.Robot, c config.Component, logger golog.Logger) (base.Base, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	b := &Boat{activeBackgroundWorkers: &sync.WaitGroup{}, cancelCtx: cancelCtx, cancel: cancel}
	var ok bool
	b.theBoard, ok = r.BoardByName("local")
	if !ok {
		return nil, errors.New("cannot find board")
	}

	b.starboard, ok = r.MotorByName("starboard")
	if !ok {
		return nil, errors.New("need a starboard motor")
	}

	b.port, ok = r.MotorByName("port")
	if !ok {
		return nil, errors.New("need a port motor")
	}

	b.throttle, ok = b.theBoard.DigitalInterruptByName("throttle")
	if !ok {
		return nil, errors.New("need a throttle digital interrupt")
	}

	b.direction, ok = b.theBoard.DigitalInterruptByName("direction")
	if !ok {
		return nil, errors.New("need a direction digital interrupt")
	}

	b.mode, ok = b.theBoard.DigitalInterruptByName("mode")
	if !ok {
		return nil, errors.New("need a mode digital interrupt")
	}

	b.aSwitch, ok = b.theBoard.DigitalInterruptByName("a")
	if !ok {
		return nil, errors.New("need a a digital interrupt")
	}

	b.rightHorizontal, ok = b.theBoard.DigitalInterruptByName("right-horizontal")
	if !ok {
		return nil, errors.New("need a horizontal digital interrupt")
	}

	b.rightVertical, ok = b.theBoard.DigitalInterruptByName("right-vertical")
	if !ok {
		return nil, errors.New("need a vertical digital interrupt")
	}

	b.StartRC(ctx)

	return b, nil
}

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()

	cfg, err := config.Read(flag.Arg(0))
	if err != nil {
		return err
	}

	// register boat as base properly
	registry.RegisterBase("viam-boat1", registry.Base{Constructor: newBoat})

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close()

	depth1, ok := myRobot.SensorByName("depth1")
	if !ok {
		return errors.New("failed to find depth1 sensor")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var activeBackgroundWorkers sync.WaitGroup
	activeBackgroundWorkers.Add(2)
	defer activeBackgroundWorkers.Wait()
	utils.ManagedGo(func() {
		trackGPS(ctx)
	}, activeBackgroundWorkers.Done)
	utils.ManagedGo(func() {
		recordDepthWorker(ctx, depth1)
	}, activeBackgroundWorkers.Done)

	return webserver.RunWeb(ctx, myRobot, web.NewOptions(), logger)
}
