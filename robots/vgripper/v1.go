package vgripper

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
)

func init() {
	api.RegisterGripper("viam", func(r api.Robot, config api.Component, logger golog.Logger) (api.Gripper, error) {
		b := r.BoardByName("local")
		if b == nil {
			return nil, fmt.Errorf("viam gripper requires a board called local")
		}
		return NewGripperV1(context.TODO(), b, logger)
	})
}

type GripperV1 struct {
	motor    board.Motor
	current  board.AnalogReader
	pressure board.AnalogReader

	encoderGap int64

	defaultSpeed byte

	closeDirection, openDirection pb.DirectionRelative
	logger                        golog.Logger
}

func NewGripperV1(ctx context.Context, theBoard board.Board, logger golog.Logger) (*GripperV1, error) {

	vg := &GripperV1{
		motor:        theBoard.Motor("g"),
		current:      theBoard.AnalogReader("current"),
		pressure:     theBoard.AnalogReader("pressure"),
		defaultSpeed: 64,
		logger:       logger,
	}

	if vg.motor == nil {
		return nil, fmt.Errorf("gripper needs a motor named 'g'")
	}
	supported, err := vg.motor.PositionSupported(ctx)
	if err != nil {
		return nil, err
	}
	if !supported {
		return nil, fmt.Errorf("gripper motor needs to support position")
	}

	if vg.current == nil || vg.pressure == nil {
		return nil, fmt.Errorf("gripper needs a current and a pressure reader")
	}

	// pick a direction and move till it stops
	sideA, hasPressureA, err := vg.moveInDirectionTillWontMoveMore(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
	if err != nil {
		return nil, err
	}

	sideB, hasPressureB, err := vg.moveInDirectionTillWontMoveMore(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
	if err != nil {
		return nil, err
	}

	if hasPressureA == hasPressureB {
		return nil, fmt.Errorf("pressure same open and closed, something is wrong encoer: %d %d", sideA, sideB)
	}

	vg.encoderGap = utils.AbsInt64(sideB - sideA)

	if hasPressureA {
		vg.closeDirection = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
		vg.openDirection = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
	} else {
		vg.closeDirection = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
		vg.openDirection = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
	}

	return vg, vg.Open(ctx)
}

func (vg *GripperV1) Open(ctx context.Context) error {
	_, _, err := vg.moveInDirectionTillWontMoveMore(ctx, vg.openDirection)
	return err
	/*
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
	*/
}

func (vg *GripperV1) Grab(ctx context.Context) (bool, error) {
	_, _, err := vg.moveInDirectionTillWontMoveMore(ctx, vg.closeDirection)
	return false, err

	/*
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
	*/
}

func (vg *GripperV1) Close(ctx context.Context) error {
	return vg.Stop(ctx)
}

func (vg *GripperV1) Stop(ctx context.Context) error {
	return vg.motor.Off(ctx)
}

func (vg *GripperV1) readCurrent(ctx context.Context) (int, error) {
	return vg.current.Read(ctx)
}

func (vg *GripperV1) encoderSame(a, b int64) bool {
	return utils.AbsInt64(b-a) < 5
}

func (vg *GripperV1) readPressure(ctx context.Context) (int, error) {
	return vg.pressure.Read(ctx)
}

func (vg *GripperV1) hasPressure(ctx context.Context) (bool, error) {
	p, err := vg.readPressure(ctx)
	return p < 1000, err
}

// return hasPressure, current
func (vg *GripperV1) analogs(ctx context.Context) (hasPressure bool, current int, err error) {
	hasPressure, err = vg.hasPressure(ctx)
	if err != nil {
		return
	}

	current, err = vg.readCurrent(ctx)
	if err != nil {
		return
	}

	return
}

func (vg *GripperV1) moveInDirectionTillWontMoveMore(ctx context.Context, dir pb.DirectionRelative) (int64, bool, error) {
	defer func() {
		err := vg.Stop(ctx)
		if err != nil {
			vg.logger.Warnf("couldn't stop motor %s", err)
		}
		vg.logger.Debugf("stopped")
	}()

	vg.logger.Debugf("starting to move dir: %v", dir)

	err := vg.motor.Go(ctx, dir, vg.defaultSpeed)
	if err != nil {
		return -1, false, err
	}

	last, err := vg.motor.Position(ctx)
	if err != nil {
		return -1, false, err
	}

	time.Sleep(500 * time.Millisecond)

	for {
		now, err := vg.motor.Position(ctx)
		if err != nil {
			return -1, false, err
		}

		hasPressure, _, err := vg.analogs(ctx)
		if err != nil {
			return -1, false, err
		}

		vg.logger.Debugf("dir: %v last: %v now: %v hasPressure: %v",
			dir, last, now, hasPressure)

		if vg.encoderSame(last, now) || hasPressure {
			// increase power temporarily
			err := vg.motor.Force(ctx, 128)
			if err != nil {
				return -1, false, err
			}
			time.Sleep(2500 * time.Millisecond)
			return now, hasPressure, err
		}
		last = now

		time.Sleep(100 * time.Millisecond)
	}

}
