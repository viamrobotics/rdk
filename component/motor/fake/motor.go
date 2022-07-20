// Package fake implements a fake motor.
package fake

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
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
				if m.cfg.TicksPerRotation <= 0 {
					m.cfg.TicksPerRotation = 1
					m.logger.Info("ticks_per_rotation must be positive, using default value 1")
				}
				if m.cfg.MaxRPM == 0 {
					m.cfg.MaxRPM = 60
					m.logger.Info("using default value 60 for max_rpm")
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
	mu         sync.Mutex
	position   float64
	speed      float64 // ticks per minute
	logger     golog.Logger
	updateRate int64 // update position in start every updateRate ms
}

// Position returns the current position in terms of ticks.
func (e *fakeEncoder) GetPosition(ctx context.Context) (float64, error) {
	return e.position, nil
}

// Start starts a background thread to run the encoder.
func (e *fakeEncoder) Start(cancelCtx context.Context, activeBackgroundWorkers *sync.WaitGroup) {
	activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		if e.updateRate == 0 {
			e.updateRate = 100
		}
		for {
			select {
			case <-cancelCtx.Done():
				if e.logger != nil {
					e.logger.Debug("Canceled")
				}
				return
			default:
			}

			if !utils.SelectContextOrWait(cancelCtx, time.Duration(e.updateRate)*time.Millisecond) {
				if e.logger != nil {
					e.logger.Debug("Context Done")
				}
				return
			}

			e.mu.Lock()
			e.position += e.speed / float64(60*1000/e.updateRate)
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

// SetSpeed sets the speed of the fake motor the encoder is measuring.
func (e *fakeEncoder) SetSpeed(ctx context.Context, speed float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.speed = speed
	return nil
}

// SetPosition sets the position of the encoder.
func (e *fakeEncoder) SetPosition(ctx context.Context, position float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.position = position
	return nil
}

// A Motor allows setting and reading a set power percentage and
// direction.
type Motor struct {
	Name                    string
	mu                      sync.Mutex
	powerPct                float64
	Board                   string
	PWM                     board.GPIOPin
	positionReporting       bool
	logger                  golog.Logger
	encoder                 fakeEncoder
	cfg                     motor.Config
	activeBackgroundWorkers sync.WaitGroup
	opMgr                   operation.SingleOperationManager
	generic.Echo
}

// NewFakeMotor returns a usable instance of Motor.
func NewFakeMotor() *Motor {
	mcfg := motor.Config{TicksPerRotation: 1, MaxRPM: 60}
	m := &Motor{encoder: fakeEncoder{}, positionReporting: true, cfg: mcfg}
	return m
}

// GetPosition returns motor position in rotations.
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

	m.opMgr.CancelRunning(ctx)
	if m.logger != nil {
		m.logger.Debugf("Motor SetPower %f", powerPct)
	}
	m.setPowerPct(powerPct)
	newSpeed := (m.cfg.MaxRPM * m.powerPct) * float64(m.cfg.TicksPerRotation)
	err := m.encoder.SetSpeed(ctx, newSpeed)
	if err != nil {
		return err
	}
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

// If revolutions is 0, the returned wait duration will be 0 representing that
// the motor should run indefinitely.
func goForMath(maxRPM, rpm, revolutions float64) (float64, time.Duration) {
	// need to do this so time is reasonable
	if rpm > maxRPM {
		rpm = maxRPM
	} else if rpm < -1*maxRPM {
		rpm = -1 * maxRPM
	}

	if revolutions == 0 {
		powerPct := rpm / maxRPM
		return powerPct, 0
	}

	dir := rpm * revolutions / math.Abs(revolutions*rpm)
	powerPct := math.Abs(rpm) / maxRPM * dir
	waitDur := time.Duration(math.Abs(revolutions/rpm)*60*1000) * time.Millisecond
	return powerPct, waitDur
}

// GoFor sets the given direction and an arbitrary power percentage.
func (m *Motor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	powerPct, waitDur := goForMath(m.cfg.MaxRPM, rpm, revolutions)
	err := m.SetPower(ctx, powerPct)
	if err != nil {
		return err
	}

	if revolutions == 0 {
		return nil
	}

	if m.opMgr.NewTimedWaitOp(ctx, waitDur) {
		return m.Stop(ctx)
	}
	return nil
}

// GoTo sets the given direction and an arbitrary power percentage for now.
func (m *Motor) GoTo(ctx context.Context, rpm float64, pos float64) error {
	curPos, err := m.encoder.GetPosition(ctx)
	curPos /= float64(m.cfg.TicksPerRotation)
	if err != nil {
		return err
	}
	revolutions := pos - curPos

	err = m.GoFor(ctx, rpm, revolutions)
	return err
}

// GoTillStop always returns an error.
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return errors.New("not supported")
}

// ResetZeroPosition resets the zero position.
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64) error {
	err := m.encoder.ResetZeroPosition(ctx, offset*float64(m.cfg.TicksPerRotation))
	if err != nil {
		return err
	}
	return nil
}

// Stop has the motor pretend to be off.
func (m *Motor) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.opMgr.CancelRunning(ctx)
	if m.logger != nil {
		m.logger.Debug("Motor Stopped")
	}
	m.setPowerPct(0.0)
	err := m.encoder.SetSpeed(ctx, 0.0)
	if err != nil {
		return err
	}
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
