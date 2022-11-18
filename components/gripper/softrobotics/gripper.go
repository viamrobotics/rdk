// Package softrobotics implements the vacuum gripper from Soft Robotics.
package softrobotics

import (
	"context"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
)

const modelname = "softrobotics"

// AttrConfig is the config for a trossen gripper.
type AttrConfig struct {
	Board        string `json:"board"`
	Open         string `json:"open"`
	Close        string `json:"close"`
	Power        string `json:"power"`
	AnalogReader string `json:"analog_reader"`
}

// Validate ensures all parts of the config are valid.
func (cfg *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.Board == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}
	if cfg.Open == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "open")
	}
	if cfg.Close == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "close")
	}
	if cfg.Power == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "power")
	}

	if cfg.AnalogReader != "psi" {
		return nil, utils.NewConfigValidationError(path,
			errors.Errorf("analog_reader %s on board must be created and called 'psi'", cfg.AnalogReader))
	}
	deps = append(deps, cfg.Board)
	return deps, nil
}

func init() {
	registry.RegisterComponent(gripper.Subtype, modelname, registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			b, err := board.FromDependencies(deps, "local")
			if err != nil {
				return nil, err
			}
			return newGripper(b, config, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(gripper.SubtypeName, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
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
func newGripper(b board.Board, cfg config.Component, logger golog.Logger) (gripper.LocalGripper, error) {
	attr, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attr, cfg.ConvertedAttributes)
	}

	psi, ok := b.AnalogReaderByName("psi")
	if !ok {
		return nil, errors.New("failed to find analog reader 'psi'")
	}
	pinOpen, err := b.GPIOPinByName(attr.Open)
	if err != nil {
		return nil, err
	}
	pinClose, err := b.GPIOPinByName(attr.Close)
	if err != nil {
		return nil, err
	}
	pinPower, err := b.GPIOPinByName(attr.Power)
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
func (g *softGripper) Stop(ctx context.Context, extra map[string]interface{}) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()
	return multierr.Combine(
		g.pinOpen.Set(ctx, false, nil),
		g.pinClose.Set(ctx, false, nil),
		g.pinPower.Set(ctx, false, nil),
	)
}

// Open TODO.
func (g *softGripper) Open(ctx context.Context, extra map[string]interface{}) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()

	err := multierr.Combine(
		g.pinOpen.Set(ctx, true, nil),
		g.pinPower.Set(ctx, true, nil),
	)
	if err != nil {
		return err
	}

	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		} // REMOVE

		val, err := g.psi.Read(ctx, nil)
		if err != nil {
			return multierr.Combine(err, g.Stop(ctx, extra))
		}

		if val > 500 {
			break
		}

		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		}
	}

	return g.Stop(ctx, extra)
}

// Grab TODO.
func (g *softGripper) Grab(ctx context.Context, extra map[string]interface{}) (bool, error) {
	ctx, done := g.opMgr.New(ctx)
	defer done()

	err := multierr.Combine(
		g.pinClose.Set(ctx, true, nil),
		g.pinPower.Set(ctx, true, nil),
	)
	if err != nil {
		return false, err
	}

	for {
		if !utils.SelectContextOrWait(ctx, 100*time.Millisecond) {
			return false, ctx.Err()
		} // REMOVE

		val, err := g.psi.Read(ctx, nil)
		if err != nil {
			return false, multierr.Combine(err, g.Stop(ctx, extra))
		}

		if val <= 200 {
			break
		}

		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return false, ctx.Err()
		}
	}

	return false, g.Stop(ctx, extra)
}

// IsMoving returns whether the gripper is moving.
func (g *softGripper) IsMoving(ctx context.Context) (bool, error) {
	return g.opMgr.OpRunning(), nil
}

// ModelFrame is unimplemented for softGripper.
func (g *softGripper) ModelFrame() referenceframe.Model {
	return nil
}
