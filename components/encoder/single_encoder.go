package encoder

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var singlemodelname = resource.NewDefaultModel("single")

func init() {
	registry.RegisterComponent(
		Subtype,
		singlemodelname,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewSingleEncoder(ctx, deps, config, logger)
		}})

	config.RegisterComponentAttributeMapConverter(
		Subtype,
		singlemodelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf SingleWireConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&SingleWireConfig{})
}

// DirectionAware lets you ask what direction something is moving. Only used for SingleEncoder for now, unclear future.
// DirectionMoving returns -1 if the motor is currently turning backwards, 1 if forwards and 0 if off.
type DirectionAware interface {
	DirectionMoving() int64
}

// SingleEncoder keeps track of a motor position using a rotary encoder.
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

// SingleWirePin describes the configuration of Pins for a Single encoder.
type SingleWirePin struct {
	I string `json:"i"`
}

// SingleWireConfig describes the configuration of a single encoder.
type SingleWireConfig struct {
	Pins      SingleWirePin `json:"pins"`
	BoardName string        `json:"board"`
}

// Validate ensures all parts of the config are valid.
func (cfg *SingleWireConfig) Validate(path string) ([]string, error) {
	var deps []string

	if cfg.Pins.I == "" {
		return nil, errors.New("expected nonempty string for i")
	}

	if len(cfg.BoardName) == 0 {
		return nil, errors.New("expected nonempty board")
	}
	deps = append(deps, cfg.BoardName)

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
	cfg config.Component,
	logger golog.Logger,
) (*SingleEncoder, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	e := &SingleEncoder{logger: logger, CancelCtx: cancelCtx, cancelFunc: cancelFunc, position: 0}
	if cfg, ok := cfg.ConvertedAttributes.(*SingleWireConfig); ok {
		board, err := board.FromDependencies(deps, cfg.BoardName)
		if err != nil {
			return nil, err
		}

		e.I, ok = board.DigitalInterruptByName(cfg.Pins.I)
		if !ok {
			return nil, errors.Errorf("cannot find pin (%s) for SingleEncoder", cfg.Pins.I)
		}

		logger.Info("no direction attached to SingleEncoder yet. SingleEncoder will not take measurements until attached to encoded motor.")

		e.Start(ctx)

		return e, nil
	}

	return nil, errors.New("encoder config for SingleEncoder is not valid")
}

// Start starts the SingleEncoder background thread.
func (e *SingleEncoder) Start(ctx context.Context) {
	encoderChannel := make(chan bool)
	e.I.AddCallback(encoderChannel)
	e.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		defer e.I.RemoveCallback(encoderChannel)
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

			if e.m != nil {
				dir := e.m.DirectionMoving()
				if dir == 1 || dir == -1 {
					atomic.AddInt64(&e.position, dir)
				}
			}
		}
	}, e.activeBackgroundWorkers.Done)
}

// TicksCount returns the current position.
func (e *SingleEncoder) TicksCount(ctx context.Context, extra map[string]interface{}) (float64, error) {
	res := atomic.LoadInt64(&e.position)
	return float64(res), nil
}

// Reset sets the current position of the motor (adjusted by a given offset)
// to be its new zero position.
func (e *SingleEncoder) Reset(ctx context.Context, offset float64, extra map[string]interface{}) error {
	offsetInt := int64(offset)
	atomic.StoreInt64(&e.position, offsetInt)
	return nil
}

// Close shuts down the SingleEncoder.
func (e *SingleEncoder) Close() error {
	e.logger.Debug("Closing SingleEncoder")
	e.cancelFunc()
	e.activeBackgroundWorkers.Wait()
	return nil
}
