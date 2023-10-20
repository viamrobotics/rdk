// Package softrobotics implements the vacuum gripper from Soft Robotics.
package softrobotics

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var model = resource.DefaultModelFamily.WithModel("softrobotics")

// Config is the config for a trossen gripper.
type Config struct {
	Board        string `json:"board"`
	Open         string `json:"open"`
	Close        string `json:"close"`
	Power        string `json:"power"`
	AnalogReader string `json:"analog_reader"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
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
	resource.RegisterComponent(gripper.API, model, resource.Registration[gripper.Gripper, *Config]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.ZapCompatibleLogger,
		) (gripper.Gripper, error) {
			b, err := board.FromDependencies(deps, "local")
			if err != nil {
				return nil, err
			}
			return newGripper(b, conf, logging.FromZapCompatible(logger))
		},
	})
}

// softGripper TODO
//
// open is 5
// close is 6.
type softGripper struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	theBoard board.Board

	psi board.AnalogReader

	pinOpen, pinClose, pinPower board.GPIOPin

	logger     logging.Logger
	opMgr      *operation.SingleOperationManager
	geometries []spatialmath.Geometry
}

// newGripper instantiates a new Gripper of softGripper type.
func newGripper(b board.Board, conf resource.Config, logger logging.Logger) (gripper.Gripper, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	psi, ok := b.AnalogReaderByName("psi")
	if !ok {
		return nil, errors.New("failed to find analog reader 'psi'")
	}
	pinOpen, err := b.GPIOPinByName(newConf.Open)
	if err != nil {
		return nil, err
	}
	pinClose, err := b.GPIOPinByName(newConf.Close)
	if err != nil {
		return nil, err
	}
	pinPower, err := b.GPIOPinByName(newConf.Power)
	if err != nil {
		return nil, err
	}

	theGripper := &softGripper{
		Named:    conf.ResourceName().AsNamed(),
		theBoard: b,
		psi:      psi,
		pinOpen:  pinOpen,
		pinClose: pinClose,
		pinPower: pinPower,
		logger:   logger,
		opMgr:    operation.NewSingleOperationManager(),
	}

	if theGripper.psi == nil {
		return nil, errors.New("no psi analog reader")
	}

	if conf.Frame != nil && conf.Frame.Geometry != nil {
		geometry, err := conf.Frame.Geometry.ParseConfig()
		if err != nil {
			return nil, err
		}
		theGripper.geometries = []spatialmath.Geometry{geometry}
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

// Geometries returns the geometries associated with the softGripper.
func (g *softGripper) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return g.geometries, nil
}
