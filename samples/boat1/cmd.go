package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"

	"go.uber.org/multierr"
)

const (
	millisPerRotation = 200
)

type Boat struct {
	theBoard        board.Board
	starboard, port board.Motor

	throttle, direction board.DigitalInterrupt
}

func (b *Boat) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
	dir := board.DirForward
	if distanceMillis < 0 {
		dir = board.DirBackward
		distanceMillis *= -1
	}

	if block {
		return fmt.Errorf("boat can't block for move straight yet")
	}

	speed := (millisPerSec * 60.0) / float64(millisPerRotation)
	rotations := float64(distanceMillis) / millisPerRotation

	return multierr.Combine(
		b.starboard.GoFor(dir, speed, rotations, false),
		b.port.GoFor(dir, speed, rotations, false),
	)

}

func (b *Boat) Spin(ctx context.Context, angleDeg float64, speed int, block bool) error {
	return fmt.Errorf("boat can't spin yet")
}

func (b *Boat) WidthMillis(ctx context.Context) (int, error) {
	return 1, nil
}

func (b *Boat) Stop(ctx context.Context) error {
	return multierr.Combine(b.starboard.Off(), b.port.Off())
}

func (b *Boat) Close(ctx context.Context) error {
	return b.Stop(ctx)
}

func (b *Boat) StartRC() {
	go func() {
		for {

			port := 285 * (float64(b.throttle.Value()) / 90)
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
					b.starboard.GoFor(board.DirForward, starboard, 0, false),
					b.port.GoFor(board.DirForward, port, 0, false),
				)
			}

			if err != nil {
				log.Print(err)
			}

			time.Sleep(10 * time.Millisecond)
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

	if b.throttle == nil || b.direction == nil {
		return nil, fmt.Errorf("need a throttle and direction")
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

	myRobot, err := robot.NewRobot(context.Background(), cfg)
	if err != nil {
		return err
	}
	defer myRobot.Close(context.Background())

	boat, err := NewBoat(myRobot)
	if err != nil {
		return err
	}
	boat.StartRC()

	myRobot.AddBase(boat, api.Component{Name: "boatbot"})

	return web.RunWeb(myRobot, web.NewOptions())
}
