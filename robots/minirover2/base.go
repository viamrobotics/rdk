package minirover2

import (
	"fmt"
	"math"
	"time"

	"github.com/viamrobotics/robotcore/base"
	"github.com/viamrobotics/robotcore/board"
	"github.com/viamrobotics/robotcore/utils"

	"github.com/edaniels/golog"
)

const ModelName = "minirover2"

const (
	PanCenter  = 83
	TiltCenter = 100

	WheelCircumferenceMM = math.Pi * 150
)

// ------

type Rover struct {
	theBoard board.Board

	fl, fr, bl, br board.Motor
}

func (r *Rover) Close() {
	err := r.Stop()
	if err != nil {
		golog.Global.Warnf("error stopping minirover2 in Close: %s", err)
	}
}

func (r *Rover) MoveStraight(distanceMM int, speed int, block bool) error {
	if distanceMM == 0 && block {
		return fmt.Errorf("cannot block unless you have a distance")
	}

	if distanceMM != 0 && speed <= 0 {
		return fmt.Errorf("if distanceMM is set, speed has to be positive")
	}

	d := "forward"
	if distanceMM < 0 || speed < 0 {
		d = "backward"
		distanceMM = utils.AbsInt(distanceMM)
		speed = utils.AbsInt(speed)
	}

	var err error
	rotations := float64(distanceMM) / WheelCircumferenceMM

	err = utils.CombineErrors(
		r.fl.GoFor(d, byte(speed), rotations, false),
		r.fr.GoFor(d, byte(speed), rotations, false),
		r.bl.GoFor(d, byte(speed), rotations, false),
		r.br.GoFor(d, byte(speed), rotations, false),
	)

	if err != nil {
		return utils.CombineErrors(err, r.Stop())
	}

	if !block {
		return nil
	}

	return r.waitForMotorsToStop()
}

func (r *Rover) Spin(angleDeg float64, speed int, block bool) error {

	if speed < 120 {
		speed = 120
	}

	a, b := "forward", "backward"
	if angleDeg < 0 {
		a, b = "backward", "forward"
	}

	rotations := math.Abs(angleDeg / 5.0)

	err := utils.CombineErrors(
		r.fl.GoFor(a, byte(speed), rotations, false),
		r.fr.GoFor(b, byte(speed), rotations, false),
		r.bl.GoFor(a, byte(speed), rotations, false),
		r.br.GoFor(b, byte(speed), rotations, false),
	)

	if err != nil {
		return utils.CombineErrors(err, r.Stop())
	}

	if !block {
		return nil
	}

	return r.waitForMotorsToStop()
}

func (r *Rover) waitForMotorsToStop() error {
	for {
		time.Sleep(10 * time.Millisecond)

		if r.fl.IsOn() ||
			r.fr.IsOn() ||
			r.bl.IsOn() ||
			r.br.IsOn() {
			continue
		}

		break
	}

	return nil
}

func (r *Rover) Stop() error {
	return utils.CombineErrors(
		r.fl.Off(),
		r.fr.Off(),
		r.bl.Off(),
		r.br.Off(),
	)
}

/*
func (r *Rover) neckCenter() error {
	return r.neckPosition(PanCenter, TiltCenter)
}

func (r *Rover) neckOffset(left int) error {
	return r.neckPosition(PanCenter+(left*-30), TiltCenter-20)
}

func (r *Rover) neckPosition(pan, tilt int) error {
	return r.sendCommand(fmt.Sprintf("p%d\rt%d\r", pan, tilt))
}
*/

func NewRover(theBoard board.Board) (base.Device, error) {
	rover := &Rover{theBoard: theBoard}
	rover.fl = theBoard.Motor("fl-m")
	rover.fr = theBoard.Motor("fr-m")
	rover.bl = theBoard.Motor("bl-m")
	rover.br = theBoard.Motor("br-m")

	if rover.fl == nil || rover.fr == nil || rover.bl == nil || rover.br == nil {
		return nil, fmt.Errorf("missing a motor for minirover2")
	}

	/*
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
			err = rover.neckCenter()
			if err != nil {
				return nil, err
			}
		}
	*/

	golog.Global.Debug("rover ready")

	return rover, nil
}
