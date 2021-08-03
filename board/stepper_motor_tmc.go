package board

import (
	"context"
	"math"
	"time"

	"github.com/go-errors/errors"

	pb "go.viam.com/core/proto/api/v1"

	"github.com/edaniels/golog"

	"go.uber.org/multierr"
)

// TMCConfig extends the MotorConfig for this specific series of drivers
type TMCConfig struct {
	SPIBus      string  `json:"spiBus"`      // SPI Bus name
	CSPin       string  `json:"csPin"`  // Motor address on serial bus
	Index       uint    `json:"index"`       // 0th or 1st motor on driver
	MaxVelocity float64 `json:"maxVelocity"` // RPM
	MaxAccel    float64 `json:"maxAccel"`    // RPM per second
	SGThresh    int32   `json:"sgThresh"`    // StallGuard threshhold for homing. -64 to +63, 0 default.
	CalFactor   float64 `json:"calFactor"`   // Ratio of time taken/exepected for a move at a given speed
}

// A TMCStepperMotor represents a brushless motor connected via a TMC controller chip (ex: TMC5072)
type TMCStepperMotor struct {
	board       Board
	bus         SPI
	csPin       string
	index       uint
	enPin       string
	stepsPerRev int
	maxVel      float64
	maxAcc      float64
	fClk        float64
	logger      golog.Logger
}

// TMC5072 Values
const (
	baseClk = 13200000 // Nominal 13.2mhz internal clock speed
	uSteps  = 256      // Microsteps per fullstep
)

// TMC5072 Register Addressses (for motor index 0)
// TODO full register set
const (
	// add 0x10 for motor 2
	CHOPCONF = 0x6C
	COOLCONF = 0X6D
	DRV_STATUS = 0X6F

	// add 0x20 for motor 2
	RAMPMODE   = 0x20
	XACTUAL    = 0x21
	VACTUAL    = 0x22
	VSTART     = 0x23
	A1         = 0x24
	V1         = 0x25
	AMAX       = 0x26
	VMAX       = 0x27
	DMAX       = 0x28
	D1         = 0x2A
	VSTOP      = 0x2B
	XTARGET    = 0x2D
	IHOLD_IRUN = 0x30
	VCOOLTHRS  = 0x31
	SW_MODE    = 0x34
	RAMP_STAT  = 0x35
)

// TMC5072 ramp modes
const (
	MODE_POSITION = int32(0)
	MODE_VELPOS   = int32(1)
	MODE_VELNEG   = int32(2)
	MODE_HOLD     = int32(3)
)

// NewTMCStepperMotor returns a TMC5072 driven motor
func NewTMCStepperMotor(b Board, mc MotorConfig, logger golog.Logger) (*TMCStepperMotor, error) {
	bus, ok := b.SPIByName(mc.TMCConfig.SPIBus)
	if !ok {
		return nil, errors.Errorf("TMCStepperMotor can't find SPI bus named %s", mc.TMCConfig.SPIBus)
	}

	m := &TMCStepperMotor{
		board:       b,
		bus:         bus,
		csPin:       mc.TMCConfig.CSPin,
		index:       mc.TMCConfig.Index,
		stepsPerRev: mc.TicksPerRotation * uSteps,
		enPin:       mc.Pins["en"],
		maxVel:      mc.TMCConfig.MaxVelocity,
		maxAcc:      mc.TMCConfig.MaxAccel,
		fClk:        baseClk / mc.TMCConfig.CalFactor,
		logger:      logger,
	}

	rawMaxAcc := m.rpmsToA(m.maxAcc)


	if mc.TMCConfig.SGThresh > 63 {
		mc.TMCConfig.SGThresh = 63
	}else if mc.TMCConfig.SGThresh < -64 {
		mc.TMCConfig.SGThresh = -64
	}

	// The register is a 6 bit signed int
	if mc.TMCConfig.SGThresh < 0 {
		mc.TMCConfig.SGThresh = int32(64 + math.Abs(float64(mc.TMCConfig.SGThresh)))
	}

	var coolConfig int32
	coolConfig = mc.TMCConfig.SGThresh << 16

	errors := multierr.Combine(
		m.WriteReg(CHOPCONF, 0x000100C3),   // TOFF=3, HSTRT=4, HEND=1, TBL=2, CHM=0 (spreadCycle)
		m.WriteReg(IHOLD_IRUN, 0x00080F0A), // IHOLD=8 (half current), IRUN=15 (max current), IHOLDDELAY=6
		m.WriteReg(COOLCONF, coolConfig),   // Sets just the SGThreshold (for now)

		// Set max acceleration and decceleration
		m.WriteReg(A1, rawMaxAcc),
		m.WriteReg(AMAX, rawMaxAcc),
		m.WriteReg(D1, rawMaxAcc),
		m.WriteReg(DMAX, rawMaxAcc),

		m.WriteReg(VSTART, 1),                              // Always start at min speed
		m.WriteReg(VSTOP, 10),                              // Always count a stop as LOW speed, but where VSTOP > VSTART
		m.WriteReg(V1, int32(float32(m.maxVel)/4)),         // Transition ramp at 25% speed (if D1 and A1 are set different)
		m.WriteReg(VCOOLTHRS, int32(float32(m.maxVel)/20)), // Set minimium speed for stall detection and coolstep
		m.WriteReg(VMAX, m.rpmToV(0)),                      // Max velocity to zero, we don't want to move

		m.WriteReg(RAMPMODE, MODE_VELPOS),                  // Lastly, set velocity mode to force a stop in case chip was left in moving state
		m.WriteReg(XACTUAL, 0),                             // Zero the position
	)

	if errors != nil {
		return nil, errors
	}

	return m, nil
}

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

	handle, err := m.bus.Open()
	if err != nil {
		return err
	}
	defer handle.Close()

	//m.logger.Debug("Write: ", buf)

	_, err = handle.Xfer(1000000, m.csPin, 3, buf) // SPI Mode 3, 1mhz
	if err != nil {
		return err
	}

	return nil
}

func (m *TMCStepperMotor) ReadReg(addr uint8) (int32, error) {

	addr = m.shiftAddr(addr)

	tbuf := make([]byte, 5)
	tbuf[0] = addr

	handle, err := m.bus.Open()
	if err != nil {
		return 0, err
	}
	defer handle.Close()

	//m.logger.Debug("ReadT: ", tbuf)

	// Read access returns data from the address sent in the PREVIOUS "packet," so we transmit, then read
	_, err = handle.Xfer(1000000, m.csPin, 3, tbuf) // SPI Mode 3, 1mhz
	if err != nil {
		return 0, err
	}

	rbuf, err := handle.Xfer(1000000, m.csPin, 3, tbuf)
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

	//m.logger.Debug("ReadR: ", rbuf)
	//m.logger.Debug("Read: ", value)

	return value, nil

}

func (m *TMCStepperMotor) GetRaw(ctx context.Context) Motor {
	return m
}

func (m *TMCStepperMotor) GetSG(ctx context.Context) (int32, error) {
	rawRead, err := m.ReadReg(DRV_STATUS)

	if err != nil {
		return 0, err
	}

	rawRead &= 1023
	return rawRead, nil
}

// Position gives the current motor position
func (m *TMCStepperMotor) Position(ctx context.Context) (float64, error) {
	rawPos, err := m.ReadReg(XACTUAL)
	if err != nil {
		return 0, err
	}
	return float64(rawPos) / float64(m.stepsPerRev), nil
}

// PositionSupported returns true.
func (m *TMCStepperMotor) PositionSupported(ctx context.Context) (bool, error) {
	return true, nil
}

// Power TODO (Should it be amps, not throttle?)
func (m *TMCStepperMotor) Power(ctx context.Context, powerPct float32) error {
	return errors.New("power not supported for stepper motors")
}

// Set a velocity as a percentage of maximum
func (m *TMCStepperMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	mode := MODE_VELPOS
	if d == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
		mode = MODE_VELNEG
	}
	speed := m.rpmToV(float64(powerPct) * m.maxVel)

	return multierr.Combine(
		m.WriteReg(RAMPMODE, mode),
		m.WriteReg(VMAX, speed),
	)
}

// GoFor turns in the given direction the given number of times at the given speed. Does not block.
func (m *TMCStepperMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error {
	curPos, err := m.Position(ctx)
	if err != nil {
		return err
	}

	if d == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
		rotations *= -1
	}
	target := curPos + rotations
	return m.GoTo(ctx, rpm, target)
}

// Convert rpm to TMC5072 steps/s
func (m *TMCStepperMotor) rpmToV(rpm float64) int32 {
	if rpm > m.maxVel {
		rpm = m.maxVel
	}
	// Time constant for velocities in TMC5072
	tConst := m.fClk / math.Pow(2, 24)
	speed := rpm / 60 * float64(m.stepsPerRev) / tConst
	return int32(speed)
}

// Convert rpm/s to TMC5072 steps/taConst^2
func (m *TMCStepperMotor) rpmsToA(acc float64) int32 {
	// Time constant for accelerations in TMC5072
	taConst := math.Pow(2, 41) / math.Pow(m.fClk, 2)
	rawMaxAcc := acc / 60 * float64(m.stepsPerRev) * taConst
	return int32(rawMaxAcc)
}

// GoTo moves to the specified position in terms of rotations.
func (m *TMCStepperMotor) GoTo(ctx context.Context, rpm float64, position float64) error {
	position *= float64(m.stepsPerRev)
	return multierr.Combine(
		m.WriteReg(RAMPMODE, MODE_POSITION),
		m.WriteReg(VMAX, m.rpmToV(rpm)),
		m.WriteReg(XTARGET, int32(position)),
	)
}

// IsOn returns true if the motor is currently moving.
func (m *TMCStepperMotor) IsOn(ctx context.Context) (bool, error) {
	vel, err := m.ReadReg(VACTUAL)
	on := vel != 0
	return on, err
}

// Enable pulls down the hardware enable pin, activating the power stage of the chip
func (m *TMCStepperMotor) Enable(turnOn bool) error {
	return m.board.GPIOSet(m.enPin, !turnOn)
}

// Off stops the motor.
func (m *TMCStepperMotor) Off(ctx context.Context) error {
	return m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 0)
}

func (m *TMCStepperMotor) Home(ctx context.Context, d pb.DirectionRelative, rpm float64) error {
	err := m.GoFor(ctx, d, rpm, 1000)
	if err != nil {
		return err
	}

	// Disable stallguard and turn off if we fail homing
	defer func() {
		_ = m.WriteReg(SW_MODE, 0x000)
		_ = m.Off(ctx)
	}()

	// Get up to speed
	var fails int
	for {
		// sg, _ := m.GetSG(ctx)
		// m.logger.Debugf("SGValueSpeed: %d", sg)

		select {
		case <-ctx.Done():
			return errors.New("Context cancelled during homing")
		case <-time.After(100 * time.Millisecond):
			fails++
		}

		stat, err := m.ReadReg(RAMP_STAT)
		if err != nil {
			return err
		}
		// Look for velocity_reached flag
		if stat&0x100 == 0x100 {
			break
		}

		if fails >= 50 {
			return errors.New("Timed out during homing accel")
		}
	}

	// Now enabled stallguard
	err = m.WriteReg(SW_MODE, 0x400)
	if err != nil {
		return err
	}

	// Wait for motion to stop at endstop
	fails = 0
	for {
		// sg, _ := m.GetSG(ctx)
		// m.logger.Debugf("SGValueReady: %d", sg)

		select {
		case <-ctx.Done():
			return errors.New("Context cancelled during homing")
		case <-time.After(100 * time.Millisecond):
			fails++
		}

		stat, err := m.ReadReg(RAMP_STAT)
		if err != nil {
			return err
		}
		// Look for vzero flag
		if stat&0x400 == 0x400 {
			break
		}

		if fails >= 100 {
			return errors.New("Timed out during homing")
		}
	}

	// Stop
	err = m.Off(ctx)
	if err != nil {
		return err
	}
	// Zero position
	err = m.Zero(ctx)
	if err != nil {
		return err
	}

	return nil
}

// Zero resets the current position to zero.
func (m *TMCStepperMotor) Zero(ctx context.Context) error {
	return m.WriteReg(XACTUAL, 0)
}

func (m *TMCStepperMotor) PositionReached(ctx context.Context) bool {
		stat, err := m.ReadReg(RAMP_STAT)
		if err != nil {
			return false
		}
		// Look for position_reached flag
		if stat&0x200 == 0x200 {
			return true
		}
		return false
}