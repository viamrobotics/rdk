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
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterComponent(gripper.Subtype, "softrobotics", registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			b, err := board.FromDependencies(deps, "local")
			if err != nil {
				return nil, err
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

	pinOpen, pinClose, pinPower board.GPIOPin

	logger golog.Logger
	opMgr  operation.SingleOperationManager
	generic.Unimplemented
}

// newGripper TODO.
func newGripper(b board.Board, config config.Component, logger golog.Logger) (gripper.Gripper, error) {
	psi, ok := b.AnalogReaderByName("psi")
	if !ok {
		return nil, errors.New("failed to find analog reader 'psi'")
	}
	pinOpen, err := b.GPIOPinByName(config.Attributes.String("open"))
	if err != nil {
		return nil, err
	}
	pinClose, err := b.GPIOPinByName(config.Attributes.String("close"))
	if err != nil {
		return nil, err
	}
	pinPower, err := b.GPIOPinByName(config.Attributes.String("power"))
	if err != nil {
		return nil, err
	}

	theGripper := &softGripper{
		theBoard: b,
		psi:      psi,
		pinOpen:  pinOpen,
		pinClose: pinClose,
		pinPower: pinPower,
		logger:   logger,
	}

	if theGripper.psi == nil {
		return nil, errors.New("no psi analog reader")
	}

	return theGripper, nil
}

// Stop TODO.
func (g *softGripper) Stop(ctx context.Context) error {
	return multierr.Combine(
		g.pinOpen.Set(ctx, false),
		g.pinClose.Set(ctx, false),
		g.pinPower.Set(ctx, false),
	)
}

// Open TODO.
func (g *softGripper) Open(ctx context.Context) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()

	err := multierr.Combine(
		g.pinOpen.Set(ctx, true),
		g.pinPower.Set(ctx, true),
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
	ctx, done := g.opMgr.New(ctx)
	defer done()

	err := multierr.Combine(
		g.pinClose.Set(ctx, true),
		g.pinPower.Set(ctx, true),
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

// ModelFrame is unimplemented for softGripper.
func (g *softGripper) ModelFrame() referenceframe.Model {
	return nil
}
