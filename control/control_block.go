package control

import (
	"context"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
)

type controlBlockType string

const (
	blockEndpoint                   controlBlockType = "endpoint"
	blockFilter                     controlBlockType = "filter"
	blockTrapezoidalVelocityProfile controlBlockType = "trapezoidalVelocityProfile"
	blockPID                        controlBlockType = "PID"
	blockGain                       controlBlockType = "gain"
	blockDerivative                 controlBlockType = "derivative"
	blockSum                        controlBlockType = "sum"
	blockConstant                   controlBlockType = "constant"
	blockEncoderToRPM               controlBlockType = "encoderToRpm"
)

// ControlBlockConfig configuration of a given block
// nolint: revive
type ControlBlockConfig struct {
	Name      string              `json:"name"`       // Control Block name
	Type      controlBlockType    `json:"type"`       // Control Block type
	Attribute config.AttributeMap `json:"attributes"` // Internal block configuration
	// TODO: include ImplicitDependsOn?
	DependsOn []string            `json:"depends_on"` // List of blocks needed for calling Next
}

// ControlBlock interface for a control block
// nolint: revive
type ControlBlock interface {
	// Reset will reset the control block to initial state. Returns an error on failure
	Reset(ctx context.Context) error

	// Next calculate the next output. Takes an array of float64 , a delta time returns True and the output value on success false otherwise
	Next(ctx context.Context, x []Signal, dt time.Duration) ([]Signal, bool)

	// UpdateConfig update the configuration of a pre-existing control block returns an error on failure
	UpdateConfig(ctx context.Context, config ControlBlockConfig) error

	// Output returns the most recent valid value, useful for block aggregating signals
	Output(ctx context.Context) []Signal

	// Config returns the underlying config for a ControlBlock
	Config(ctx context.Context) ControlBlockConfig
}

func createControlBlock(cfg ControlBlockConfig, logger golog.Logger) (ControlBlock, error) {
	t := cfg.Type
	switch t {
	case blockEndpoint:
		b, err := newEndpoint(cfg, logger, nil)
		if err != nil {
			return nil, err
		}
		return b, nil
	case blockSum:
		b, err := newSum(cfg, logger)
		if err != nil {
			return nil, err
		}
		return b, nil
	case blockDerivative:
		b, err := newDerivative(cfg, logger)
		if err != nil {
			return nil, err
		}
		return b, nil
	case blockTrapezoidalVelocityProfile:
		b, err := newTrapezoidVelocityProfile(cfg, logger)
		if err != nil {
			return nil, err
		}
		return b, nil
	case blockGain:
		b, err := newGain(cfg, logger)
		if err != nil {
			return nil, err
		}
		return b, nil
	case blockPID:
		b, err := newPID(cfg, logger)
		if err != nil {
			return nil, err
		}
		return b, nil
	case blockFilter:
		b, err := newFilter(cfg, logger)
		if err != nil {
			return nil, err
		}
		return b, nil
	case blockConstant:
		b, err := newConstant(cfg, logger)
		if err != nil {
			return nil, err
		}
		return b, nil
	case blockEncoderToRPM:
		b, err := newEncoderSpeed(cfg, logger)
		if err != nil {
			return nil, err
		}
		return b, nil
	}
	return nil, errors.Errorf("unsupported block type %s", t)
}
