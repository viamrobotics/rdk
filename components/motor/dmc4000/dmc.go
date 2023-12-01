// Package dmc4000 implements stepper motors behind a Galil DMC4000 series motor controller
package dmc4000

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jacobsa/go-serial/serial"
	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/usb"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
)

// Timeout for Home().
const homeTimeout = time.Minute

var model = resource.DefaultModelFamily.WithModel("DMC4000")

// controllers is global to all instances, mapped by serial device.
var (
	globalMu    sync.Mutex
	controllers map[string]*controller
	usbFilter   = usb.SearchFilter{}
)

// controller is common across all DMC4000 motor instances sharing a controller.
type controller struct {
	mu           sync.Mutex
	port         io.ReadWriteCloser
	serialDevice string
	logger       logging.Logger
	activeAxes   map[string]bool
	ampModel1    string
	ampModel2    string
	testChan     chan string
}

// Motor is a single axis/motor/component instance.
type Motor struct {
	resource.Named
	resource.AlwaysRebuild
	c                *controller
	Axis             string
	TicksPerRotation int
	maxRPM           float64
	MaxAcceleration  float64
	HomeRPM          float64
	jogging          bool
	opMgr            *operation.SingleOperationManager
	powerPct         float64
	logger           logging.Logger
}

// Config adds DMC-specific config options.
type Config struct {
	resource.TriviallyValidateConfig
	DirectionFlip    bool    `json:"dir_flip,omitempty"` // Flip the direction of the signal sent if there is a Dir pin
	MaxRPM           float64 `json:"max_rpm,omitempty"`
	MaxAcceleration  float64 `json:"max_acceleration_rpm_per_sec,omitempty"`
	TicksPerRotation int     `json:"ticks_per_rotation"`
	SerialDevice     string  `json:"serial_path"`     // path to /dev/ttyXXXX file
	Axis             string  `json:"controller_axis"` // A-H
	HomeRPM          float64 `json:"home_rpm"`        // Speed for Home()

	// Set the per phase current (when using stepper amp)
	// https://www.galil.com/download/comref/com4103/index.html#amplifier_gain.html
	AmplifierGain int `json:"amplifier_gain"`
	// Can reduce current when holding
	// https://www.galil.com/download/comref/com4103/index.html#low_current_stepper_mode.html
	LowCurrent int `json:"low_current"`

	// TestChan is a fake "serial" path for test use only
	TestChan chan string `json:"-"`
}

func init() {
	controllers = make(map[string]*controller)

	resource.RegisterComponent(motor.API, model, resource.Registration[motor.Motor, *Config]{
		Constructor: func(
			ctx context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger,
		) (motor.Motor, error) {
			newConf, err := resource.NativeConfig[*Config](conf)
			if err != nil {
				return nil, err
			}
			return NewMotor(ctx, newConf, conf.ResourceName(), logger)
		},
	})
}

// NewMotor returns a DMC4000 driven motor.
func NewMotor(ctx context.Context, c *Config, name resource.Name, logger logging.Logger) (motor.Motor, error) {
	if c.SerialDevice == "" {
		devs := usb.Search(usbFilter, func(vendorID, productID int) bool {
			if vendorID == 0x403 && productID == 0x6001 {
				return true
			}
			return false
		})

		if len(devs) > 0 {
			c.SerialDevice = devs[0].Path
		} else {
			return nil, errors.New("couldn't find DMC4000 serial connection")
		}
	}

	globalMu.Lock()
	ctrl, ok := controllers[c.SerialDevice]
	if !ok {
		newCtrl, err := newController(c, logger)
		if err != nil {
			return nil, err
		}
		controllers[c.SerialDevice] = newCtrl
		ctrl = newCtrl
	}
	globalMu.Unlock()

	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()

	// is on a known/supported amplifier only when map entry exists
	claimed, ok := ctrl.activeAxes[c.Axis]
	if !ok {
		return nil, fmt.Errorf("invalid dmc4000 motor axis: %s", c.Axis)
	}
	if claimed {
		return nil, fmt.Errorf("axis %s is already in use", c.Axis)
	}
	ctrl.activeAxes[c.Axis] = true

	if c.TicksPerRotation == 0 {
		return nil, errors.New("expected ticks_per_rotation in config for motor")
	}

	m := &Motor{
		Named:            name.AsNamed(),
		c:                ctrl,
		Axis:             c.Axis,
		TicksPerRotation: c.TicksPerRotation,
		maxRPM:           c.MaxRPM,
		MaxAcceleration:  c.MaxAcceleration,
		HomeRPM:          c.HomeRPM,
		powerPct:         0.0,
		opMgr:            operation.NewSingleOperationManager(),
		logger:           logger,
	}

	if m.maxRPM <= 0 {
		m.maxRPM = 1000 // arbitrary high value
	}

	if m.MaxAcceleration <= 0 {
		m.MaxAcceleration = 1000 // rpm/s, arbitrary/safe value (1s to max)
	}

	if m.HomeRPM <= 0 {
		m.HomeRPM = m.maxRPM / 4
	}

	if err := m.configure(c); err != nil {
		return nil, err
	}

	return m, nil
}

// Close stops the motor and marks the axis inactive.
func (m *Motor) Close(ctx context.Context) error {
	m.c.mu.Lock()
	active := m.c.activeAxes[m.Axis]
	m.c.mu.Unlock()
	if !active {
		return nil
	}
	err := m.Stop(context.Background(), nil)
	if err != nil {
		m.c.logger.Error(err)
	}

	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	m.c.activeAxes[m.Axis] = false
	for _, active = range m.c.activeAxes {
		if active {
			return nil
		}
	}
	if m.c.port != nil {
		err = m.c.port.Close()
		if err != nil {
			m.c.logger.Error(err)
		}
	}
	globalMu.Lock()
	defer globalMu.Unlock()
	delete(controllers, m.c.serialDevice)
	return nil
}

func newController(c *Config, logger logging.Logger) (*controller, error) {
	ctrl := new(controller)
	ctrl.activeAxes = make(map[string]bool)
	ctrl.serialDevice = c.SerialDevice
	ctrl.logger = logger

	if c.TestChan != nil {
		ctrl.testChan = c.TestChan
	} else {
		serialOptions := serial.OpenOptions{
			PortName:          c.SerialDevice,
			BaudRate:          115200,
			DataBits:          8,
			StopBits:          1,
			MinimumReadSize:   1,
			RTSCTSFlowControl: true,
		}

		port, err := serial.Open(serialOptions)
		if err != nil {
			return nil, err
		}
		ctrl.port = port
	}

	// Set echo off to not scramble our returns
	_, err := ctrl.sendCmd("EO 0")
	if err != nil && !strings.HasPrefix(err.Error(), "unknown error after cmd") {
		return nil, err
	}

	ret, err := ctrl.sendCmd("ID")
	if err != nil {
		return nil, err
	}

	var modelNum string
	for _, line := range strings.Split(ret, "\n") {
		tokens := strings.Split(line, ", ")
		switch tokens[0] {
		case "DMC":
			modelNum = tokens[1]
			logger.Infof("Found DMC4000 (%s) on port: %s\n%s", modelNum, c.SerialDevice, ret)
		case "AMP1":
			ctrl.ampModel1 = tokens[1]
		case "AMP2":
			ctrl.ampModel2 = tokens[1]
		}
	}

	if modelNum != "4000" && modelNum != "4103" && modelNum != "4200" {
		return nil, fmt.Errorf("unsupported DMC model number: %s", modelNum)
	}

	if ctrl.ampModel1 != "44140" && ctrl.ampModel2 != "44140" {
		return nil, fmt.Errorf("unsupported amplifier model(s) found, amp1: %s, amp2, %s", ctrl.ampModel1, ctrl.ampModel2)
	}

	// Add to the map if it's a known amp
	if ctrl.ampModel1 == "44140" {
		ctrl.activeAxes["A"] = false
		ctrl.activeAxes["B"] = false
		ctrl.activeAxes["C"] = false
		ctrl.activeAxes["D"] = false
	}

	if ctrl.ampModel2 == "44140" {
		ctrl.activeAxes["E"] = false
		ctrl.activeAxes["F"] = false
		ctrl.activeAxes["G"] = false
		ctrl.activeAxes["H"] = false
	}

	return ctrl, nil
}

// Must be run inside a lock.
func (m *Motor) configure(c *Config) error {
	var amp string
	if m.Axis == "A" || m.Axis == "B" || m.Axis == "C" || m.Axis == "D" {
		amp = m.c.ampModel1
	} else {
		amp = m.c.ampModel2
	}

	switch amp {
	case "44140":
		m.TicksPerRotation *= 64 // fixed microstepping

		// Stepper type, with optional reversing
		motorType := "2" // string because no trailing zeros
		if c.DirectionFlip {
			motorType = "2.5"
		}

		// Turn off the motor
		_, err := m.c.sendCmd(fmt.Sprintf("MO%s", m.Axis))
		if err != nil {
			return err
		}

		// Set motor type to stepper (possibly reversed)
		_, err = m.c.sendCmd(fmt.Sprintf("MT%s=%s", m.Axis, motorType))
		if err != nil {
			return err
		}

		// Set amplifier gain
		_, err = m.c.sendCmd(fmt.Sprintf("AG%s=%d", m.Axis, c.AmplifierGain))
		if err != nil {
			return err
		}

		// Set low current mode
		_, err = m.c.sendCmd(fmt.Sprintf("LC%s=%d", m.Axis, c.LowCurrent))
		if err != nil {
			return err
		}

		// Acceleration
		_, err = m.c.sendCmd(fmt.Sprintf("AC%s=%d", m.Axis, m.rpmsToA(m.MaxAcceleration)))
		if err != nil {
			return err
		}

		// Deceleration
		_, err = m.c.sendCmd(fmt.Sprintf("DC%s=%d", m.Axis, m.rpmsToA(m.MaxAcceleration)))
		if err != nil {
			return err
		}

		// Enable the motor
		_, err = m.c.sendCmd(fmt.Sprintf("SH%s", m.Axis))
		return err

	default:
		return fmt.Errorf("unsupported amplifier model: %s", amp)
	}
}

// Must be run inside a lock.
func (c *controller) sendCmd(cmd string) (string, error) {
	if c.testChan != nil {
		c.testChan <- cmd
	} else {
		_, err := c.port.Write([]byte(cmd + "\r\n"))
		if err != nil {
			return "", err
		}
	}

	var ret []byte
	for {
		buf := make([]byte, 4096)
		if c.testChan != nil {
			ret = []byte(<-c.testChan)
			break
		} else {
			n, err := c.port.Read(buf)
			if err != nil {
				return string(ret), err
			}
			ret = append(ret, buf[:n]...)
			if bytes.ContainsAny(buf[:n], ":?") {
				break
			}
		}
	}
	if bytes.LastIndexByte(ret, []byte(":")[0]) == len(ret)-1 {
		ret := string(bytes.TrimSpace(ret[:len(ret)-1]))
		// c.logger.Debugf("CMD (%s) OK: %s", cmd, ret)
		return ret, nil
	}

	if bytes.LastIndexByte(ret, []byte("?")[0]) == len(ret)-1 {
		errorDetail, err := c.sendCmd("TC1")
		if err != nil {
			return string(ret), fmt.Errorf("error when trying to get error code from previous command (%s): %w", cmd, err)
		}
		return string(bytes.TrimSpace(ret[:len(ret)-1])), fmt.Errorf("cmd (%s) returned error: %s", cmd, errorDetail)
	}

	return string(ret), fmt.Errorf("unknown error after cmd (%s), response: %s", cmd, string(ret))
}

// Convert rpm to DMC4000 counts/sec.
func (m *Motor) rpmToV(rpm float64) int {
	rpm = math.Abs(rpm)
	if rpm > m.maxRPM {
		rpm = m.maxRPM
	}
	speed := rpm * float64(m.TicksPerRotation) / 60

	// Hard limits from controller
	if speed > 3000000 {
		speed = 3000000
	}
	return int(speed)
}

// Convert rpm/s to DMC4000 counts/sec^2.
func (m *Motor) rpmsToA(rpms float64) int {
	rpms = math.Abs(rpms)
	if rpms > m.MaxAcceleration {
		rpms = m.MaxAcceleration
	}
	acc := rpms * float64(m.TicksPerRotation) / 60

	// Hard limits from controller
	if acc > 1073740800 {
		acc = 1073740800
	} else if acc < 1024 {
		acc = 1024
	}
	return int(acc)
}

// Convert revolutions to steps.
func (m *Motor) posToSteps(pos float64) int32 {
	goal := int32(pos * float64(m.TicksPerRotation))

	// Hard limits from controller
	if goal > 2147483647 {
		goal = 2147483647
	} else if goal < -2147483648 {
		goal = -2147483648
	}
	return goal
}

// SetPower instructs the motor to go in a specific direction at a percentage
// of power between -1 and 1. Scaled to MaxRPM.
func (m *Motor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	powerPct = math.Min(powerPct, 1.0)
	powerPct = math.Max(powerPct, -1.0)

	switch pow := math.Abs(powerPct); {
	case pow < 0.1:
		m.c.logger.CWarn(ctx, "motor speed is nearly 0 rev_per_min")
		return m.Stop(ctx, extra)
	case m.maxRPM > 0 && pow*m.maxRPM > m.maxRPM-0.1:
		m.c.logger.CWarnf(ctx, "motor speed is nearly the max rev_per_min (%f)", m.maxRPM)
	default:
	}

	m.powerPct = powerPct
	return m.Jog(ctx, powerPct*m.maxRPM)
}

// Jog moves indefinitely at the specified RPM.
func (m *Motor) Jog(ctx context.Context, rpm float64) error {
	m.opMgr.CancelRunning(ctx)
	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	m.jogging = true

	rawSpeed := m.rpmToV(rpm)
	if math.Signbit(rpm) {
		rawSpeed *= -1
	}

	// Jog
	_, err := m.c.sendCmd(fmt.Sprintf("JG%s=%d", m.Axis, rawSpeed))
	if err != nil {
		return err
	}

	// Begin action
	_, err = m.c.sendCmd(fmt.Sprintf("BG%s", m.Axis))
	if err != nil {
		return err
	}
	return nil
}

func (m *Motor) stopJog() error {
	if m.jogging {
		m.jogging = false
		_, err := m.c.sendCmd(fmt.Sprintf("ST%s", m.Axis))
		return err
	}
	return nil
}

// GoFor instructs the motor to go in a specific direction for a specific amount of
// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
// can be assigned negative values to move in a backwards direction. Note: if both are
// negative the motor will spin in the forward direction.
func (m *Motor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	switch speed := math.Abs(rpm); {
	case speed < 0.1:
		m.c.logger.CWarn(ctx, "motor speed is nearly 0 rev_per_min")
		return motor.NewZeroRPMError()
	case m.maxRPM > 0 && speed > m.maxRPM-0.1:
		m.c.logger.CWarnf(ctx, "motor speed is nearly the max rev_per_min (%f)", m.maxRPM)
	default:
	}
	ctx, done := m.opMgr.New(ctx)
	defer done()

	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	curPos, err := m.doPosition()
	if err != nil {
		return err
	}
	if math.Signbit(rpm) {
		revolutions *= -1
	}
	goal := curPos + revolutions
	err = m.doGoTo(rpm, goal)
	if err != nil {
		return err
	}

	return m.opMgr.WaitForSuccess(
		ctx,
		time.Millisecond*10,
		m.isStopped,
	)
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
// towards the specified target/position.
func (m *Motor) GoTo(ctx context.Context, rpm, position float64, extra map[string]interface{}) error {
	ctx, done := m.opMgr.New(ctx)
	defer done()

	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	if err := m.doGoTo(rpm, position); err != nil {
		return motor.NewGoToUnsupportedError(m.Name().ShortName())
	}

	return m.opMgr.WaitForSuccess(
		ctx,
		time.Millisecond*10,
		m.isStopped,
	)
}

// ResetZeroPosition defines the current position to be zero (+/- offset).
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	_, err := m.c.sendCmd(fmt.Sprintf("DP%s=%d", m.Axis, int(-1*offset*float64(m.TicksPerRotation))))
	if err != nil {
		return errors.Wrap(err, "error in ResetZeroPosition")
	}
	return err
}

// Position reports the position in revolutions.
func (m *Motor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	return m.doPosition()
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (m *Motor) Stop(ctx context.Context, extra map[string]interface{}) error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	ctx, done := m.opMgr.New(ctx)
	defer done()

	m.jogging = false
	_, err := m.c.sendCmd(fmt.Sprintf("ST%s", m.Axis))
	if err != nil {
		return errors.Wrap(err, "error in Stop function")
	}

	return m.opMgr.WaitForSuccess(
		ctx,
		time.Millisecond*10,
		m.isStopped,
	)
}

// IsMoving returns whether or not the motor is currently moving.
func (m *Motor) IsMoving(ctx context.Context) (bool, error) {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	stopped, err := m.isStopped(ctx)
	if err != nil {
		return false, err
	}
	return !stopped, nil
}

// IsPowered returns whether or not the motor is currently moving.
func (m *Motor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	on, err := m.IsMoving(ctx)
	if err != nil {
		return on, m.powerPct, errors.Wrap(err, "error in IsPowered")
	}
	return on, m.powerPct, err
}

// Must be run inside a lock.
func (m *Motor) isStopped(ctx context.Context) (bool, error) {
	// check that stop was actually commanded
	ret, err := m.c.sendCmd(fmt.Sprintf("SC%s", m.Axis))
	if err != nil {
		return false, err
	}
	sc, err := strconv.Atoi(ret)
	if err != nil {
		return false, err
	}

	// Stop codes that indicate the motor is NOT actually stopped
	// https://www.galil.com/download/comref/com4103/index.html#stop_code.html
	if sc == 0 || sc == 30 || sc == 50 || sc == 60 || sc == 100 {
		return false, nil
	}

	// check that total error is zero (not coasting)
	ret, err = m.c.sendCmd(fmt.Sprintf("TE%s", m.Axis))
	if err != nil {
		return false, err
	}
	te, err := strconv.Atoi(ret)
	if err != nil {
		return false, err
	}

	if te != 0 {
		return false, nil
	}

	return true, nil
}

// Home runs the dmc homing routine.
func (m *Motor) Home(ctx context.Context) error {
	ctx, done := m.opMgr.New(ctx)
	defer done()

	// start homing (self-locking)
	if err := m.startHome(); err != nil {
		return err
	}

	// wait for routine to finish
	defer func() {
		if err := m.Stop(ctx, nil); err != nil {
			m.c.logger.Error(err)
		}
	}()

	startTime := time.Now()
	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return errors.New("context cancelled during Home")
		}

		// Wait for
		m.c.mu.Lock()
		ret, err := m.c.sendCmd(fmt.Sprintf("SC%s", m.Axis))
		m.c.mu.Unlock()
		if err != nil {
			return err
		}
		sc, err := strconv.Atoi(ret)
		if err != nil {
			return err
		}

		// stop code 10 indicates homing sequence finished
		if sc == 10 {
			return nil
		}

		if time.Since(startTime) >= homeTimeout {
			return errors.New("timed out during Home")
		}
	}
}

// Does its own locking.
func (m *Motor) startHome() error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	// Exit jog mode if in it
	err := m.stopJog()
	if err != nil {
		return err
	}

	// Speed (stage 1)
	_, err = m.c.sendCmd(fmt.Sprintf("SP%s=%d", m.Axis, m.rpmToV(m.HomeRPM)))
	if err != nil {
		return err
	}

	// Speed (stage 2)
	_, err = m.c.sendCmd(fmt.Sprintf("HV%s=%d", m.Axis, m.rpmToV(m.HomeRPM/10)))
	if err != nil {
		return err
	}

	// Homing action
	_, err = m.c.sendCmd(fmt.Sprintf("HM%s", m.Axis))
	if err != nil {
		return err
	}

	// Begin action
	_, err = m.c.sendCmd(fmt.Sprintf("BG%s", m.Axis))
	if err != nil {
		return err
	}

	return nil
}

// Must be run inside a lock.
func (m *Motor) doGoTo(rpm, position float64) error {
	// Exit jog mode if in it
	err := m.stopJog()
	if err != nil {
		return err
	}

	// Position tracking mode
	_, err = m.c.sendCmd(fmt.Sprintf("PT%s=1", m.Axis))
	if err != nil {
		return err
	}

	switch speed := math.Abs(rpm); {
	case speed < 0.1:
		m.c.logger.Warn("motor speed is nearly 0 rev_per_min")
	case m.maxRPM > 0 && speed > m.maxRPM-0.1:
		m.c.logger.Warnf("motor speed is nearly the max rev_per_min (%f)", m.maxRPM)
	default:
	}

	// Speed
	_, err = m.c.sendCmd(fmt.Sprintf("SP%s=%d", m.Axis, m.rpmToV(rpm)))
	if err != nil {
		return err
	}

	// Position target
	_, err = m.c.sendCmd(fmt.Sprintf("PA%s=%d", m.Axis, m.posToSteps(position)))
	if err != nil {
		return err
	}
	return nil
}

// Must be run inside a lock.
func (m *Motor) doPosition() (float64, error) {
	ret, err := m.c.sendCmd(fmt.Sprintf("RP%s", m.Axis))
	if err != nil {
		return 0, err
	}
	position, err := strconv.ParseFloat(ret, 64)
	if err != nil {
		return 0, err
	}
	return position / float64(m.TicksPerRotation), nil
}

// DoCommand executes additional commands beyond the Motor{} interface.
func (m *Motor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	name, ok := cmd["command"]
	if !ok {
		return nil, errors.New("missing 'command' value")
	}
	switch name {
	case "home":
		return nil, m.Home(ctx)
	case "jog":
		rpmRaw, ok := cmd["rpm"]
		if !ok {
			return nil, errors.New("need rpm value for jog")
		}
		rpm, ok := rpmRaw.(float64)
		if !ok {
			return nil, errors.New("rpm value must be floating point")
		}
		return nil, m.Jog(ctx, rpm)
	case "raw":
		raw, ok := cmd["raw_input"]
		if !ok {
			return nil, errors.New("need raw string to send to controller")
		}
		m.c.mu.Lock()
		defer m.c.mu.Unlock()
		retVal, err := m.c.sendCmd(raw.(string))
		ret := map[string]interface{}{"return": retVal}
		return ret, err
	default:
		return nil, fmt.Errorf("no such command: %s", name)
	}
}

// Properties returns the additional properties supported by this motor.
func (m *Motor) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{PositionReporting: true}, nil
}
