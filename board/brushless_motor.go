package board

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

var (
	minute     = time.Minute
	defaultRPM = 60.0
)

// NewBrushlessMotor returns a brushless motor on board with the given configuration. When done using it,
// please call Close.
func NewBrushlessMotor(b GPIOBoard, pins map[string]string, mc MotorConfig, logger golog.Logger) (*BrushlessMotor, error) {

	cancelCtx, cancel := context.WithCancel(context.Background())

	// Technically you can have the two jumpers set to keep ENA and ENB always-on on a dual H-bridge.
	// Note that this may cause unwanted heat buildup on the H-bridge, so we require PWM control.
	// Two PWM pins are used, each control the on/off state of one side of a dual H-bridge.
	// We use both sides for a stepper motor, so they should either both be on or both off.
	// Since we step manually, these should not be actually used for PWM, with motor speed instead
	// being controlled via the timing of the ABCD pins. Otherwise we risk partial steps and getting the
	// motor coils into a bad state.
	m := &BrushlessMotor{
		cfg:       mc,
		Board:     b,
		A:         pins["a"],
		B:         pins["b"],
		C:         pins["c"],
		D:         pins["d"],
		PWMs:      []string{pins["pwm"]},
		on:        false,
		commands:  make(chan brushlessMotorCmd),
		logger:    logger,
		done:      make(chan struct{}),
		cancelCtx: cancelCtx,
		cancel:    cancel,
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

// brushlessMotorCmd is for passing messages to the motor manager.
type brushlessMotorCmd struct {
	d     pb.DirectionRelative
	wait  time.Duration
	steps int
	cont  bool
}

// A BrushlessMotor represents a brushless motor connected to a board via GPIO.
type BrushlessMotor struct {
	cfg                     MotorConfig
	Board                   GPIOBoard
	PWMs                    []string
	A, B, C, D              string
	steps                   int64
	on                      bool
	startedMgr              bool
	commands                chan brushlessMotorCmd
	logger                  golog.Logger
	done                    chan struct{}
	cancelCtx               context.Context
	cancel                  func()
	activeBackgroundWorkers sync.WaitGroup
}

// TODO(pl): One nice feature of stepper motors is their ability to hold a stationary position and remain torqued.
// This should eventually be a supported feature.
func (m *BrushlessMotor) Position(ctx context.Context) (float64, error) {
	return float64(atomic.LoadInt64(&m.steps)) / float64(m.cfg.TicksPerRotation), nil
}

func (m *BrushlessMotor) PositionSupported(ctx context.Context) (bool, error) {
	return true, nil
}

// TODO(pl): Implement this feature once we have a driver board allowing PWM control.
func (m *BrushlessMotor) Power(ctx context.Context, powerPct float32) error {
	return errors.New("power not supported for stepper motors on dual H-bridges")
}

func (m *BrushlessMotor) setStep(pins []bool) error {
	return multierr.Combine(
		m.Board.GPIOSet(m.A, pins[0]),
		m.Board.GPIOSet(m.B, pins[1]),
		m.Board.GPIOSet(m.C, pins[2]),
		m.Board.GPIOSet(m.D, pins[3]),
	)
}

// This will power on the motor if necessary, and make one full step sequence (4 steps) in the specified direction.
func (m *BrushlessMotor) step(ctx context.Context, d pb.DirectionRelative, wait time.Duration) error {
	seq := stepSequence()
	switch d {
	case pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED:
		return nil
	case pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD:
		for i := 0; i < len(seq); i++ {
			if err := m.setStep(seq[i]); err != nil {
				return err
			}
			atomic.AddInt64(&m.steps, 1)
			// Waiting between each setStep() call is the best way to adjust motor speed.
			// See the comment above in NewBrushlessMotor() for why to not use PWM.
			if !utils.SelectContextOrWait(ctx, wait) {
				return ctx.Err()
			}
		}
	case pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD:
		for i := len(seq) - 1; i >= 0; i-- {
			if err := m.setStep(seq[i]); err != nil {
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

// Use this to launch a goroutine that will rotate in a direction while listening on a channel for Off().
func (m *BrushlessMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	m.motorManagerStart()

	wait := time.Duration(float64(minute.Microseconds())/(float64(m.cfg.TicksPerRotation)*defaultRPM)) * time.Microsecond

	m.commands <- brushlessMotorCmd{d: d, wait: wait, cont: true}

	return nil
}

// Turn in the given direction the given number of times at the given speed. Does not block.
func (m *BrushlessMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error {
	// Set our wait time off of the specified RPM
	m.motorManagerStart()

	wait := time.Duration(float64(minute.Microseconds())/(float64(m.cfg.TicksPerRotation)*rpm)) * time.Microsecond
	steps := int(math.Abs(rotations * float64(m.cfg.TicksPerRotation)))

	m.commands <- brushlessMotorCmd{d: d, wait: wait, steps: steps, cont: false}

	return nil
}

func (m *BrushlessMotor) IsOn(ctx context.Context) (bool, error) {
	return m.on, nil
}

func (m *BrushlessMotor) turnOnOrOff(turnOn bool) error {
	var err error

	if turnOn != m.on {
		for _, pwmPin := range m.PWMs {
			err = multierr.Combine(
				err,
				m.Board.GPIOSet(pwmPin, turnOn),
			)
		}
		if err == nil {
			m.on = turnOn
		}
	}

	return err
}

// Turn off power to the motor and stop all movement.
func (m *BrushlessMotor) Off(ctx context.Context) error {
	if m.on {
		return m.turnOnOrOff(false)
	}
	return nil
}

func (m *BrushlessMotor) motorManager(ctx context.Context) {
	var err error
	if m.startedMgr {
		return
	}
	m.startedMgr = true

	motorCmd := brushlessMotorCmd{}

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
			if err = m.turnOnOrOff(true); err != nil {
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

func (m *BrushlessMotor) motorManagerStart() {
	if m.startedMgr {
		return
	}
	m.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		m.motorManager(m.cancelCtx)
	}, m.activeBackgroundWorkers.Done)
}

func (m *BrushlessMotor) Close() error {
	m.cancel()
	close(m.done)
	m.activeBackgroundWorkers.Wait()
	return m.turnOnOrOff(false)
}
