// Package softrobotics implements the vacuum gripper from Soft Robotics.
package softrobotics

import (
	"context"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterComponent(gripper.Subtype, "softrobotics", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			b, ok := r.BoardByName("local")
			if !ok {
				return nil, errors.New("softrobotics gripper requires a board called local")
			}
			return newGripper(b, config, logger)
		},
	})
}

// softGripper TODO
//
// open is 5
// close is 6.
type softGripper struct {
	theBoard board.Board

	psi board.AnalogReader

	pinOpen, pinClose, pinPower string

	logger golog.Logger
}

// newGripper TODO.
func newGripper(b board.Board, config config.Component, logger golog.Logger) (*softGripper, error) {
	psi, ok := b.AnalogReaderByName("psi")
	if !ok {
		return nil, errors.New("failed to find analog reader 'psi'")
	}
	theGripper := &softGripper{
		theBoard: b,
		psi:      psi,
		pinOpen:  config.Attributes.String("open"),
		pinClose: config.Attributes.String("close"),
		pinPower: config.Attributes.String("power"),
		logger:   logger,
	}

	if theGripper.psi == nil {
		return nil, errors.New("no psi analog reader")
	}

	if theGripper.pinOpen == "" || theGripper.pinClose == "" || theGripper.pinPower == "" {
		return nil, errors.New("need pins for open, close, power")
	}

	return theGripper, nil
}

// Stop TODO.
func (g *softGripper) Stop(ctx context.Context) error {
	return multierr.Combine(
		g.theBoard.GPIOSet(ctx, g.pinOpen, false),
		g.theBoard.GPIOSet(ctx, g.pinClose, false),
		g.theBoard.GPIOSet(ctx, g.pinPower, false),
	)
}

// Open TODO.
func (g *softGripper) Open(ctx context.Context) error {
	err := multierr.Combine(
		g.theBoard.GPIOSet(ctx, g.pinOpen, true),
		g.theBoard.GPIOSet(ctx, g.pinPower, true),
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
			return multierr.Combine(err, g.Stop(ctx))
		}

		if val > 500 {
			break
		}

		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		}
	}

	return g.Stop(ctx)
}

// Grab TODO.
func (g *softGripper) Grab(ctx context.Context) (bool, error) {
	err := multierr.Combine(
		g.theBoard.GPIOSet(ctx, g.pinClose, true),
		g.theBoard.GPIOSet(ctx, g.pinPower, true),
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
			return false, multierr.Combine(err, g.Stop(ctx))
		}

		if val <= 200 {
			break
		}

		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return false, ctx.Err()
		}
	}

	return false, g.Stop(ctx)
}
