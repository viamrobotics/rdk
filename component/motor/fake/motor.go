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
	"go.viam.com/rdk/component/encoder"
	fakeencoder "go.viam.com/rdk/component/encoder/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
)

func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			m := &Motor{Name: config.Name, Logger: logger}
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
				m.MaxRPM = mcfg.MaxRPM

				if mcfg.Encoder != "" {
					e, err := encoder.FromDependencies(deps, mcfg.Encoder)
					if err != nil {
						return nil, err
					}
					m.Encoder = e.(*fakeencoder.Encoder)

					tpr, err := m.Encoder.TicksPerRotation(ctx)
					if err != nil {
						return 0, err
					}

					if tpr <= 0 {
						return nil, errors.Errorf("need a TicksPerRotation for encoder (%s)", mcfg.Encoder)
					}

					m.PositionReporting = true

					m.Encoder = &fakeencoder.Encoder{}
					m.Encoder.Start(ctx, &m.activeBackgroundWorkers)
				} else {
					m.PositionReporting = false
				}
			}
			return m, nil
		},
	}
	registry.RegisterComponent(motor.Subtype, "fake", _motor)

	motor.RegisterConfigAttributeConverter("fake")
}

var _ motor.LocalMotor = &Motor{}


// A Motor allows setting and reading a set power percentage and
// direction.
type Motor struct {
	Name                    string
	mu                      sync.Mutex
	powerPct                float64
	Board                   string
	PWM                     board.GPIOPin
	PositionReporting       bool
	Logger                  golog.Logger
	Encoder                 *fakeencoder.Encoder
	MaxRPM                  float64
	activeBackgroundWorkers sync.WaitGroup
	opMgr                   operation.SingleOperationManager
	generic.Echo
}

// GetPosition returns motor position in rotations.
func (m *Motor) GetPosition(ctx context.Context, extra map[string]interface{}) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Encoder == nil {
		return 0, errors.New("encoder is not defined")
	}

	ticks, err := m.Encoder.GetTicksCount(ctx, extra)
	if err != nil {
		return 0, err
	}

	tpr, err := m.Encoder.TicksPerRotation(ctx)
	if err != nil {
		return 0, err
	}
	if tpr == 0 {
		return 0, errors.New("need nonzero Tpr (ticks per rotation) for encoder")
	}

	return float64(ticks) / float64(tpr), nil
}

// GetFeatures returns the status of whether the motor supports certain optional features.
func (m *Motor) GetFeatures(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: m.PositionReporting,
	}, nil
}

// SetPower sets the given power percentage.
func (m *Motor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.opMgr.CancelRunning(ctx)
	m.Logger.Debugf("Motor SetPower %f", powerPct)
	m.setPowerPct(powerPct)

	if m.Encoder != nil {
		if m.MaxRPM == 0 {
			return errors.New("not supported, define max_rpm attribute != 0")
		}
		
		tpr, err := m.Encoder.TicksPerRotation(ctx)
		if err != nil {
			return err
		}
		if tpr == 0 {
			return errors.New("need nonzero Tpr (ticks per rotation) for encoder")
		}

		newSpeed := (m.MaxRPM * m.powerPct) * float64(tpr)
		err = m.Encoder.SetSpeed(ctx, newSpeed)
		if err != nil {
			return err
		}
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
func goForMath(maxRPM, rpm, revolutions float64) (float64, time.Duration, float64) {
	// need to do this so time is reasonable
	if rpm > maxRPM {
		rpm = maxRPM
	} else if rpm < -1*maxRPM {
		rpm = -1 * maxRPM
	}
	if rpm == 0 {
		return 0, 0, revolutions / math.Abs(revolutions)
	}

	if revolutions == 0 {
		powerPct := rpm / maxRPM
		return powerPct, 0, 1
	}

	dir := rpm * revolutions / math.Abs(revolutions*rpm)
	powerPct := math.Abs(rpm) / maxRPM * dir
	waitDur := time.Duration(math.Abs(revolutions/rpm)*60*1000) * time.Millisecond
	return powerPct, waitDur, dir
}

// GoFor sets the given direction and an arbitrary power percentage.
// If rpm is 0, the motor should immediately move to the final position.
func (m *Motor) GoFor(ctx context.Context, rpm float64, revolutions float64, extra map[string]interface{}) error {
	if m.MaxRPM == 0 {
		return errors.New("not supported, define max_rpm attribute != 0")
	}

	powerPct, waitDur, dir := goForMath(m.MaxRPM, rpm, revolutions)

	var finalPos float64
	if m.Encoder != nil {
		curPos, err := m.GetPosition(ctx, nil)
		if err != nil {
			return err
		}
		finalPos = curPos + dir*math.Abs(revolutions)
	}

	err := m.SetPower(ctx, powerPct, nil)
	if err != nil {
		return err
	}

	if revolutions == 0 {
		return nil
	}

	if m.opMgr.NewTimedWaitOp(ctx, waitDur) {
		err = m.Stop(ctx, nil)
		if err != nil {
			return err
		}

		if m.Encoder != nil {
			tpr, err := m.Encoder.TicksPerRotation(ctx)
			if err != nil {
				return err
			}

			if tpr == 0 {
				return errors.New("need nonzero Tpr (ticks per rotation) for encoder")
			}

			return m.Encoder.SetPosition(ctx, int64(finalPos*float64(tpr)))
		}
	}
	return nil
}

// GoTo sets the given direction and an arbitrary power percentage for now.
func (m *Motor) GoTo(ctx context.Context, rpm float64, pos float64, extra map[string]interface{}) error {
	if m.Encoder == nil {
		return errors.New("encoder is not defined")
	}

	if m.MaxRPM == 0 {
		return errors.New("not supported, define max_rpm attribute != 0")
	}

	curPos, err := m.GetPosition(ctx, nil)
	if err != nil {
		return err
	}
	if curPos == pos {
		return nil
	}

	revolutions := pos - curPos

	powerPct, waitDur, _ := goForMath(m.MaxRPM, math.Abs(rpm), revolutions)

	err = m.SetPower(ctx, powerPct, nil)
	if err != nil {
		return err
	}

	if revolutions == 0 {
		return nil
	}

	if m.opMgr.NewTimedWaitOp(ctx, waitDur) {
		err = m.Stop(ctx, nil)
		if err != nil {
			return err
		}
		
		tpr, err := m.Encoder.TicksPerRotation(ctx)
		if err != nil {
			return err
		}

		if tpr == 0 {
			return errors.New("need nonzero Tpr (ticks per rotation) for encoder")
		}

		return m.Encoder.SetPosition(ctx, int64(pos*float64(tpr)))
	}

	return nil
}

// GoTillStop always returns an error.
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return errors.New("not supported")
}

// ResetZeroPosition resets the zero position.
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	if m.Encoder == nil {
		return errors.New("encoder is not defined")
	}

	tpr, err := m.Encoder.TicksPerRotation(ctx)
	if err != nil {
		return err
	}

	if tpr == 0 {
		return errors.New("need nonzero Tpr (ticks per rotation) for encoder")
	}

	err = m.Encoder.ResetZeroPosition(ctx, int64(offset*float64(tpr)), extra)
	if err != nil {
		return err
	}

	return nil
}

// Stop has the motor pretend to be off.
func (m *Motor) Stop(ctx context.Context, extra map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Logger.Debug("Motor Stopped")
	m.setPowerPct(0.0)
	if m.Encoder != nil {
		err := m.Encoder.SetSpeed(ctx, 0.0)
		if err != nil {
			return err
		}
	}
	return nil
}

// IsPowered returns if the motor is pretending to be on or not.
func (m *Motor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, error) {
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
