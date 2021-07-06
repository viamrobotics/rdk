package board

import (
	"context"
	"sync/atomic"

	"github.com/go-errors/errors"

	pb "go.viam.com/core/proto/api/v1"

	"github.com/edaniels/golog"
)

// TMCConfig extends the MotorConfig for this specific series of drivers
type TMCConfig struct {
	Serial      string `json:"serial"`      // board.Serial.Name
	Address     int    `json:"address"`     // Motor address on serial bus
	Index       int    `json:"index"`       // 0th or 1st motor on driver
	MaxVelocity uint32 `json:"maxVelocity"` // Steps per second
	MaxAccel    uint32 `json:"maxAccel"`    // Steps per second per second
	MaxDecel    uint32 `json:"maxDecel"`    // Steps per second per second
}

// A TMCStepperMotor represents a brushless motor connected via a TMC controller chip (ex: TMC5072)
type TMCStepperMotor struct {
	board       SerialGPIOBoard
	serial      Serial
	addr        int
	idx         int
	enPin       string
	stepsPerRev int
	on          bool
	steps       int64
	logger      golog.Logger
	done        chan struct{}
	cancelCtx   context.Context
	cancel      func()
}

// NewTMCStepperMotor returns a TMC5072 driven motor
func NewTMCStepperMotor(b SerialGPIOBoard, mc MotorConfig, logger golog.Logger) (*TMCStepperMotor, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())

	m := &TMCStepperMotor{
		board:       b,
		serial:      b.Serial(mc.TMCConfig.Serial),
		addr:        mc.TMCConfig.Address,
		idx:         mc.TMCConfig.Index,
		stepsPerRev: mc.TicksPerRotation,
		enPin:       mc.Pins["en"],
		on:          false,
		logger:      logger,
		done:        make(chan struct{}),
		cancelCtx:   cancelCtx,
		cancel:      cancel,
	}
	logger.Debugf("Motor: %+v", m)

	return m, nil
}

// Position TODO
// TODO(pl): One nice feature of stepper motors is their ability to hold a stationary position and remain torqued.
// This should eventually be a supported feature.
func (m *TMCStepperMotor) Position(ctx context.Context) (float64, error) {
	return float64(atomic.LoadInt64(&m.steps)) / float64(m.stepsPerRev), nil
}

// PositionSupported returns true.
func (m *TMCStepperMotor) PositionSupported(ctx context.Context) (bool, error) {
	return true, nil
}

// Power TODO
// TODO(pl): Implement this feature once we have a driver board allowing PWM control.
func (m *TMCStepperMotor) Power(ctx context.Context, powerPct float32) error {
	return errors.New("power not supported for stepper motors")
}

// Go TODO
// Set a velocity as a percentage of maximum
func (m *TMCStepperMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	// m.velocityTarget(powerPct * m.MaxPower)
	return nil
}

// GoFor turns in the given direction the given number of times at the given speed. Does not block.
func (m *TMCStepperMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error {
	// target := (rotations * m.cfg.TicksPerRotation) + m.Position()
	// m.GoTo(rpm, target)
	return nil
}

// GoTo moves to the specified position in terms of rotations.
func (m *TMCStepperMotor) GoTo(ctx context.Context, rpm float64, position float64) error {

	var buffer []byte
	buffer[0] = 1
	m.serial.Lock()
	len, err := m.serial.Write(buffer)
	if len != 1 || err != nil {
		return err
	}
	m.serial.Unlock()
	// m.setPosTarget(position * m.cfg.TicksPerRotation)
	return nil
}

// IsOn returns if the motor is currently on or not.
func (m *TMCStepperMotor) IsOn(ctx context.Context) (bool, error) {
	return m.on, nil
}

// Enable pulls down the hardware enable pin, activating the power stage of the chip
func (m *TMCStepperMotor) Enable(turnOn bool) error {
	err := m.board.GPIOSet(m.enPin, turnOn)
	if err == nil {
		m.on = turnOn
	}
	return err
}

// Off turns off power to the motor and stop all movement.
func (m *TMCStepperMotor) Off(ctx context.Context) error {
	if m.on {
		return m.Enable(false)
	}
	return nil
}

// Zero resets the current position to zero.
func (m *TMCStepperMotor) Zero(ctx context.Context) error {
	//if m.readReg(voff) == 1 {
	//	return m.writeReg(XACTUAL, 0)
	//}
	return nil
}
