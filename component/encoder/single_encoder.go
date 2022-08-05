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
		"single-encoder",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewSingleEncoder(ctx, deps, config, logger)
		}})
}

// DirectionAware lets you ask what direction something is moving. Only used for SingleEncoder for now, unclear future.
// DirectionMoving returns -1 if the motor is currently turning backwards, 1 if forwards and 0 if off.
type DirectionAware interface {
	DirectionMoving() int64
}

// SingleEncoder keeps track of a motor position using a rotary hall encoder.
type SingleEncoder struct {
	generic.Unimplemented
	I                board.DigitalInterrupt
	position         int64
	m                DirectionAware
	ticksPerRotation int64

	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// SinglePin defines the format the pin config should be in for SingleEncoder.
type SinglePin struct {
	I string
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
	e := &SingleEncoder{logger: logger, cancelCtx: cancelCtx, cancelFunc: cancelFunc, position: 0}
	if cfg, ok := config.ConvertedAttributes.(*Config); ok {
		if cfg.BoardName == "" {
			return nil, errors.New("SingleEncoder expected non-empty string for board")
		}
		if pins, ok := cfg.Pins.(*SinglePin); ok {
			board, err := board.FromDependencies(deps, cfg.BoardName)
			if err != nil {
				return nil, err
			}

			e.I, ok = board.DigitalInterruptByName(pins.I)
			if !ok {
				return nil, errors.Errorf("cannot find pin (%s) for SingleEncoder", pins.I)
			}
		} else {
			return nil, errors.New("Pin configuration not valid for SingleEncoder")
		}
		e.ticksPerRotation = int64(cfg.TicksPerRotation)
	}

	e.Start(ctx, func() {})

	return e, nil
}

// Start starts the SingleEncoder background thread.
// Note: unsure about whether we still need onStart.
func (e *SingleEncoder) Start(ctx context.Context, onStart func()) {
	encoderChannel := make(chan bool)
	e.I.AddCallback(encoderChannel)
	e.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		onStart()
		for {
			select {
			case <-e.cancelCtx.Done():
				return
			default:
			}

			select {
			case <-e.cancelCtx.Done():
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

// TicksPerRotation returns the number of ticks needed for a full rotation.
func (e *SingleEncoder) TicksPerRotation(ctx context.Context) (int64, error) {
	return atomic.LoadInt64(&e.ticksPerRotation), nil
}

// Close shuts down the SingleEncoder.
func (e *SingleEncoder) Close() error {
	e.logger.Debug("Closing SingleEncoder")
	e.cancelFunc()
	e.activeBackgroundWorkers.Wait()
	return nil
}
