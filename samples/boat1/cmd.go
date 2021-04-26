package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	_ "go.viam.com/robotcore/board/detector"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/serial"

	"github.com/adrianmo/go-nmea"
	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

const (
	millisPerRotation = 200
	maxRPM            = 600.0
)

var logger = golog.NewDevelopmentLogger("boat1")

type Boat struct {
	theBoard        board.Board
	starboard, port board.Motor

	throttle, direction, mode, aSwitch board.DigitalInterrupt
	rightVertical, rightHorizontal     board.DigitalInterrupt
}

func (b *Boat) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
	dir := pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
	if distanceMillis < 0 {
		dir = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
		distanceMillis *= -1
	}

	if block {
		return 0, fmt.Errorf("boat can't block for move straight yet")
	}

	speed := (millisPerSec * 60.0) / float64(millisPerRotation)
	rotations := float64(distanceMillis) / millisPerRotation

	// TODO(erh): return how much it actually moved
	return distanceMillis, multierr.Combine(
		b.starboard.GoFor(ctx, dir, speed, rotations),
		b.port.GoFor(ctx, dir, speed, rotations),
	)

}

func (b *Boat) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
	return math.NaN(), fmt.Errorf("boat can't spin yet")
}

func (b *Boat) WidthMillis(ctx context.Context) (int, error) {
	return 1, nil
}

func (b *Boat) Stop(ctx context.Context) error {
	return multierr.Combine(b.starboard.Off(ctx), b.port.Off(ctx))
}

func (b *Boat) Close() error {
	return b.Stop(context.Background())
}

func (b *Boat) StartRC() {
	go func() {
		for {

			time.Sleep(10 * time.Millisecond)

			mode := b.mode.Value()
			if mode == 0 {
				continue
			}

			var port, starboard float64

			var portDirection = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
			var starboardDirection = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD

			direction := 0.0

			if b.aSwitch.Value() >= 1600 {
				port = maxRPM * float64(b.rightVertical.Value()) / 100.0
				starboard = port

				if math.Abs(port) < 10 {
					// either not moving or spin mode
					port = maxRPM * float64(b.rightHorizontal.Value()) / 100.0
					starboard = -1 * port
				} else {
					// moving mostly forward or back, but turning a bit
					direction = float64(b.rightHorizontal.Value())
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

				port = maxRPM * (float64(b.throttle.Value()) / 90)
				starboard = port

				direction = float64(b.direction.Value())

			}

			if direction > 0 {
				// we want to turn towards starboard
				// so we slow down the starboard motor
				starboard *= 1 - (direction / 100.0)
			} else if direction < 0 {
				port *= 1 - (direction / -100.0)
			}

			var err error

			if port < 8 && starboard < 8 {
				err = b.Stop(context.Background())
			} else {
				err = multierr.Combine(
					b.starboard.GoFor(context.TODO(), starboardDirection, starboard, 0),
					b.port.GoFor(context.TODO(), portDirection, port, 0),
				)
			}

			if err != nil {
				log.Print(err)
			}

		}
	}()
}

type SavedDetph struct {
	Longitude float64
	Latitude  float64
	Depth     float64
	Extra     interface{}
}

func storeAll(docs []SavedDetph) error {
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

func trackGPS() {
	dev, err := serial.OpenDevice("/dev/ttyAMA1")
	if err != nil {
		golog.Global.Fatalf("canot open gps serial %s", err)
	}
	defer dev.Close()

	r := bufio.NewReader(dev)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			golog.Global.Fatalf("can't read gps serial %s", err)
		}

		s, err := nmea.Parse(line)
		if err != nil {
			golog.Global.Debugf("can't parse nmea %s : %s", line, err)
			continue
		}

		gll, ok := s.(nmea.GLL)
		if ok {
			currentLocation = gll
		}
	}
}

var toStore []SavedDetph

func doRecordDepth(depthSensor sensor.Device) error {
	if currentLocation.Longitude == 0 {
		return fmt.Errorf("currentLocation is 0")
	}

	readings, err := depthSensor.Readings(context.Background())
	if err != nil {
		return err
	}
	if len(readings) != 1 {
		return fmt.Errorf("readings is unexpected %v", readings)
	}

	m := readings[0].(map[string]interface{})

	confidence := m["confidence"].(float64)
	depth := m["distance"].(float64)

	if confidence < 90 {
		golog.Global.Debugf("confidence too low, skipping confidence: %v depth: %v", confidence, depth)
		return nil
	}

	d := SavedDetph{currentLocation.Longitude, currentLocation.Latitude, depth, m}

	toStore = append(toStore, d)

	err = storeAll(toStore)
	if err == nil {
		toStore = []SavedDetph{}
	}
	return err
}

func recordDepthThread(depthSensor sensor.Device) {
	if depthSensor == nil {
		golog.Global.Fatalf("depthSensor cannot be nil")
	}

	for {
		time.Sleep(5 * time.Second)

		err := doRecordDepth(depthSensor)
		if err != nil {
			golog.Global.Debugf("erorr recording depth %s", err)
		}
	}
}

func NewBoat(robot *robot.Robot) (*Boat, error) {
	b := &Boat{}
	b.theBoard = robot.BoardByName("local")
	if b.theBoard == nil {
		return nil, fmt.Errorf("cannot find board")
	}

	b.starboard = b.theBoard.Motor("starboard")
	b.port = b.theBoard.Motor("port")

	if b.starboard == nil || b.port == nil {
		return nil, fmt.Errorf("need a starboard and port motor")
	}

	b.throttle = b.theBoard.DigitalInterrupt("throttle")
	b.direction = b.theBoard.DigitalInterrupt("direction")
	b.mode = b.theBoard.DigitalInterrupt("mode")
	b.aSwitch = b.theBoard.DigitalInterrupt("a")
	b.rightHorizontal = b.theBoard.DigitalInterrupt("right-horizontal")
	b.rightVertical = b.theBoard.DigitalInterrupt("right-vertical")

	if b.throttle == nil || b.direction == nil || b.mode == nil {
		return nil, fmt.Errorf("need a throttle and direction and mode")
	}

	return b, nil
}

func main() {
	err := realMain()
	if err != nil {
		log.Fatal(err)
	}
}

func realMain() error {
	flag.Parse()

	cfg, err := api.ReadConfig(flag.Arg(0))
	if err != nil {
		return err
	}

	myRobot, err := robot.NewRobot(context.Background(), cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close()

	boat, err := NewBoat(myRobot)
	if err != nil {
		return err
	}
	boat.StartRC()

	myRobot.AddBase(boat, api.Component{Name: "boatbot"})

	go trackGPS()
	go recordDepthThread(myRobot.SensorByName("depth1"))

	return web.RunWeb(context.Background(), myRobot, web.NewOptions(), logger)
}
