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
	SPIBus      string `json:"spiBus"`      // SPI Bus name
	ChipSelect  uint   `json:"chipSelect"`  // Motor address on serial bus
	Index       uint   `json:"index"`       // 0th or 1st motor on driver
	MaxVelocity uint32 `json:"maxVelocity"` // Steps per second
	MaxAccel    uint32 `json:"maxAccel"`    // Steps per second per second
}

// A TMCStepperMotor represents a brushless motor connected via a TMC controller chip (ex: TMC5072)
type TMCStepperMotor struct {
	board       SPIGPIOBoard
	bus         SPI
	chip        uint
	index       uint
	enPin       string
	stepsPerRev int
	maxVel      uint32
	maxAcc      uint32
	on          bool
	steps       int64
	logger      golog.Logger
	cancelCtx   context.Context
	cancel      func()
}

// NewTMCStepperMotor returns a TMC5072 driven motor
func NewTMCStepperMotor(b SPIGPIOBoard, mc MotorConfig, logger golog.Logger) (*TMCStepperMotor, error) {
	bus := b.SPI(mc.TMCConfig.SPIBus)
	if bus == nil {
		return nil, errors.Errorf("TMCStepperMotor can't find SPI bus named %s", mc.TMCConfig.SPIBus)
	}

	cancelCtx, cancel := context.WithCancel(context.Background())

	m := &TMCStepperMotor{
		board:       b,
		bus:         bus,
		chip:        mc.TMCConfig.ChipSelect,
		index:       mc.TMCConfig.Index,
		stepsPerRev: mc.TicksPerRotation,
		enPin:       mc.Pins["en"],
		maxVel:      mc.TMCConfig.MaxVelocity,
		maxAcc:      mc.TMCConfig.MaxAccel,
		on:          false,
		logger:      logger,
		cancelCtx:   cancelCtx,
		cancel:      cancel,
	}
	logger.Debugf("Motor: %+v", m)

	m.initRegisters()

	return m, nil
}

// TMC5072 Register Addressses (for motor index 0)
// TODO full register set
const (
	// add 0x10 for motor 2
	CHOPCONF = 0x6C

	// add 0x20 for motor 2
	RAMPMODE   = 0x20
	VSTART     = 0x23
	A1         = 0x24
	V1         = 0x25
	AMAX       = 0x26
	VMAX       = 0x27
	DMAX       = 0x28
	D1         = 0x2A
	VSTOP      = 0x2B
	IHOLD_IRUN = 0x30
)

// TMC5072 ramp modes
const (
	MODE_POSITION = int32(0)
	MODE_VELPOS   = int32(1)
	MODE_VELNEG   = int32(2)
	MODE_HOLD     = int32(3)
)

func (m *TMCStepperMotor) shiftAddr(addr uint8) uint8 {
	//Shift register address for motor 1 instead of motor zero
	if m.index == 1 {
		if addr >= 0x10 && addr <= 0x11 {
			addr += 0x08
		} else if addr >= 0x20 && addr <= 0x3C {
			addr += 0x20
		} else if addr >= 0x6A && addr <= 0x6F {
			addr += 0x10
		}
	}
	return addr
}

func (m *TMCStepperMotor) WriteReg(addr uint8, value int32) error {

	addr = m.shiftAddr(addr)

	buf := make([]byte, 5)
	buf[0] = addr | 0x80
	buf[1] = 0xFF & byte(value>>24)
	buf[2] = 0xFF & byte(value>>16)
	buf[3] = 0xFF & byte(value>>8)
	buf[4] = 0xFF & byte(value)

	m.bus.Lock()
	defer m.bus.Unlock()

	m.logger.Debug("Write: ", buf)

	// SPI Mode 3, 1mhz
	_, err := m.bus.Xfer(1000000, m.chip, 3, buf)
	if err != nil {
		return err
	}
	return nil
}

func (m *TMCStepperMotor) ReadReg(ctx context.Context, addr uint8) (int32, error) {

	addr = m.shiftAddr(addr)

	tbuf := make([]byte, 5)

	tbuf[0] = addr & 0x7F // Make sure write bit is cleared

	m.bus.Lock()
	defer m.bus.Unlock()

	m.logger.Debug("ReadT: ", tbuf)

	// SPI Mode 3, 1mhz
	rbuf, err := m.bus.Xfer(1000000, m.chip, 3, tbuf)
	if err != nil {
		return 0, err
	}

	var value int32
	value = int32(rbuf[1])
	value <<= 8
	value |= int32(rbuf[2])
	value <<= 8
	value |= int32(rbuf[3])
	value <<= 8
	value |= int32(rbuf[4])

	m.logger.Debug("ReadR: ", rbuf)
	m.logger.Debug("Read: ", value)

	return value, nil

}

func (m *TMCStepperMotor) initRegisters() error {

	// TOFF=3, HSTRT=4, HEND=1, TBL=2, CHM=0 (spreadCycle)
	m.WriteReg(CHOPCONF, 0x000100C3)

	// IHOLD=8 (half current), IRUN=15 (max current), IHOLDDELAY=6
	m.WriteReg(IHOLD_IRUN, 0x00080F0A)

	// Max accelerations
	m.WriteReg(A1, int32(float32(m.maxAcc)*1.5))
	m.WriteReg(AMAX, int32(m.maxAcc))
	m.WriteReg(D1, int32(float32(m.maxAcc)*1.5))
	m.WriteReg(DMAX, int32(m.maxAcc))

	// Max velocity
	m.WriteReg(VMAX, int32(m.maxVel))

	// Always start at min speed
	m.WriteReg(VSTART, 1)
	// Always count a stop as LOW speed, but where VSTOP > VSTART
	m.WriteReg(VSTOP, 10)

	// Transition ramp at 25% speed
	m.WriteReg(V1, int32(float32(m.maxVel)/4))

	return nil
}

// Position gives the current motor position
func (m *TMCStepperMotor) Position(ctx context.Context) (float64, error) {
	return float64(atomic.LoadInt64(&m.steps)) / float64(m.stepsPerRev), nil
}

// PositionSupported returns true.
func (m *TMCStepperMotor) PositionSupported(ctx context.Context) (bool, error) {
	return true, nil
}

// Power TODO (Should it be amps, not throttle?)
func (m *TMCStepperMotor) Power(ctx context.Context, powerPct float32) error {
	return errors.New("power not supported for stepper motors")
}

// Go TODO
// Set a velocity as a percentage of maximum
func (m *TMCStepperMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	mode := MODE_VELPOS
	if d == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
		mode = MODE_VELNEG
	}

	err := m.WriteReg(VMAX, int32(powerPct*float32(m.maxVel)))
	if err != nil {
		return err
	}
	m.WriteReg(RAMPMODE, mode)
	if err != nil {
		return err
	}
	return nil
}

// GoFor turns in the given direction the given number of times at the given speed. Does not block.
func (m *TMCStepperMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error {
	m.logger.Debugf("GoFor Motor: %+v", m)
	// target := (rotations * m.cfg.TicksPerRotation) + m.Position()
	// m.GoTo(rpm, target)
	return nil
}

// GoTo moves to the specified position in terms of rotations.
func (m *TMCStepperMotor) GoTo(ctx context.Context, rpm float64, position float64) error {

	m.bus.Lock()
	defer m.bus.Unlock()

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
