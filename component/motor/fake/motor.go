// Package fake implements a fake motor.
package fake

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/utils"
)

func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			m := &Motor{Name: config.Name, logger: logger, encoder: fakeEncoder{logger: logger}}
			if mcfg, ok := config.ConvertedAttributes.(*motor.Config); ok {
				if mcfg.BoardName != "" {
					m.Board = mcfg.BoardName
					b, err := board.FromDependencies(deps, m.Board)
					if err != nil {
						return nil, err
					}
					if mcfg.Pins.PWM != "" {
						m.PWM, err = b.GPIOPinByName(mcfg.Pins.PWM)
						if err != nil {
							return nil, err
						}
						if err = m.PWM.SetPWMFreq(ctx, mcfg.PWMFreq); err != nil {
							return nil, err
						}
					}
				}
				
				m.cfg = *mcfg
				if m.cfg.TicksPerRotation == 0 {
					m.cfg.TicksPerRotation = 1
				}
				if m.cfg.MaxRPM == 0 {
					m.cfg.MaxRPM = 60
				}

				if mcfg.EncoderA != "" || mcfg.EncoderB != "" {
					m.positionReporting = true
					
					m.encoder.Start(ctx, &m.activeBackgroundWorkers)
				}
			}
			return m, nil
		},
	}
	registry.RegisterComponent(motor.Subtype, "fake", _motor)

	motor.RegisterConfigAttributeConverter("fake")
}

var _ motor.LocalMotor = &Motor{}

type fakeEncoder struct {
	mu       			sync.Mutex
	position 			float64
	speed				float64		// ticks per minute
	logger 				golog.Logger
}

// Position returns the current position in terms of ticks.
func (e *fakeEncoder) GetPosition(ctx context.Context) (float64, error) {
	return e.position, nil
}

// Start starts a background thread to run the encoder
func (e *fakeEncoder) Start(cancelCtx context.Context, activeBackgroundWorkers *sync.WaitGroup) {
	activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			select {
			case <-cancelCtx.Done():
				e.logger.Debug("Canceled")
				return
			default:
			}

			if !utils.SelectContextOrWait(cancelCtx, 100*time.Millisecond) {
				e.logger.Debug("Context Done")
				return
			}

			e.mu.Lock()
			e.position += e.speed / 600.
			e.mu.Unlock()
		}
	}, activeBackgroundWorkers.Done)
}

// ResetZeroPosition resets the zero position.
func (e *fakeEncoder) ResetZeroPosition(ctx context.Context, offset float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.position = offset
	return nil
}

// SetSpeed sets the speed of the fake motor the encoder is measuring
func (e *fakeEncoder) SetSpeed(ctx context.Context, speed float64) (error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.speed = speed
	return nil
}

// SetPosition sets the position of the encoder
func (e *fakeEncoder) SetPosition(ctx context.Context, position float64) (error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.position = position
	return nil
}

// A Motor allows setting and reading a set power percentage and
// direction.
type Motor struct {
	Name     			string
	mu       			sync.Mutex
	powerPct 			float64
	position 			float64
	Board    			string
	PWM      			board.GPIOPin
	positionReporting 	bool
	logger				golog.Logger
	encoder             fakeEncoder
	cfg					motor.Config
	activeBackgroundWorkers sync.WaitGroup
	generic.Echo
}

// GetPosition returns motor position in rotations
func (m *Motor) GetPosition(ctx context.Context) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ticks, err := m.encoder.GetPosition(ctx)
	if err != nil {
		return 0, err
	}
	return ticks / float64(m.cfg.TicksPerRotation), nil
}

// GetFeatures returns the status of whether the motor supports certain optional features.
func (m *Motor) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: m.positionReporting,
	}, nil
}

// SetPower sets the given power percentage.
func (m *Motor) SetPower(ctx context.Context, powerPct float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Debugf("Motor SetPower %f", powerPct)
	m.setPowerPct(powerPct)
	newSpeed := (m.cfg.MaxRPM * m.powerPct) * float64(m.cfg.TicksPerRotation)
	_ = m.encoder.SetSpeed(ctx, newSpeed)
	return nil
}

func (m *Motor) setPowerPct(powerPct float64) {
	m.powerPct = powerPct
}

// PowerPct returns the set power percentage.
func (m *Motor) PowerPct() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.powerPct
}

// Direction returns the set direction.
func (m *Motor) Direction() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.powerPct > 0 {
		return 1
	}
	if m.powerPct < 0 {
		return -1
	}
	return 0
}

// GoFor sets the given direction and an arbitrary power percentage.
func (m *Motor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rpm < 0 {
		revolutions *= -1
	}
	if rpm == 0 {
		revolutions = 0
	}

	pos, _ := m.encoder.GetPosition(ctx)
	m.encoder.SetPosition(ctx, pos + revolutions * float64(m.cfg.TicksPerRotation))
	m.encoder.SetSpeed(ctx, 0)
	m.setPowerPct(0.0)

	return nil
}

// GoTo sets the given direction and an arbitrary power percentage for now.
func (m *Motor) GoTo(ctx context.Context, rpm float64, pos float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.encoder.SetPosition(ctx, pos * float64(m.cfg.TicksPerRotation))
	m.encoder.SetSpeed(ctx, 0)
	m.setPowerPct(0.0)
	return nil
}

// GoTillStop always returns an error.
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return errors.New("not supported")
}

// ResetZeroPosition resets the zero position.
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64) error {
	m.encoder.ResetZeroPosition(ctx, offset * float64(m.cfg.TicksPerRotation))
	return nil
}

// Stop has the motor pretend to be off.
func (m *Motor) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Debug("Motor Stopped")
	m.setPowerPct(0.0)
	_ = m.encoder.SetSpeed(ctx, 0.0)
	return nil
}

// IsPowered returns if the motor is pretending to be on or not.
func (m *Motor) IsPowered(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return math.Abs(m.powerPct) >= 0.005, nil
}

// IsMoving returns if the motor is pretending to be moving or not.
func (m *Motor) IsMoving(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return math.Abs(m.powerPct) >= 0.005, nil
}
