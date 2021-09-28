package board

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

var (
	minute     = time.Minute
	defaultRPM = 60.0
)

// NewGPIOStepperMotor returns a brushless motor on board with the given configuration. When done using it,
// please call Close.
func NewGPIOStepperMotor(b Board, pins map[string]string, mc motor.Config, logger golog.Logger) (*GPIOStepperMotor, error) {

	cancelCtx, cancel := context.WithCancel(context.Background())

	// Technically you can have the two jumpers set to keep ENA and ENB always-on on a dual H-bridge.
	// Note that this may cause unwanted heat buildup on the H-bridge, so we require PWM control.
	// Two PWM pins are used, each control the on/off state of one side of a dual H-bridge.
	// We use both sides for a stepper motor, so they should either both be on or both off.
	// Since we step manually, these should not be actually used for PWM, with motor speed instead
	// being controlled via the timing of the ABCD pins. Otherwise we risk partial steps and getting the
	// motor coils into a bad state.
	m := &GPIOStepperMotor{
		cfg:                     mc,
		Board:                   b,
		A:                       pins["a"],
		B:                       pins["b"],
		C:                       pins["c"],
		D:                       pins["d"],
		PWMs:                    []string{pins["pwm"]},
		on:                      false,
		commands:                make(chan gpioStepperMotorCmd),
		logger:                  logger,
		done:                    make(chan struct{}),
		cancelCtx:               cancelCtx,
		cancel:                  cancel,
		activeBackgroundWorkers: &sync.WaitGroup{},
	}
	if _, ok := pins["pwmb"]; ok {
		// The two PWM inputs can be controlled by one pin whose output is forked, above, or two individual pins.
		// Benefit of two individual pins is that the H-bridge can be plugged directly into a Pi without
		// the use of a breadboard.
		m.PWMs = append(m.PWMs, pins["pwmb"])
	}

	return m, nil
}

// 4-wire stepper motors have various coils that must be activated in the correct sequence.
// https://www.raspberrypi.org/forums/viewtopic.php?t=55580
func stepSequence() [][]bool {
	return [][]bool{
		{true, false, true, false},
		{false, true, true, false},
		{false, true, false, true},
		{true, false, false, true},
	}
}

// gpioStepperMotorCmd is for passing messages to the motor manager.
type gpioStepperMotorCmd struct {
	d     pb.DirectionRelative
	wait  time.Duration
	steps int
	cont  bool
}

// A GPIOStepperMotor represents a brushless motor connected to a board via GPIO.
type GPIOStepperMotor struct {
	cfg                     motor.Config
	Board                   Board
	PWMs                    []string
	A, B, C, D              string
	steps                   int64
	on                      bool
	startedMgr              bool
	commands                chan gpioStepperMotorCmd
	logger                  golog.Logger
	done                    chan struct{}
	cancelCtx               context.Context
	cancel                  func()
	activeBackgroundWorkers *sync.WaitGroup
}

// Position TODO
// TODO(pl): One nice feature of stepper motors is their ability to hold a stationary position and remain torqued.
// This should eventually be a supported feature.
func (m *GPIOStepperMotor) Position(ctx context.Context) (float64, error) {
	return float64(atomic.LoadInt64(&m.steps)) / float64(m.cfg.TicksPerRotation), nil
}

// PositionSupported returns true.
func (m *GPIOStepperMotor) PositionSupported(ctx context.Context) (bool, error) {
	return true, nil
}

// Power TODO
// TODO(pl): Implement this feature once we have a driver board allowing PWM control.
func (m *GPIOStepperMotor) Power(ctx context.Context, powerPct float32) error {
	return errors.New("power not supported for stepper motors on dual H-bridges")
}

func (m *GPIOStepperMotor) setStep(ctx context.Context, pins []bool) error {
	return multierr.Combine(
		m.Board.GPIOSet(ctx, m.A, pins[0]),
		m.Board.GPIOSet(ctx, m.B, pins[1]),
		m.Board.GPIOSet(ctx, m.C, pins[2]),
		m.Board.GPIOSet(ctx, m.D, pins[3]),
	)
}

// This will power on the motor if necessary, and make one full step sequence (4 steps) in the specified direction.
func (m *GPIOStepperMotor) step(ctx context.Context, d pb.DirectionRelative, wait time.Duration) error {
	seq := stepSequence()
	switch d {
	case pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED:
		return nil
	case pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD:
		for i := 0; i < len(seq); i++ {
			if err := m.setStep(ctx, seq[i]); err != nil {
				return err
			}
			atomic.AddInt64(&m.steps, 1)
			// Waiting between each setStep() call is the best way to adjust motor speed.
			// See the comment above in NewGPIOStepperMotor() for why to not use PWM.
			if !utils.SelectContextOrWait(ctx, wait) {
				return ctx.Err()
			}
		}
	case pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD:
		for i := len(seq) - 1; i >= 0; i-- {
			if err := m.setStep(ctx, seq[i]); err != nil {
				return err
			}
			atomic.AddInt64(&m.steps, -1)
			if !utils.SelectContextOrWait(ctx, wait) {
				return ctx.Err()
			}
		}
	}
	return nil
}

// Go TODO
// Use this to launch a goroutine that will rotate in a direction while listening on a channel for Off().
func (m *GPIOStepperMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	m.motorManagerStart()

	wait := time.Duration(float64(minute.Microseconds())/(float64(m.cfg.TicksPerRotation)*defaultRPM)) * time.Microsecond

	m.commands <- gpioStepperMotorCmd{d: d, wait: wait, cont: true}

	return nil
}

// GoFor turn in the given direction the given number of times at the given speed. Does not block.
func (m *GPIOStepperMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error {
	// Set our wait time off of the specified RPM
	m.motorManagerStart()

	wait := time.Duration(float64(minute.Microseconds())/(float64(m.cfg.TicksPerRotation)*rpm)) * time.Microsecond
	steps := int(math.Abs(rotations * float64(m.cfg.TicksPerRotation)))

	m.commands <- gpioStepperMotorCmd{d: d, wait: wait, steps: steps, cont: false}

	return nil
}

// IsOn returns if the motor is currently on or not.
func (m *GPIOStepperMotor) IsOn(ctx context.Context) (bool, error) {
	return m.on, nil
}

func (m *GPIOStepperMotor) turnOnOrOff(ctx context.Context, turnOn bool) error {
	var err error

	if turnOn != m.on {
		for _, pwmPin := range m.PWMs {
			err = multierr.Combine(
				err,
				m.Board.GPIOSet(ctx, pwmPin, turnOn),
			)
		}
		if err == nil {
			m.on = turnOn
		}
	}

	return err
}

// Off turns off power to the motor and stop all movement.
func (m *GPIOStepperMotor) Off(ctx context.Context) error {
	if m.on {
		return m.turnOnOrOff(ctx, false)
	}
	return nil
}

func (m *GPIOStepperMotor) motorManager(ctx context.Context) {
	var err error
	if m.startedMgr {
		return
	}
	m.startedMgr = true

	motorCmd := gpioStepperMotorCmd{}

	nextCommand := func(block bool) bool {
		if block {
			select {
			case <-m.done:
				m.startedMgr = false
				return false
			case motorCmd = <-m.commands:
			}
			return true
		}
		select {
		case <-m.done:
			m.startedMgr = false
			return false
		case motorCmd = <-m.commands:
		default:
		}
		return true
	}

	for {
		// block if our non-cont command is complete
		if cont := nextCommand(!motorCmd.cont && motorCmd.steps <= 0); !cont {
			return
		}

		// Perform one set of steps, then check again for new commands.
		if motorCmd.d == pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED || (!motorCmd.cont && motorCmd.steps <= 0) {
			if err = m.Off(ctx); err != nil {
				m.logger.Warnf("error turning off: %s", err)
			}
		} else {
			if err = m.turnOnOrOff(ctx, true); err != nil {
				m.logger.Warnf("error turning on: %s", err)
				// If we couldn't turn on for some reason, we'll wait a moment then try the whole thing over again
				if !utils.SelectContextOrWait(ctx, 500*time.Millisecond) {
					return
				}
				continue
			}
			if err = m.step(ctx, motorCmd.d, motorCmd.wait); err != nil {
				m.logger.Warnf("error performing step: %s", err)
			} else {
				// TODO(pl): remember what step we're on so we can do one at a time instead of 4
				motorCmd.steps -= 4
			}
		}
	}
}

func (m *GPIOStepperMotor) motorManagerStart() {
	if m.startedMgr {
		return
	}
	m.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		m.motorManager(m.cancelCtx)
	}, m.activeBackgroundWorkers.Done)
}

// Close cleanly stops the motor and waits for background goroutines to finish.
func (m *GPIOStepperMotor) Close() error {
	m.cancel()
	close(m.done)
	m.activeBackgroundWorkers.Wait()
	return m.turnOnOrOff(context.Background(), false)
}

// GoTo is not supported
func (m *GPIOStepperMotor) GoTo(ctx context.Context, rpm float64, position float64) error {
	return errors.New("not supported")
}

// GoTillStop is not supported
func (m *GPIOStepperMotor) GoTillStop(ctx context.Context, d pb.DirectionRelative, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return errors.New("not supported")
}

// Zero is not supported
func (m *GPIOStepperMotor) Zero(ctx context.Context, offset float64) error {
	return errors.New("not supported")
}
