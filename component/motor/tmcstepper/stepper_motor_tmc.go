// Package tmcstepper implements a TMC stepper motor.
package tmcstepper

import (
	"context"
	"math"
	"time"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
)

// TMC5072Config extends motor.Config, mainly for RegisterComponentAttributeMapConverter.
type TMC5072Config struct {
	motor.Config
	SPIBus      string  `json:"spi_bus"`
	ChipSelect  string  `json:"chip_select"`
	Index       int     `json:"index"`
	SGThresh    int32   `json:"sg_thresh"`
	HomeRPM     float64 `json:"home_rpm"`
	CalFactor   float64 `json:"cal_factor"`
	RunCurrent  int32   `json:"run_current"`  // 1-32 as a percentage of rsense voltage, 15 default
	HoldCurrent int32   `json:"hold_current"` // 1-32 as a percentage of rsense voltage, 8 default
	HoldDelay   int32   `json:"hold_delay"`   // 0=instant powerdown, 1-15=delay * 2^18 clocks, 6 default
}

const (
	modelname = "TMC5072"
)

func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewMotor(ctx, deps, *config.ConvertedAttributes.(*TMC5072Config), logger)
		},
	}
	registry.RegisterComponent(motor.Subtype, modelname, _motor)

	config.RegisterComponentAttributeMapConverter(
		motor.SubtypeName,
		modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf TMC5072Config
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Squash: true, Result: &conf})
			if err != nil {
				return nil, err
			}
			if err := decoder.Decode(attributes); err != nil {
				return nil, err
			}
			return &conf, nil
		}, &TMC5072Config{})
}

// A Motor represents a brushless motor connected via a TMC controller chip (ex: TMC5072).
type Motor struct {
	board       board.Board
	bus         board.SPI
	csPin       string
	index       int
	enLowPin    board.GPIOPin
	stepsPerRev int
	homeRPM     float64
	maxRPM      float64
	maxAcc      float64
	fClk        float64
	logger      golog.Logger
	opMgr       operation.SingleOperationManager
}

// TMC5072 Values.
const (
	baseClk = 13200000 // Nominal 13.2mhz internal clock speed
	uSteps  = 256      // Microsteps per fullstep
)

// TMC5072 Register Addressses (for motor index 0)
// TODO full register set.
const (
	// add 0x10 for motor 2.
	chopConf  = 0x6C
	coolConf  = 0x6D
	drvStatus = 0x6F

	// add 0x20 for motor 2.
	rampMode = 0x20
	xActual  = 0x21
	// vActual    = 0x22.
	vStart     = 0x23
	a1         = 0x24
	v1         = 0x25
	aMax       = 0x26
	vMax       = 0x27
	dMax       = 0x28
	d1         = 0x2A
	vStop      = 0x2B
	xTarget    = 0x2D
	iHoldIRun  = 0x30
	vCoolThres = 0x31
	swMode     = 0x34
	rampStat   = 0x35
)

// TMC5072 ramp modes.
const (
	modePosition = int32(0)
	modeVelPos   = int32(1)
	modeVelNeg   = int32(2)
	modeHold     = int32(3)
)

// NewMotor returns a TMC5072 driven motor.
func NewMotor(ctx context.Context, deps registry.Dependencies, c TMC5072Config, logger golog.Logger) (motor.LocalMotor, error) {
	b, err := board.FromDependencies(deps, c.BoardName)
	if err != nil {
		return nil, errors.Errorf("%q is not a board", c.BoardName)
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, errors.Errorf("board %s is not local", c.BoardName)
	}
	bus, ok := localB.SPIByName(c.SPIBus)
	if !ok {
		return nil, errors.Errorf("can't find SPI bus (%s) requested by Motor", c.SPIBus)
	}

	if c.CalFactor == 0 {
		c.CalFactor = 1.0
	}

	if c.TicksPerRotation == 0 {
		logger.Warn("ticks_per_rotation isn't set: defaulting to 200")
		c.TicksPerRotation = 200
	}

	if c.HomeRPM == 0 {
		logger.Warn("home_rpm not set: defaulting to 1/4 of max_rpm")
		c.HomeRPM = c.MaxRPM / 4
	}
	c.HomeRPM *= -1

	m := &Motor{
		board:       b,
		bus:         bus,
		csPin:       c.ChipSelect,
		index:       c.Index,
		stepsPerRev: c.TicksPerRotation * uSteps,
		homeRPM:     c.HomeRPM,
		maxRPM:      c.MaxRPM,
		maxAcc:      c.MaxAcceleration,
		fClk:        baseClk / c.CalFactor,
		logger:      logger,
	}

	rawMaxAcc := m.rpmsToA(m.maxAcc)

	if c.SGThresh > 63 {
		c.SGThresh = 63
	} else if c.SGThresh < -64 {
		c.SGThresh = -64
	}
	// The register is a 6 bit signed int
	if c.SGThresh < 0 {
		c.SGThresh = int32(64 + math.Abs(float64(c.SGThresh)))
	}

	// Hold/Run currents are 0-31 (linear scale),
	// but we'll take 1-32 so zero can remain default
	if c.RunCurrent == 0 {
		c.RunCurrent = 15 // Default
	} else {
		c.RunCurrent--
	}

	if c.RunCurrent > 31 {
		c.RunCurrent = 31
	} else if c.RunCurrent < 0 {
		c.RunCurrent = 0
	}

	if c.HoldCurrent == 0 {
		c.HoldCurrent = 8 // Default
	} else {
		c.HoldCurrent--
	}

	if c.HoldCurrent > 31 {
		c.HoldCurrent = 31
	} else if c.HoldCurrent < 0 {
		c.HoldCurrent = 0
	}

	// HoldDelay is 2^18 clocks per step between current stepdown phases
	// Approximately 1/16th of a second for default 16mhz clock
	// Repurposing zero for default, and -1 for "instant"
	if c.HoldDelay == 0 {
		c.HoldDelay = 6 // default
	} else if c.HoldDelay < 0 {
		c.HoldDelay = 0
	}

	if c.HoldDelay > 15 {
		c.HoldDelay = 15
	}

	coolConfig := c.SGThresh << 16

	iCfg := c.HoldDelay<<16 | c.RunCurrent<<8 | c.HoldCurrent

	err = multierr.Combine(
		m.writeReg(ctx, chopConf, 0x000100C3), // TOFF=3, HSTRT=4, HEND=1, TBL=2, CHM=0 (spreadCycle)
		m.writeReg(ctx, iHoldIRun, iCfg),
		m.writeReg(ctx, coolConf, coolConfig), // Sets just the SGThreshold (for now)

		// Set max acceleration and decceleration
		m.writeReg(ctx, a1, rawMaxAcc),
		m.writeReg(ctx, aMax, rawMaxAcc),
		m.writeReg(ctx, d1, rawMaxAcc),
		m.writeReg(ctx, dMax, rawMaxAcc),

		m.writeReg(ctx, vStart, 1),                         // Always start at min speed
		m.writeReg(ctx, vStop, 10),                         // Always count a stop as LOW speed, but where vStop > vStart
		m.writeReg(ctx, v1, m.rpmToV(m.maxRPM/4)),          // Transition ramp at 25% speed (if d1 and a1 are set different)
		m.writeReg(ctx, vCoolThres, m.rpmToV(m.maxRPM/20)), // Set minimum speed for stall detection and coolstep
		m.writeReg(ctx, vMax, m.rpmToV(0)),                 // Max velocity to zero, we don't want to move

		m.writeReg(ctx, rampMode, modeVelPos), // Lastly, set velocity mode to force a stop in case chip was left in moving state
		m.writeReg(ctx, xActual, 0),           // Zero the position
	)
	if err != nil {
		return nil, err
	}

	if c.Pins.EnablePinLow != "" {
		m.enLowPin, err = b.GPIOPinByName(c.Pins.EnablePinLow)
		if err != nil {
			return nil, err
		}
		err = m.Enable(ctx, true)
		if err != nil {
			return nil, err
		}
	}

	return m, nil
}

func (m *Motor) shiftAddr(addr uint8) uint8 {
	// Shift register address for motor 1 instead of motor zero
	if m.index == 1 {
		switch {
		case addr >= 0x10 && addr <= 0x11:
			addr += 0x08
		case addr >= 0x20 && addr <= 0x3C:
			addr += 0x20
		case addr >= 0x6A && addr <= 0x6F:
			addr += 0x10
		}
	}
	return addr
}

func (m *Motor) writeReg(ctx context.Context, addr uint8, value int32) error {
	addr = m.shiftAddr(addr)

	var buf [5]byte
	buf[0] = addr | 0x80
	buf[1] = 0xFF & byte(value>>24)
	buf[2] = 0xFF & byte(value>>16)
	buf[3] = 0xFF & byte(value>>8)
	buf[4] = 0xFF & byte(value)

	handle, err := m.bus.OpenHandle()
	if err != nil {
		return err
	}
	defer func() {
		if err := handle.Close(); err != nil {
			m.logger.Error(err)
		}
	}()

	// m.logger.Debug("Write: ", buf)

	_, err = handle.Xfer(ctx, 1000000, m.csPin, 3, buf[:]) // SPI Mode 3, 1mhz
	if err != nil {
		return err
	}

	return nil
}

func (m *Motor) readReg(ctx context.Context, addr uint8) (int32, error) {
	addr = m.shiftAddr(addr)

	var tbuf [5]byte
	tbuf[0] = addr

	handle, err := m.bus.OpenHandle()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := handle.Close(); err != nil {
			m.logger.Error(err)
		}
	}()

	// m.logger.Debug("ReadT: ", tbuf)

	// Read access returns data from the address sent in the PREVIOUS "packet," so we transmit, then read
	_, err = handle.Xfer(ctx, 1000000, m.csPin, 3, tbuf[:]) // SPI Mode 3, 1mhz
	if err != nil {
		return 0, err
	}

	rbuf, err := handle.Xfer(ctx, 1000000, m.csPin, 3, tbuf[:])
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

	// m.logger.Debug("ReadR: ", rbuf)
	// m.logger.Debug("Read: ", value)

	return value, nil
}

// GetSG returns the current StallGuard reading (effectively an indication of motor load.)
func (m *Motor) GetSG(ctx context.Context) (int32, error) {
	rawRead, err := m.readReg(ctx, drvStatus)
	if err != nil {
		return 0, err
	}

	rawRead &= 1023
	return rawRead, nil
}

// GetPosition gives the current motor position.
func (m *Motor) GetPosition(ctx context.Context) (float64, error) {
	rawPos, err := m.readReg(ctx, xActual)
	if err != nil {
		return 0, err
	}
	return float64(rawPos) / float64(m.stepsPerRev), nil
}

// GetFeatures returns the status of optional features on the motor.
func (m *Motor) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: true,
	}, nil
}

// SetPower sets the motor at a particular rpm based on the percent of
// maxRPM supplied by powerPct (between -1 and 1).
func (m *Motor) SetPower(ctx context.Context, powerPct float64) error {
	m.opMgr.CancelRunning(ctx)
	return m.doJog(ctx, powerPct*m.maxRPM)
}

// Jog sets a fixed RPM.
func (m *Motor) Jog(ctx context.Context, rpm float64) error {
	m.opMgr.CancelRunning(ctx)
	return m.doJog(ctx, rpm)
}

func (m *Motor) doJog(ctx context.Context, rpm float64) error {
	mode := modeVelPos
	if rpm < 0 {
		mode = modeVelNeg
	}
	speed := m.rpmToV(math.Abs(rpm))
	return multierr.Combine(
		m.writeReg(ctx, rampMode, mode),
		m.writeReg(ctx, vMax, speed),
	)
}

// GoFor turns in the given direction the given number of times at the given speed.
// Both the RPM and the revolutions can be assigned negative values to move in a backwards direction.
// Note: if both are negative the motor will spin in the forward direction.
func (m *Motor) GoFor(ctx context.Context, rpm float64, rotations float64) error {
	curPos, err := m.GetPosition(ctx)
	if err != nil {
		return err
	}

	var d int64 = 1
	if math.Signbit(rotations) != math.Signbit(rpm) {
		d *= -1
	}

	rotations = math.Abs(rotations) * float64(d)
	rpm = math.Abs(rpm)

	target := curPos + rotations
	return m.GoTo(ctx, rpm, target)
}

// Convert rpm to TMC5072 steps/s.
func (m *Motor) rpmToV(rpm float64) int32 {
	if rpm > m.maxRPM {
		rpm = m.maxRPM
	}
	// Time constant for velocities in TMC5072
	tConst := m.fClk / math.Pow(2, 24)
	speed := rpm / 60 * float64(m.stepsPerRev) / tConst
	return int32(speed)
}

// Convert rpm/s to TMC5072 steps/taConst^2.
func (m *Motor) rpmsToA(acc float64) int32 {
	// Time constant for accelerations in TMC5072
	taConst := math.Pow(2, 41) / math.Pow(m.fClk, 2)
	rawMaxAcc := acc / 60 * float64(m.stepsPerRev) * taConst
	return int32(rawMaxAcc)
}

// GoTo moves to the specified position in terms of (provided in revolutions from home/zero),
// at a specific speed. Regardless of the directionality of the RPM this function will move the
// motor towards the specified target.
func (m *Motor) GoTo(ctx context.Context, rpm float64, positionRevolutions float64) error {
	ctx, done := m.opMgr.New(ctx)
	defer done()

	positionRevolutions *= float64(m.stepsPerRev)
	err := multierr.Combine(
		m.writeReg(ctx, rampMode, modePosition),
		m.writeReg(ctx, vMax, m.rpmToV(math.Abs(rpm))),
		m.writeReg(ctx, xTarget, int32(positionRevolutions)),
	)
	if err != nil {
		return err
	}

	return m.opMgr.WaitForSuccess(
		ctx,
		time.Millisecond*10,
		m.IsStopped,
	)
}

// IsPowered returns true if the motor is currently moving.
func (m *Motor) IsPowered(ctx context.Context) (bool, error) {
	return m.IsMoving(ctx)
}

// IsStopped returns true if the motor is NOT moving.
func (m *Motor) IsStopped(ctx context.Context) (bool, error) {
	stat, err := m.readReg(ctx, rampStat)
	if err != nil {
		return false, err
	}
	// Look for vzero flag
	return stat&0x400 == 0x400, nil
}

// AtVelocity returns true if the motor has reached the requested velocity.
func (m *Motor) AtVelocity(ctx context.Context) (bool, error) {
	stat, err := m.readReg(ctx, rampStat)
	if err != nil {
		return false, err
	}
	// Look for velocity reached flag
	return stat&0x100 == 0x100, nil
}

// Enable pulls down the hardware enable pin, activating the power stage of the chip.
func (m *Motor) Enable(ctx context.Context, turnOn bool) error {
	if m.enLowPin == nil {
		return errors.New("no enable pin configured")
	}
	return m.enLowPin.Set(ctx, !turnOn)
}

// Stop stops the motor.
func (m *Motor) Stop(ctx context.Context) error {
	m.opMgr.CancelRunning(ctx)
	return m.doJog(ctx, 0)
}

// IsMoving returns true if the motor is currently moving.
func (m *Motor) IsMoving(ctx context.Context) (bool, error) {
	stop, err := m.IsStopped(ctx)
	return !stop, err
}

// Home homes the motor using stallguard.
func (m *Motor) Home(ctx context.Context) error {
	err := m.GoTillStop(ctx, m.homeRPM, nil)
	if err != nil {
		return err
	}
	for {
		stopped, err := m.IsStopped(ctx)
		if err != nil {
			return err
		}
		if stopped {
			break
		}
	}

	return m.ResetZeroPosition(ctx, 0)
}

// GoTillStop enables StallGuard detection, then moves in the direction/speed given until resistance (endstop) is detected.
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	if err := m.Jog(ctx, rpm); err != nil {
		return err
	}
	ctx, done := m.opMgr.New(ctx)
	defer done()

	// Disable stallguard and turn off if we fail homing
	defer func() {
		if err := multierr.Combine(
			m.writeReg(ctx, swMode, 0x000),
			m.doJog(ctx, 0),
		); err != nil {
			m.logger.Error(err)
		}
	}()

	// Get up to speed
	var fails int
	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return errors.New("context cancelled during GoTillStop")
		}

		if stopFunc != nil && stopFunc(ctx) {
			return nil
		}

		ready, err := m.AtVelocity(ctx)
		if err != nil {
			return err
		}

		if ready {
			break
		}

		if fails >= 500 {
			return errors.New("timed out during GoTillStop acceleration")
		}
		fails++
	}

	// Now enable stallguard
	if err := m.writeReg(ctx, swMode, 0x400); err != nil {
		return err
	}

	// Wait for motion to stop at endstop
	fails = 0
	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return errors.New("context cancelled during GoTillStop")
		}

		if stopFunc != nil && stopFunc(ctx) {
			return nil
		}

		stopped, err := m.IsStopped(ctx)
		if err != nil {
			return err
		}
		if stopped {
			break
		}

		if fails >= 10000 {
			return errors.New("timed out during GoTillStop")
		}
		fails++
	}

	return nil
}

// ResetZeroPosition sets the current position of the motor specified by the request
// (adjusted by a given offset) to be its new zero position.
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64) error {
	on, err := m.IsPowered(ctx)
	if err != nil {
		return err
	} else if on {
		return errors.New("can't zero while moving")
	}
	return multierr.Combine(
		m.writeReg(ctx, rampMode, modeHold),
		m.writeReg(ctx, xTarget, int32(offset*float64(m.stepsPerRev))),
		m.writeReg(ctx, xActual, int32(offset*float64(m.stepsPerRev))),
	)
}

// Do() related constants.
const (
	Command = "command"
	Home    = "home"
	Jog     = "jog"
	RPMVal  = "rpm"
)

// Do executes additional commands beyond the Motor{} interface.
func (m *Motor) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	name, ok := cmd["command"]
	if !ok {
		return nil, errors.Errorf("missing %s value", Command)
	}
	switch name {
	case Home:
		return nil, m.Home(ctx)
	case Jog:
		rpmRaw, ok := cmd[RPMVal]
		if !ok {
			return nil, errors.Errorf("need %s value for jog", RPMVal)
		}
		rpm, ok := rpmRaw.(float64)
		if !ok {
			return nil, errors.New("rpm value must be floating point")
		}
		return nil, m.Jog(ctx, rpm)
	default:
		return nil, errors.Errorf("no such command: %s", name)
	}
}
