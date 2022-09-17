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
)

func init() {
	registry.RegisterComponent(
		Subtype,
		"hall",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewHallEncoder(ctx, deps, config, logger)
		}})

	config.RegisterComponentAttributeMapConverter(
		SubtypeName,
		"hall",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf HallConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&HallConfig{})
}

// HallEncoder keeps track of a motor position using a rotary hall encoder.
type HallEncoder struct {
	A, B     board.DigitalInterrupt
	position int64
	pRaw     int64
	pState   int64

	logger                  golog.Logger
	CancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup

	generic.Unimplemented
}

// HallPins describes the configuration of Pins for a Hall encoder.
type HallPins struct {
	A string `json:"a"`
	B string `json:"b"`
}

// HallConfig describes the configuration of a Hall encoder.
type HallConfig struct {
	Pins      HallPins `json:"pins"`
	BoardName string   `json:"board"`
}

// Validate ensures all parts of the config are valid.
func (config *HallConfig) Validate(path string) ([]string, error) {
	var deps []string

	if config.Pins.A == "" {
		return nil, errors.New("expected nonempty string for a")
	}
	if config.Pins.B == "" {
		return nil, errors.New("expected nonempty string for b")
	}

	if len(config.BoardName) == 0 {
		return nil, errors.New("expected nonempty board")
	}
	deps = append(deps, config.BoardName)

	return deps, nil
}

// NewHallEncoder creates a new HallEncoder.
func NewHallEncoder(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (*HallEncoder, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	e := &HallEncoder{logger: logger, CancelCtx: cancelCtx, cancelFunc: cancelFunc, position: 0, pRaw: 0, pState: 0}
	if cfg, ok := config.ConvertedAttributes.(*HallConfig); ok {
		board, err := board.FromDependencies(deps, cfg.BoardName)
		if err != nil {
			return nil, err
		}

		e.A, ok = board.DigitalInterruptByName(cfg.Pins.A)
		if !ok {
			return nil, errors.Errorf("cannot find pin (%s) for HallEncoder", cfg.Pins.A)
		}
		e.B, ok = board.DigitalInterruptByName(cfg.Pins.B)
		if !ok {
			return nil, errors.Errorf("cannot find pin (%s) for HallEncoder", cfg.Pins.B)
		}

		e.Start(ctx)

		return e, nil
	}

	return nil, errors.New("encoder config for HallEncoder is not valid")
}

// Start starts the HallEncoder background thread.
func (e *HallEncoder) Start(ctx context.Context) {
	/**
	  a rotary encoder looks like

	  picture from https://github.com/joan2937/pigpio/blob/master/EXAMPLES/C/ROTARY_ENCODER/rotary_encoder.c
	    1   2     3    4    1    2    3    4     1

	            +---------+         +---------+      0
	            |         |         |         |
	  A         |         |         |         |
	            |         |         |         |
	  +---------+         +---------+         +----- 1

	      +---------+         +---------+            0
	      |         |         |         |
	  B   |         |         |         |
	      |         |         |         |
	  ----+         +---------+         +---------+  1

	*/

	// State Transition Table
	//     +---------------+----+----+----+----+
	//     | pState/nState | 00 | 01 | 10 | 11 |
	//     +---------------+----+----+----+----+
	//     |       00      | 0  | -1 | +1 | x  |
	//     +---------------+----+----+----+----+
	//     |       01      | +1 | 0  | x  | -1 |
	//     +---------------+----+----+----+----+
	//     |       10      | -1 | x  | 0  | +1 |
	//     +---------------+----+----+----+----+
	//     |       11      | x  | +1 | -1 | 0  |
	//     +---------------+----+----+----+----+
	// 0 -> same state
	// x -> impossible state

	chanA := make(chan bool)
	chanB := make(chan bool)

	e.A.AddCallback(chanA)
	e.B.AddCallback(chanB)

	aLevel, err := e.A.Value(ctx, nil)
	if err != nil {
		utils.Logger.Errorw("error reading a level", "error", err)
	}
	bLevel, err := e.B.Value(ctx, nil)
	if err != nil {
		utils.Logger.Errorw("error reading b level", "error", err)
	}
	e.pState = aLevel | (bLevel << 1)

	e.activeBackgroundWorkers.Add(1)

	utils.ManagedGo(func() {
		defer e.A.RemoveCallback(chanA)
		defer e.B.RemoveCallback(chanB)
		for {
			select {
			case <-e.CancelCtx.Done():
				return
			default:
			}

			var level bool

			select {
			case <-e.CancelCtx.Done():
				return
			case level = <-chanA:
				aLevel = 0
				if level {
					aLevel = 1
				}
			case level = <-chanB:
				bLevel = 0
				if level {
					bLevel = 1
				}
			}
			nState := aLevel | (bLevel << 1)
			if e.pState == nState {
				continue
			}
			switch (e.pState << 2) | nState {
			case 0b0001:
				fallthrough
			case 0b0111:
				fallthrough
			case 0b1000:
				fallthrough
			case 0b1110:
				e.dec()
				atomic.StoreInt64(&e.position, atomic.LoadInt64(&e.pRaw)>>1)
				e.pState = nState
			case 0b0010:
				fallthrough
			case 0b0100:
				fallthrough
			case 0b1011:
				fallthrough
			case 0b1101:
				e.inc()
				atomic.StoreInt64(&e.position, atomic.LoadInt64(&e.pRaw)>>1)
				e.pState = nState
			}
		}
	}, e.activeBackgroundWorkers.Done)
}

// GetTicksCount returns number of ticks since last zeroing.
func (e *HallEncoder) GetTicksCount(ctx context.Context, extra map[string]interface{}) (int64, error) {
	return atomic.LoadInt64(&e.position), nil
}

// Reset sets the current position of the motor (adjusted by a given offset)
// to be its new zero position..
func (e *HallEncoder) Reset(ctx context.Context, offset int64, extra map[string]interface{}) error {
	atomic.StoreInt64(&e.position, offset)
	atomic.StoreInt64(&e.pRaw, (offset<<1)|atomic.LoadInt64(&e.pRaw)&0x1)
	return nil
}

// RawPosition returns the raw position of the encoder.
func (e *HallEncoder) RawPosition() int64 {
	return atomic.LoadInt64(&e.pRaw)
}

func (e *HallEncoder) inc() {
	atomic.AddInt64(&e.pRaw, 1)
}

func (e *HallEncoder) dec() {
	atomic.AddInt64(&e.pRaw, -1)
}

// Close shuts down the HallEncoder.
func (e *HallEncoder) Close() error {
	e.logger.Debug("Closing HallEncoder")
	e.cancelFunc()
	e.activeBackgroundWorkers.Wait()
	return nil
}
