package vgripper

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
)

func init() {
	api.RegisterGripper("viam", func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (api.Gripper, error) {
		b := r.BoardByName("local")
		if b == nil {
			return nil, fmt.Errorf("viam gripper requires a board called local")
		}
		return NewGripperV1(ctx, b, config.Attributes.GetInt("pressureLimit", 800), logger)
	})
}

type GripperV1 struct {
	motor    board.Motor
	current  board.AnalogReader
	pressure board.AnalogReader

	openPos, closePos float64

	defaultPowerPct, holdingPressure float32

	pressureLimit int

	closeDirection, openDirection pb.DirectionRelative
	logger                        golog.Logger
}

func NewGripperV1(ctx context.Context, theBoard board.Board, pressureLimit int, logger golog.Logger) (*GripperV1, error) {

	vg := &GripperV1{
		motor:           theBoard.Motor("g"),
		current:         theBoard.AnalogReader("current"),
		pressure:        theBoard.AnalogReader("pressure"),
		defaultPowerPct: 25,
		holdingPressure: 6,
		pressureLimit:   pressureLimit,
		logger:          logger,
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
	posA, hasPressureA, err := vg.moveInDirectionTillWontMoveMore(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
	if err != nil {
		return nil, err
	}

	posB, hasPressureB, err := vg.moveInDirectionTillWontMoveMore(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
	if err != nil {
		return nil, err
	}

	if hasPressureA == hasPressureB {
		return nil, fmt.Errorf("pressure same open and closed, something is wrong encoer: %f %f", posA, posB)
	}

	if hasPressureA {
		vg.closeDirection = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
		vg.openDirection = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
		vg.openPos = posB
		vg.closePos = posA
	} else {
		vg.closeDirection = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
		vg.openDirection = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
		vg.openPos = posA
		vg.closePos = posB

	}

	return vg, vg.Open(ctx)
}

func (vg *GripperV1) Open(ctx context.Context) error {
	err := vg.motor.Go(ctx, vg.openDirection, vg.defaultPowerPct)
	if err != nil {
		return err
	}

	msPer := 10
	total := 0
	for {
		if !utils.SelectContextOrWait(ctx, time.Duration(msPer)*time.Millisecond) {
			return ctx.Err()
		}
		now, err := vg.motor.Position(ctx)
		if err != nil {
			return err
		}
		if vg.encoderSame(now, vg.openPos) {
			return vg.Stop(ctx)
		}

		total += msPer
		if total > 5000 {
			err = vg.Stop(ctx)
			return fmt.Errorf("open timed out, wanted: %f at: %f stop error: %s", vg.openPos, now, err)
		}
	}
}

func (vg *GripperV1) Grab(ctx context.Context) (bool, error) {
	err := vg.motor.Go(ctx, vg.closeDirection, vg.defaultPowerPct)
	if err != nil {
		return false, err
	}

	msPer := 10
	total := 0
	for {
		if !utils.SelectContextOrWait(ctx, time.Duration(msPer)*time.Millisecond) {
			return false, ctx.Err()
		}
		now, err := vg.motor.Position(ctx)
		if err != nil {
			return false, err
		}

		if vg.encoderSame(now, vg.closePos) {
			// we are fully closed
			return false, vg.Stop(ctx)
		}

		pressure, _, err := vg.hasPressure(ctx)
		if err != nil {
			return false, err
		}

		if pressure {
			vg.logger.Debugf("i think i grabbed something, have pressure, pos: %f closePos: %v", now, vg.closePos)
			err := vg.motor.Go(ctx, vg.closeDirection, vg.holdingPressure)
			return true, err
		}

		total += msPer
		if total > 5000 {
			err = vg.Stop(ctx)
			if err != nil {
				return false, err
			}
			pressureRaw, err := vg.readPressure(ctx)
			if err != nil {
				return false, err
			}
			return false, fmt.Errorf("close timed out, wanted: %f at: %f pressure: %d", vg.closePos, now, pressureRaw)
		}
	}
}

func (vg *GripperV1) Close() error {
	return vg.Stop(context.Background())
}

func (vg *GripperV1) Stop(ctx context.Context) error {
	return vg.motor.Off(ctx)
}

func (vg *GripperV1) readCurrent(ctx context.Context) (int, error) {
	return vg.current.Read(ctx)
}

func (vg *GripperV1) encoderSame(a, b float64) bool {
	return math.Abs(b-a) < .1
}

func (vg *GripperV1) readPressure(ctx context.Context) (int, error) {
	return vg.pressure.Read(ctx)
}

func (vg *GripperV1) hasPressure(ctx context.Context) (bool, int, error) {
	p, err := vg.readPressure(ctx)
	return p < vg.pressureLimit, p, err
}

// return hasPressure, current
func (vg *GripperV1) analogs(ctx context.Context) (hasPressure bool, pressure, current int, err error) {
	hasPressure, pressure, err = vg.hasPressure(ctx)
	if err != nil {
		return
	}

	current, err = vg.readCurrent(ctx)
	if err != nil {
		return
	}

	return
}

func (vg *GripperV1) moveInDirectionTillWontMoveMore(ctx context.Context, dir pb.DirectionRelative) (float64, bool, error) {
	defer func() {
		err := vg.Stop(ctx)
		if err != nil {
			vg.logger.Warnf("couldn't stop motor %s", err)
		}
		vg.logger.Debugf("stopped")
	}()

	vg.logger.Debugf("starting to move dir: %v", dir)

	err := vg.motor.Go(ctx, dir, vg.defaultPowerPct)
	if err != nil {
		return -1, false, err
	}

	last, err := vg.motor.Position(ctx)
	if err != nil {
		return -1, false, err
	}

	if !utils.SelectContextOrWait(ctx, 500*time.Millisecond) {
		return -1, false, ctx.Err()
	}

	for {
		now, err := vg.motor.Position(ctx)
		if err != nil {
			return -1, false, err
		}

		hasPressure, pressure, _, err := vg.analogs(ctx)
		if err != nil {
			return -1, false, err
		}

		vg.logger.Debugf("dir: %v last: %v now: %v hasPressure: %v pressure: %v",
			dir, last, now, hasPressure, pressure)

		if vg.encoderSame(last, now) || hasPressure {
			// increase power temporarily
			err := vg.motor.Power(ctx, 128)
			if err != nil {
				return -1, false, err
			}
			if !utils.SelectContextOrWait(ctx, time.Second) {
				return -1, false, ctx.Err()
			}

			hasPressure, pressure, _, err := vg.analogs(ctx)
			if err != nil {
				return -1, false, err
			}

			vg.logger.Debugf("inner dir: %v last: %v now: %v hasPressure: %v pressure: %v",
				dir, last, now, hasPressure, pressure)

			return now, hasPressure, err
		}
		last = now

		if !utils.SelectContextOrWait(ctx, 100*time.Millisecond) {
			return -1, false, ctx.Err()
		}
	}

}
