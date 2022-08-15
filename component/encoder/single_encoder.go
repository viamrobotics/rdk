package encoder

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterComponent(
		Subtype,
		"single",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewSingleEncoder(ctx, deps, config, logger)
		}})

	config.RegisterComponentAttributeMapConverter(
		SubtypeName,
		"single",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf SingleConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&SingleConfig{})
}

// DirectionAware lets you ask what direction something is moving. Only used for SingleEncoder for now, unclear future.
// DirectionMoving returns -1 if the motor is currently turning backwards, 1 if forwards and 0 if off.
type DirectionAware interface {
	DirectionMoving() int64
}

// SingleEncoder keeps track of a motor position using a rotary hall encoder.
type SingleEncoder struct {
	generic.Unimplemented
	I        board.DigitalInterrupt
	position int64
	m        DirectionAware

	logger                  golog.Logger
	CancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// SinglePins describes the configuration of Pins for a Single encoder.
type SinglePins struct {
	I   string `json:"i"`
	Dir string `json:"dir"`
}

// SingleConfig describes the configuration of a single encoder.
type SingleConfig struct {
	Pins      SinglePins `json:"pins"`
	BoardName string     `json:"board"`
}

// Validate ensures all parts of the config are valid.
func (config *SingleConfig) Validate(path string) ([]string, error) {
	var deps []string

	if config.Pins.I == "" {
		return nil, errors.New("expected nonempty string for i")
	}
	if config.Pins.Dir == "" {
		return nil, errors.New("expected nonempty string for dir")
	}

	if len(config.BoardName) == 0 {
		return nil, errors.New("expected nonempty board")
	}
	deps = append(deps, config.BoardName)

	return deps, nil
}

// AttachDirectionalAwareness to pre-created encoder.
func (e *SingleEncoder) AttachDirectionalAwareness(da DirectionAware) {
	e.m = da
}

// NewSingleEncoder creates a new SingleEncoder.
func NewSingleEncoder(
	ctx context.Context,
	deps registry.Dependencies,
	config config.Component,
	logger golog.Logger,
) (*SingleEncoder, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	e := &SingleEncoder{logger: logger, CancelCtx: cancelCtx, cancelFunc: cancelFunc, position: 0}
	if cfg, ok := config.ConvertedAttributes.(*SingleConfig); ok {
		if cfg.BoardName == "" {
			return nil, errors.New("SingleEncoder expected non-empty string for board")
		}

		pin := cfg.Pins.I
		if pin == "" {
			return nil, errors.New("HallEncoder pin configuration expects non-empty string for a")
		}

		if cfg.Pins.Dir == "" {
			return nil, errors.New("single line encoder needs motor direction pin")
		}

		board, err := board.FromDependencies(deps, cfg.BoardName)
		if err != nil {
			return nil, err
		}

		e.I, ok = board.DigitalInterruptByName(pin)
		if !ok {
			return nil, errors.Errorf("cannot find pin (%s) for SingleEncoder", pin)
		}
	}

	return e, nil
}

// Start starts the SingleEncoder background thread.
func (e *SingleEncoder) Start(ctx context.Context, onStart func()) {
	encoderChannel := make(chan bool)
	e.I.AddCallback(encoderChannel)
	e.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		onStart()
		for {
			select {
			case <-e.CancelCtx.Done():
				return
			default:
			}

			select {
			case <-e.CancelCtx.Done():
				return
			case <-encoderChannel:
			}

			dir := e.m.DirectionMoving()
			if dir == 1 || dir == -1 {
				atomic.AddInt64(&e.position, dir)
			}
		}
	}, e.activeBackgroundWorkers.Done)
}

// GetTicksCount returns the current position.
func (e *SingleEncoder) GetTicksCount(ctx context.Context, extra map[string]interface{}) (int64, error) {
	return atomic.LoadInt64(&e.position), nil
}

// ResetToZero sets the current position of the motor (adjusted by a given offset)
// to be its new zero position.
func (e *SingleEncoder) ResetToZero(ctx context.Context, offset int64, extra map[string]interface{}) error {
	atomic.StoreInt64(&e.position, offset)
	return nil
}

// Close shuts down the SingleEncoder.
func (e *SingleEncoder) Close() error {
	e.logger.Debug("Closing SingleEncoder")
	e.cancelFunc()
	e.activeBackgroundWorkers.Wait()
	return nil
}
