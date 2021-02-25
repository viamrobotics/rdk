package gripper

import (
	"fmt"
	"time"

	"github.com/edaniels/golog"

	"github.com/viamrobotics/robotcore/board"
	"github.com/viamrobotics/robotcore/utils"
)

type ViamGripper struct {
	motor         board.Motor
	potentiometer board.AnalogReader
	pressure      board.AnalogReader

	potentiometerOpen   int
	potentiometerClosed int

	defaultSpeed byte

	closeDirection, openDirection board.Direction
}

func NewViamGripper(theBoard board.Board) (*ViamGripper, error) {

	vg := &ViamGripper{
		motor:         theBoard.Motor("g"),
		potentiometer: theBoard.AnalogReader("potentiometer"),
		pressure:      theBoard.AnalogReader("pressure"),
		defaultSpeed:  16,
	}

	if vg.motor == nil {
		return nil, fmt.Errorf("gripper needs a motor named 'g'")
	}

	if vg.potentiometer == nil || vg.pressure == nil {
		return nil, fmt.Errorf("gripper needs a potentiometer and a pressure reader")
	}

	// pick a direction and move till it stops
	sideA, hasPressureA, err := vg.moveInDirectionTillWontMoveMore(board.DirForward)
	if err != nil {
		return nil, err
	}

	sideB, hasPressureB, err := vg.moveInDirectionTillWontMoveMore(board.DirBackward)
	if err != nil {
		return nil, err
	}

	if hasPressureA == hasPressureB {
		return nil, fmt.Errorf("pressure same open and closed, something is wrong potentiometer: %d %d", sideA, sideB)
	}

	if hasPressureA {
		vg.closeDirection = board.DirForward
		vg.openDirection = board.DirBackward
		vg.potentiometerOpen = sideB
		vg.potentiometerClosed = sideA
	} else {
		vg.closeDirection = board.DirBackward
		vg.openDirection = board.DirForward
		vg.potentiometerOpen = sideA
		vg.potentiometerClosed = sideB
	}

	return vg, vg.Open()
}

func (vg *ViamGripper) Open() error {
	err := vg.motor.Go(vg.openDirection, vg.defaultSpeed)
	if err != nil {
		return err
	}

	msPer := 10
	total := 0
	for {
		time.Sleep(time.Duration(msPer) * time.Millisecond)
		now, err := vg.readPotentiometer()
		if err != nil {
			return err
		}
		if vg.potentiometerSame(now, vg.potentiometerOpen) {
			return vg.Stop()
		}

		total += msPer
		if total > 5000 {
			err = vg.Stop()
			return fmt.Errorf("open timed out, wanted: %d at: %d stop error: %s", vg.potentiometerOpen, now, err)
		}
	}

}

func (vg *ViamGripper) Grab() (bool, error) {
	err := vg.motor.Go(vg.closeDirection, vg.defaultSpeed)
	if err != nil {
		return false, err
	}

	msPer := 10
	total := 0
	for {
		time.Sleep(time.Duration(msPer) * time.Millisecond)
		now, err := vg.readPotentiometer()
		if err != nil {
			return false, err
		}

		if vg.potentiometerSame(now, vg.potentiometerClosed) {
			// we fully closed
			return false, vg.Stop()
		}

		pressure, err := vg.hasPressure()
		if err != nil {
			return false, err
		}

		if pressure {
			// don't turn motor off, keep pressure being applied
			return true, nil
		}

		total += msPer
		if total > 5000 {
			err = vg.Stop()
			if err != nil {
				return false, err
			}
			pressureRaw, err := vg.readPressure()
			if err != nil {
				return false, err
			}
			return false, fmt.Errorf("close timed out, wanted: %d at: %d pressure: %d", vg.potentiometerOpen, now, pressureRaw)
		}
	}

}

func (vg *ViamGripper) Close() error {
	return vg.Stop()
}

func (vg *ViamGripper) Stop() error {
	return vg.motor.Off()
}

func (vg *ViamGripper) readPotentiometer() (int, error) {
	return vg.potentiometer.Read()
}

func (vg *ViamGripper) potentiometerSame(a, b int) bool {
	return utils.AbsInt(b-a) < 5
}

func (vg *ViamGripper) readPressure() (int, error) {
	return vg.pressure.Read()
}

func (vg *ViamGripper) hasPressure() (bool, error) {
	p, err := vg.readPressure()
	return p < 1000, err
}

func (vg *ViamGripper) moveInDirectionTillWontMoveMore(dir board.Direction) (int, bool, error) {
	defer func() {
		err := vg.Stop()
		if err != nil {
			golog.Global.Warnf("couldn't stop motor %s", err)
		}
	}()

	err := vg.motor.Go(dir, vg.defaultSpeed)
	if err != nil {
		return -1, false, err
	}

	last, err := vg.readPotentiometer()
	if err != nil {
		return -1, false, err
	}

	time.Sleep(300 * time.Millisecond)

	for {
		now, err := vg.readPotentiometer()
		if err != nil {
			return -1, false, err
		}

		golog.Global.Debugf("dir: %s last: %v now: %v", dir, last, now)
		if vg.potentiometerSame(last, now) {
			// increase power temporarily
			err := vg.motor.Force(128)
			if err != nil {
				return -1, false, err
			}
			time.Sleep(500 * time.Millisecond)
			hasPressure, err := vg.hasPressure()
			return now, hasPressure, err
		}
		last = now

		time.Sleep(100 * time.Millisecond)
	}

}
