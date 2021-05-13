// Package softrobotics implements the vacuum gripper from Soft Robotics.
package softrobotics

import (
	"context"
	"errors"
	"time"

	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/config"
	"go.viam.com/robotcore/gripper"
	"go.viam.com/robotcore/registry"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

func init() {
	registry.RegisterGripper("softrobotics", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gripper.Gripper, error) {
		b := r.BoardByName("local")
		if b == nil {
			return nil, errors.New("softrobotics gripper requires a board called local")
		}
		g, ok := b.(board.GPIOBoard)
		if !ok {
			return nil, errors.New("softrobotics gripper requires a baord that is a GPIOBoard")
		}
		return NewGripper(ctx, b, g, config, logger)
	})
}

/*
   open is 5
   close is 6
*/
type Gripper struct {
	theBoard  board.Board
	gpioBoard board.GPIOBoard

	psi board.AnalogReader

	pinOpen, pinClose, pinPower string

	logger golog.Logger
}

func NewGripper(ctx context.Context, b board.Board, g board.GPIOBoard, config config.Component, logger golog.Logger) (*Gripper, error) {
	theGripper := &Gripper{
		theBoard:  b,
		gpioBoard: g,
		psi:       b.AnalogReader("psi"),
		pinOpen:   config.Attributes.String("open"),
		pinClose:  config.Attributes.String("close"),
		pinPower:  config.Attributes.String("power"),
		logger:    logger,
	}

	if theGripper.psi == nil {
		return nil, errors.New("no psi analog reader")
	}

	if theGripper.pinOpen == "" || theGripper.pinClose == "" || theGripper.pinPower == "" {
		return nil, errors.New("need pins for open, close, power")
	}

	return theGripper, nil
}

func (g *Gripper) Stop() error {
	return multierr.Combine(
		g.gpioBoard.GPIOSet(g.pinOpen, false),
		g.gpioBoard.GPIOSet(g.pinClose, false),
		g.gpioBoard.GPIOSet(g.pinPower, false),
	)
}

func (g *Gripper) Open(ctx context.Context) error {
	err := multierr.Combine(
		g.gpioBoard.GPIOSet(g.pinOpen, true),
		g.gpioBoard.GPIOSet(g.pinPower, true),
	)
	if err != nil {
		return err
	}

	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		} // REMOVE

		val, err := g.psi.Read(ctx)
		if err != nil {
			return multierr.Combine(err, g.Stop())
		}

		if val > 500 {
			break
		}

		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		}
	}

	return g.Stop()
}

func (g *Gripper) Grab(ctx context.Context) (bool, error) {
	err := multierr.Combine(
		g.gpioBoard.GPIOSet(g.pinClose, true),
		g.gpioBoard.GPIOSet(g.pinPower, true),
	)
	if err != nil {
		return false, err
	}

	for {

		if !utils.SelectContextOrWait(ctx, 100*time.Millisecond) {
			return false, ctx.Err()
		} // REMOVE

		val, err := g.psi.Read(ctx)
		if err != nil {
			return false, multierr.Combine(err, g.Stop())
		}

		if val <= 200 {
			break
		}

		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return false, ctx.Err()
		}
	}

	return false, g.Stop()

}
