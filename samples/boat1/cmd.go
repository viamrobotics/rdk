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

	"github.com/adrianmo/go-nmea"
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/serial"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/web"
	rdkutils "go.viam.com/rdk/utils"
	webserver "go.viam.com/rdk/web/server"
)

const (
	millisPerRotation = 200
	maxRPM            = 600.0
)

var logger = golog.NewDevelopmentLogger("boat1")

// Boat TODO.
type Boat struct {
	theBoard        board.Board
	starboard, port motor.Motor

	throttle, direction, mode, aSwitch board.DigitalInterrupt
	rightVertical, rightHorizontal     board.DigitalInterrupt
	activeBackgroundWorkers            *sync.WaitGroup

	cancel    func()
	cancelCtx context.Context
}

// MoveStraight TODO.
func (b *Boat) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, block bool) error {
	if block {
		return errors.New("boat can't block for move straight yet")
	}

	speed := (mmPerSec * 60.0) / float64(millisPerRotation)
	rotations := float64(distanceMm) / millisPerRotation

	return multierr.Combine(
		b.starboard.GoFor(ctx, speed, rotations),
		b.port.GoFor(ctx, speed, rotations),
	)
}

// MoveArc allows the motion along an arc defined by speed, distance and angular velocity (TBD).
func (b *Boat) MoveArc(ctx context.Context, distanceMm int, mmPerSec float64, angleDeg float64, block bool) error {
	return errors.New("boat can't move in arc yet")
}

// Spin TODO.
func (b *Boat) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
	return errors.New("boat can't spin yet")
}

// Stop TODO.
func (b *Boat) Stop(ctx context.Context) error {
	return multierr.Combine(b.starboard.Stop(ctx), b.port.Stop(ctx))
}

// Close TODO.
func (b *Boat) Close(ctx context.Context) error {
	defer b.activeBackgroundWorkers.Wait()
	b.cancel()
	return b.Stop(ctx)
}

// StartRC TODO.
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
			} else {
				if mode == 2 {
					port *= -1
					starboard *= -1
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
					b.starboard.GoFor(ctx, starboard, 0),
					b.port.GoFor(ctx, port, 0),
				)
			}

			if err != nil {
				log.Print(err)
			}
		}
	}, b.activeBackgroundWorkers.Done)
}

// SavedDepth TODO.
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

		resp, err := http.Post(
			"https://us-east-1.aws.webhooks.mongodb-realm.com/api/client/v2.0/app/boat1-lwcji/service/http1/incoming_webhook/depthRecord",
			"application/json",
			bytes.NewReader(data))
		if err != nil {
			return err
		}
		if err := resp.Body.Close(); err != nil {
			rlog.Logger.Error(err)
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

	readings, err := depthSensor.GetReadings(ctx)
	if err != nil {
		return err
	}
	if len(readings) != 1 {
		return errors.Errorf("readings is unexpected %v", readings)
	}

	m, ok := readings[0].(map[string]interface{})
	if !ok {
		return rdkutils.NewUnexpectedTypeError(m, readings[0])
	}

	confidence, ok := m["confidence"].(float64)
	if !ok {
		return rdkutils.NewUnexpectedTypeError(confidence, m["confidence"])
	}
	depth, ok := m["distance"].(float64)
	if !ok {
		return rdkutils.NewUnexpectedTypeError(depth, m["distance"])
	}

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

// newBoat TODO.
func newBoat(ctx context.Context, r robot.Robot) (base.Base, error) {
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

	cfg, err := config.Read(ctx, flag.Arg(0))
	if err != nil {
		return err
	}

	// register boat as base properly
	registry.RegisterComponent(base.Subtype, "viam-boat1", registry.Component{
		Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newBoat(ctx, r)
		},
	})

	myRobot, err := robotimpl.New(ctx, cfg, logger, client.WithDialOptions(rpc.WithInsecure()))
	if err != nil {
		return err
	}
	defer myRobot.Close(ctx)

	depth1, ok := sensor.FromRobot(myRobot, "depth1")
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
