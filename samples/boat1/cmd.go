package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	_ "go.viam.com/robotcore/board/detector"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

const (
	millisPerRotation = 200
)

var logger = golog.NewDevelopmentLogger("boat1")

type Boat struct {
	theBoard        board.Board
	starboard, port board.Motor

	throttle, direction, mode board.DigitalInterrupt
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

func (b *Boat) Spin(ctx context.Context, angleDeg float64, speed int, block bool) (float64, error) {
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

			motorDirection := pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
			if mode == 2 {
				motorDirection = board.FlipDirection(motorDirection)
			}

			port := 600 * (float64(b.throttle.Value()) / 90)
			starboard := port

			direction := b.direction.Value()

			if direction > 0 {
				// we want to turn towards starboard
				// so we slow down the starboard motor
				starboard *= 1 - (float64(direction) / 100.0)
			} else if direction < 0 {
				port *= 1 - (float64(direction) / -100.0)
			}

			var err error

			if port < 5 && starboard < 5 {
				err = b.Stop(context.Background())
			} else {
				err = multierr.Combine(
					b.starboard.GoFor(context.TODO(), motorDirection, starboard, 0),
					b.port.GoFor(context.TODO(), motorDirection, port, 0),
				)
			}

			if err != nil {
				log.Print(err)
			}

		}
	}()
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

	cfg, err := api.ReadConfig("samples/boat1/boat.json")
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

	return web.RunWeb(context.Background(), myRobot, web.NewOptions(), logger)
}
