// Package fake implements a fake motor.
package fake

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	fakeencoder "go.viam.com/rdk/components/encoder/fake"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

var model = resource.NewDefaultModel("fake")

const defaultMaxRpm = 100

// PinConfig defines the mapping of where motor are wired.
type PinConfig struct {
	Direction string `json:"dir"`
	PWM       string `json:"pwm"`
}

// Config describes the configuration of a motor.
type Config struct {
	Pins             PinConfig `json:"pins"`
	BoardName        string    `json:"board"`
	MinPowerPct      float64   `json:"min_power_pct,omitempty"`
	MaxPowerPct      float64   `json:"max_power_pct,omitempty"`
	PWMFreq          uint      `json:"pwm_freq,omitempty"`
	Encoder          string    `json:"encoder,omitempty"`
	MaxRPM           float64   `json:"max_rpm,omitempty"`
	TicksPerRotation int       `json:"ticks_per_rotation,omitempty"`
	DirectionFlip    bool      `json:"direction_flip"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.BoardName != "" {
		deps = append(deps, cfg.BoardName)
	}
	return deps, nil
}

func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			m := &Motor{Name: config.Name, Logger: logger}
			if mcfg, ok := config.ConvertedAttributes.(*Config); ok {
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
						if err = m.PWM.SetPWMFreq(ctx, mcfg.PWMFreq, nil); err != nil {
							return nil, err
						}
					}
				}
				m.MaxRPM = mcfg.MaxRPM

				if m.MaxRPM == 0 {
					logger.Infof("Max RPM not provided to a fake motor, defaulting to %v", defaultMaxRpm)
					m.MaxRPM = defaultMaxRpm
				}

				if mcfg.Encoder != "" {
					if mcfg.TicksPerRotation <= 0 {
						return nil, errors.New("need nonzero TicksPerRotation for encoded motor")
					}
					m.TicksPerRotation = mcfg.TicksPerRotation

					e, err := encoder.FromDependencies(deps, mcfg.Encoder)
					if err != nil {
						return nil, err
					}
					realEncoder := rdkutils.UnwrapProxy(e)
					m.Encoder = realEncoder.(*fakeencoder.Encoder)
					m.PositionReporting = true
				} else {
					m.PositionReporting = false
				}
				m.DirFlip = false
				if mcfg.DirectionFlip {
					m.DirFlip = true
				}
			}
			return m, nil
		},
	}
	registry.RegisterComponent(motor.Subtype, model, _motor)
	config.RegisterComponentAttributeMapConverter(
		motor.Subtype,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&Config{},
	)
}

var _ motor.LocalMotor = &Motor{}

// A Motor allows setting and reading a set power percentage and
// direction.
type Motor struct {
	Name              string
	mu                sync.Mutex
	powerPct          float64
	Board             string
	PWM               board.GPIOPin
	PositionReporting bool
	Logger            golog.Logger
	Encoder           *fakeencoder.Encoder
	MaxRPM            float64
	DirFlip           bool
	opMgr             operation.SingleOperationManager
	TicksPerRotation  int
	generic.Echo
}

// Position returns motor position in rotations.
func (m *Motor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Encoder == nil {
		return 0, errors.New("encoder is not defined")
	}

	ticks, err := m.Encoder.TicksCount(ctx, extra)
	if err != nil {
		return 0, err
	}

	if m.TicksPerRotation == 0 {
		return 0, errors.New("need nonzero TicksPerRotation for motor")
	}

	return ticks / float64(m.TicksPerRotation), nil
}

// Properties returns the status of whether the motor supports certain optional features.
func (m *Motor) Properties(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
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
		if m.TicksPerRotation <= 0 {
			return errors.New("need positive nonzero TicksPerRotation")
		}

		newSpeed := (m.MaxRPM * m.powerPct) * float64(m.TicksPerRotation)
		err := m.Encoder.SetSpeed(ctx, newSpeed)
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
	if m.DirFlip {
		m.powerPct *= -1
	}
	return m.powerPct
}

// Direction returns the set direction.
func (m *Motor) Direction() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case m.powerPct > 0:
		return 1
	case m.powerPct < 0:
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
func (m *Motor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	if rpm == 0 {
		return motor.NewZeroRPMError()
	}

	powerPct, waitDur, dir := goForMath(m.MaxRPM, rpm, revolutions)

	var finalPos float64
	if m.Encoder != nil {
		curPos, err := m.Position(ctx, nil)
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
			return m.Encoder.SetPosition(ctx, int64(finalPos*float64(m.TicksPerRotation)))
		}
	}
	return nil
}

// GoTo sets the given direction and an arbitrary power percentage for now.
func (m *Motor) GoTo(ctx context.Context, rpm, pos float64, extra map[string]interface{}) error {
	if m.Encoder == nil {
		return errors.New("encoder is not defined")
	}

	curPos, err := m.Position(ctx, nil)
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

		return m.Encoder.SetPosition(ctx, int64(pos*float64(m.TicksPerRotation)))
	}

	return nil
}

// GoTillStop always returns an error.
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return motor.NewGoTillStopUnsupportedError(m.Name)
}

// ResetZeroPosition resets the zero position.
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	if m.Encoder == nil {
		return errors.New("encoder is not defined")
	}

	if m.TicksPerRotation == 0 {
		return errors.New("need nonzero TicksPerRotation for motor")
	}

	err := m.Encoder.Reset(ctx, offset*float64(m.TicksPerRotation), extra)
	if err != nil {
		return errors.Wrapf(err, "error in ResetZeroPosition from motor (%s)", m.Name)
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
			return errors.Wrapf(err, "error in Stop from motor (%s)", m.Name)
		}
	}
	return nil
}

// IsPowered returns if the motor is pretending to be on or not, and its power level.
func (m *Motor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return math.Abs(m.powerPct) >= 0.005, m.powerPct, nil
}

// IsMoving returns if the motor is pretending to be moving or not.
func (m *Motor) IsMoving(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return math.Abs(m.powerPct) >= 0.005, nil
}
