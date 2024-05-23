// Package fake implements a fake encoder.
package fake

import (
	"context"
	"math"
	"sync"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

var fakeModel = resource.DefaultModelFamily.WithModel("fake")

func init() {
	resource.RegisterComponent(encoder.API, fakeModel, resource.Registration[encoder.Encoder, *Config]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (encoder.Encoder, error) {
			return NewEncoder(ctx, conf, logger)
		},
	})
}

// NewEncoder creates a new Encoder.
func NewEncoder(
	ctx context.Context,
	cfg resource.Config,
	logger logging.Logger,
) (encoder.Encoder, error) {
	e := &fakeEncoder{
		Named:        cfg.ResourceName().AsNamed(),
		position:     0,
		positionType: encoder.PositionTypeTicks,
		logger:       logger,
	}
	if err := e.Reconfigure(ctx, nil, cfg); err != nil {
		return nil, err
	}
	e.workers = rdkutils.NewStoppableWorkers(e.start)

	return e, nil
}

func (e *fakeEncoder) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}
	e.mu.Lock()

	e.updateRate = newConf.UpdateRate
	if e.updateRate == 0 {
		e.updateRate = 100
	}
	e.mu.Unlock()
	return nil
}

// Config describes the configuration of a fake encoder.
type Config struct {
	UpdateRate  int64   `json:"update_rate_msec,omitempty"`
}

// Validate ensures all parts of a config is valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	return nil, nil
}

// fakeEncoder keeps track of a fake motor position.
type fakeEncoder struct {
	resource.Named

	positionType encoder.PositionType
	logger       logging.Logger

	mu          sync.RWMutex
	workers     rdkutils.StoppableWorkers
	position    int64
	updateRate  int64   // update position in start every updateRate ms
}

// Position returns the current position in terms of ticks or
// degrees, and whether it is a relative or absolute position.
func (e *fakeEncoder) Position(
	ctx context.Context,
	positionType encoder.PositionType,
	extra map[string]interface{},
) (float64, encoder.PositionType, error) {
	if positionType == encoder.PositionTypeDegrees {
		return math.NaN(), encoder.PositionTypeUnspecified, encoder.NewPositionTypeUnsupportedError(positionType)
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return float64(e.position), e.positionType, nil
}

// Start starts a background thread to run the encoder.
func (e *fakeEncoder) start(cancelCtx context.Context) {
	for {
		select {
		case <-cancelCtx.Done():
			return
		default:
		}

		e.mu.RLock()
		updateRate := e.updateRate
		e.mu.RUnlock()
		if !utils.SelectContextOrWait(cancelCtx, time.Duration(updateRate)*time.Millisecond) {
			return
		}

		e.mu.Lock()
		e.position ++
		e.mu.Unlock()
	}
}

// ResetPosition sets the current position of the motor (adjusted by a given offset)
// to be its new zero position.
func (e *fakeEncoder) ResetPosition(ctx context.Context, extra map[string]interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.position = 0
	return nil
}

// Properties returns a list of all the position types that are supported by a given encoder.
func (e *fakeEncoder) Properties(ctx context.Context, extra map[string]interface{}) (encoder.Properties, error) {
	return encoder.Properties{
		TicksCountSupported:   true,
		AngleDegreesSupported: false,
	}, nil
}

// Close safely shuts down the fake encoder.
func (e *fakeEncoder) Close(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.workers.Stop()
	return nil
}

// Encoder is a fake encoder used for testing.
type Encoder interface {
	encoder.Encoder
	SetPosition(ctx context.Context, position int64) error
}

// SetPosition sets the position of the encoder.
func (e *fakeEncoder) SetPosition(ctx context.Context, position int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.position = position
	return nil
}
