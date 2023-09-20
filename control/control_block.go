package control

import (
	"context"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/utils"
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

// BlockConfig configuration of a given block.
type BlockConfig struct {
	Name      string             `json:"name"`       // Control Block name
	Type      controlBlockType   `json:"type"`       // Control Block type
	Attribute utils.AttributeMap `json:"attributes"` // Internal block configuration
	DependsOn []string           `json:"depends_on"` // List of blocks needed for calling Next
}

/* Full List of Attributes

*/


// Block interface for a control block.
type Block interface {
	// Reset will reset the control block to initial state. Returns an error on failure
	Reset(ctx context.Context) error

	// Next calculate the next output. Takes an array of float64 , a delta time returns True and the output value on success false otherwise
	Next(ctx context.Context, x []*Signal, dt time.Duration) ([]*Signal, bool)

	// UpdateConfig update the configuration of a pre-existing control block returns an error on failure
	UpdateConfig(ctx context.Context, config BlockConfig) error

	// Output returns the most recent valid value, useful for block aggregating signals
	Output(ctx context.Context) []*Signal

	// Config returns the underlying config for a Block
	Config(ctx context.Context) BlockConfig
}

func createBlock(cfg BlockConfig, logger golog.Logger) (Block, error) {
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
