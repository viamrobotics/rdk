// Package incremental implements an incremental encoder
package incremental

import (
	"context"
	"math"
	"sync"
	"sync/atomic"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

var incrModel = resource.NewDefaultModel("incremental")

func init() {
	registry.RegisterComponent(
		encoder.Subtype,
		incrModel,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			return NewIncrementalEncoder(ctx, deps, conf, logger)
		}})

	config.RegisterComponentAttributeMapConverter(
		encoder.Subtype,
		incrModel,
		func(attributes rutils.AttributeMap) (interface{}, error) {
			return config.TransformAttributeMapToStruct(&Config{}, attributes)
		})
}

// Encoder keeps track of a motor position using a rotary incremental encoder.
type Encoder struct {
	resource.Named

	mu       sync.Mutex
	A, B     board.DigitalInterrupt
	position int64
	pRaw     int64
	pState   int64

	positionType encoder.PositionType
	boardName    string
	encAName     string
	encBName     string

	logger golog.Logger
	// TODO(RSDK-2672): This is exposed for tests and should be unexported with
	// the constructor being used instead.
	CancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// Pins describes the configuration of Pins for a quadrature encoder.
type Pins struct {
	A string `json:"a"`
	B string `json:"b"`
}

// Config describes the configuration of a quadrature encoder.
type Config struct {
	Pins      Pins   `json:"pins"`
	BoardName string `json:"board"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string

	if conf.Pins.A == "" {
		return nil, errors.New("expected nonempty string for a")
	}
	if conf.Pins.B == "" {
		return nil, errors.New("expected nonempty string for b")
	}

	if len(conf.BoardName) == 0 {
		return nil, errors.New("expected nonempty board")
	}
	deps = append(deps, conf.BoardName)

	return deps, nil
}

// NewIncrementalEncoder creates a new Encoder.
func NewIncrementalEncoder(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (encoder.Encoder, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	e := &Encoder{
		Named:        conf.ResourceName().AsNamed(),
		logger:       logger,
		CancelCtx:    cancelCtx,
		cancelFunc:   cancelFunc,
		position:     0,
		positionType: encoder.PositionTypeTicks,
		pRaw:         0,
		pState:       0,
	}
	if err := e.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return e, nil
}

// Reconfigure atomically reconfigures this encoder in place based on the new config.
func (e *Encoder) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	e.mu.Lock()
	existingBoardName := e.boardName
	existingEncAName := e.encAName
	existingEncBName := e.encBName
	e.mu.Unlock()

	needRestart := existingBoardName != newConf.BoardName ||
		existingEncAName != newConf.Pins.A ||
		existingEncBName != newConf.Pins.B

	board, err := board.FromDependencies(deps, newConf.BoardName)
	if err != nil {
		return err
	}

	encA, ok := board.DigitalInterruptByName(newConf.Pins.A)
	if !ok {
		err := errors.Errorf("cannot find pin (%s) for incremental Encoder", newConf.Pins.A)
		return err
	}
	encB, ok := board.DigitalInterruptByName(newConf.Pins.B)
	if !ok {
		err := errors.Errorf("cannot find pin (%s) for incremental Encoder", newConf.Pins.B)
		return err
	}

	if !needRestart {
		return nil
	}
	utils.UncheckedError(e.Close())
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	e.CancelCtx = cancelCtx
	e.cancelFunc = cancelFunc

	e.mu.Lock()
	e.A = encA
	e.B = encB
	e.boardName = newConf.BoardName
	e.encAName = newConf.Pins.A
	e.encBName = newConf.Pins.B
	// state is not really valid anymore
	atomic.StoreInt64(&e.position, 0)
	atomic.StoreInt64(&e.pRaw, 0)
	atomic.StoreInt64(&e.pState, 0)
	e.mu.Unlock()

	e.Start(ctx)

	return nil
}

// Start starts the Encoder background thread.
func (e *Encoder) Start(ctx context.Context) {
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

	chanA := make(chan board.Tick)
	chanB := make(chan board.Tick)

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

			var tick board.Tick

			select {
			case <-e.CancelCtx.Done():
				return
			case tick = <-chanA:
				aLevel = 0
				if tick.High {
					aLevel = 1
				}
			case tick = <-chanB:
				bLevel = 0
				if tick.High {
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

// GetPosition returns the current position in terms of ticks or
// degrees, and whether it is a relative or absolute position.
func (e *Encoder) GetPosition(
	ctx context.Context,
	positionType encoder.PositionType,
	extra map[string]interface{},
) (float64, encoder.PositionType, error) {
	if positionType == encoder.PositionTypeDegrees {
		return math.NaN(), encoder.PositionTypeUnspecified, encoder.NewPositionTypeUnsupportedError(positionType)
	}
	res := atomic.LoadInt64(&e.position)
	return float64(res), e.positionType, nil
}

// ResetPosition sets the current position of the motor (adjusted by a given offset)
// to be its new zero position..
func (e *Encoder) ResetPosition(ctx context.Context, extra map[string]interface{}) error {
	offsetInt := int64(0)
	atomic.StoreInt64(&e.position, offsetInt)
	atomic.StoreInt64(&e.pRaw, (offsetInt<<1)|atomic.LoadInt64(&e.pRaw)&0x1)
	return nil
}

// GetProperties returns a list of all the position types that are supported by a given encoder.
func (e *Encoder) GetProperties(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error) {
	return map[encoder.Feature]bool{
		encoder.TicksCountSupported:   true,
		encoder.AngleDegreesSupported: false,
	}, nil
}

// RawPosition returns the raw position of the encoder.
func (e *Encoder) RawPosition() int64 {
	return atomic.LoadInt64(&e.pRaw)
}

func (e *Encoder) inc() {
	atomic.AddInt64(&e.pRaw, 1)
}

func (e *Encoder) dec() {
	atomic.AddInt64(&e.pRaw, -1)
}

// Close shuts down the Encoder.
func (e *Encoder) Close() error {
	e.cancelFunc()
	e.activeBackgroundWorkers.Wait()
	return nil
}
