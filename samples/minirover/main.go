package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/rlog"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"

	_ "go.viam.com/robotcore/board/detector"
	_ "go.viam.com/robotcore/rimage/imagesource"

	"go.uber.org/multierr"
)

const (
	PanCenter  = 94
	TiltCenter = 113
)

var logger = rlog.Logger.Named("minirover")

// ------

type Rover struct {
	theBoard  board.Board
	pan, tilt board.Servo
}

func (r *Rover) neckCenter() error {
	return r.neckPosition(PanCenter, TiltCenter)
}

func (r *Rover) neckOffset(left int) error {
	return r.neckPosition(uint8(PanCenter+(left*-30)), uint8(TiltCenter-20))
}

func (r *Rover) neckPosition(pan, tilt uint8) error {
	logger.Debugf("neckPosition to %v %v", pan, tilt)
	return multierr.Combine(r.pan.Move(pan), r.tilt.Move(tilt))
}

func (r *Rover) Ready(theRobot api.Robot) error {
	logger.Debugf("minirover2 Ready called")
	cam := theRobot.CameraByName("front")
	if cam == nil {
		return fmt.Errorf("no camera named front")
	}

	// doing this in a goroutine so i can see camera and servo data in web ui, but probably not right long term
	if false {
		go func() {
			for {
				time.Sleep(time.Second)
				var depthErr bool
				func() {
					img, release, err := cam.Next(context.Background())
					if err != nil {
						logger.Debugf("error from camera %s", err)
						return
					}
					defer release()
					pc := rimage.ConvertToImageWithDepth(img)
					if pc.Depth == nil {
						logger.Warnf("no depth data")
						depthErr = true
						return
					}
					err = pc.WriteTo(fmt.Sprintf("data/rover-centering-%d.both.gz", time.Now().Unix()))
					if err != nil {
						logger.Debugf("error writing %s", err)
					}
				}()
				if depthErr {
					return
				}
			}
		}()
	}

	return nil
}

func NewRover(theBoard board.Board) (*Rover, error) {
	rover := &Rover{theBoard: theBoard}
	rover.pan = theBoard.Servo("pan")
	rover.tilt = theBoard.Servo("tilt")

	if false {
		go func() {
			for {
				time.Sleep(1500 * time.Millisecond)
				err := rover.neckCenter()
				if err != nil {
					panic(err)
				}

				time.Sleep(1500 * time.Millisecond)

				err = rover.neckOffset(-1)
				if err != nil {
					panic(err)
				}

				time.Sleep(1500 * time.Millisecond)

				err = rover.neckOffset(1)
				if err != nil {
					panic(err)
				}

			}
		}()
	} else {
		err := rover.neckCenter()
		if err != nil {
			return nil, err
		}
	}

	logger.Debug("rover ready")

	return rover, nil
}

func main() {
	err := realMain()
	if err != nil {
		log.Fatal(err)
	}
}

func realMain() error {
	flag.Parse()

	cfg, err := api.ReadConfig("samples/minirover/config.json")
	if err != nil {
		return err
	}

	myRobot, err := robot.NewRobot(context.Background(), cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close(context.Background())

	rover, err := NewRover(myRobot.BoardByName("local"))
	if err != nil {
		return err
	}
	err = rover.Ready(myRobot)
	if err != nil {
		return err
	}

	return web.RunWeb(myRobot, web.NewOptions(), logger)
}
