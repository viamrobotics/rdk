// Package dmc4000 implements stepper motors behind a Galil DMC4000 series motor controller
package dmc4000

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/jacobsa/go-serial/serial"
	"github.com/mitchellh/mapstructure"
	"go.viam.com/utils"
	"go.viam.com/utils/usb"

	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
)

const (
	modelName = "DMC4000"

	// Timeout for Home() and GoTillStop().
	homeTimeout = time.Minute
)

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
	logger       golog.Logger
	activeAxes   map[string]bool
	ampModel1    string
	ampModel2    string
	testChan     chan string
}

// Motor is a single axis/motor/component instance.
type Motor struct {
	c                *controller
	Axis             string
	StepsPerRotation int
	MaxRPM           float64
	MaxAcceleration  float64
	HomeRPM          float64
	jogging          bool
	opMgr            operation.SingleOperationManager
}

// Config adds DMC-specific config options.
type Config struct {
	motor.Config
	SerialDevice string  `json:"serial_device"`   // path to /dev/ttyXXXX file
	Axis         string  `json:"controller_axis"` // A-H
	HomeRPM      float64 `json:"home_rpm"`        // Speed for Home()

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

	_motor := registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewMotor(ctx, config.ConvertedAttributes.(*Config), logger)
		},
	}
	registry.RegisterComponent(motor.Subtype, modelName, _motor)

	config.RegisterComponentAttributeMapConverter(
		motor.SubtypeName,
		modelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Squash: true, Result: &conf})
			if err != nil {
				return nil, err
			}
			if err := decoder.Decode(attributes); err != nil {
				return nil, err
			}
			return &conf, nil
		},
		&Config{},
	)
}

// NewMotor returns a DMC4000 driven motor.
func NewMotor(ctx context.Context, c *Config, logger golog.Logger) (motor.LocalMotor, error) {
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

	m := &Motor{
		c:                ctrl,
		Axis:             c.Axis,
		StepsPerRotation: c.TicksPerRotation,
		MaxRPM:           c.MaxRPM,
		MaxAcceleration:  c.MaxAcceleration,
		HomeRPM:          c.HomeRPM,
	}

	if m.StepsPerRotation <= 0 {
		m.StepsPerRotation = 200 // standard for most steppers
	}

	if m.MaxRPM <= 0 {
		m.MaxRPM = 1000 // arbitrary high value
	}

	if m.MaxAcceleration <= 0 {
		m.MaxAcceleration = 1000 // rpm/s, arbitrary/safe value (1s to max)
	}

	if m.HomeRPM <= 0 {
		m.HomeRPM = m.MaxRPM / 4
	}

	if err := m.configure(c); err != nil {
		return nil, err
	}

	return m, nil
}

// Close stops the motor and marks the axis inactive.
func (m *Motor) Close() {
	m.c.mu.Lock()
	active := m.c.activeAxes[m.Axis]
	m.c.mu.Unlock()
	if !active {
		return
	}
	err := m.Stop(context.Background())
	if err != nil {
		m.c.logger.Error(err)
	}

	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	m.c.activeAxes[m.Axis] = false
	for _, active = range m.c.activeAxes {
		if active {
			return
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
}

func newController(c *Config, logger golog.Logger) (*controller, error) {
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
		m.StepsPerRotation *= 64 // fixed microstepping

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
	if rpm > m.MaxRPM {
		rpm = m.MaxRPM
	}
	speed := rpm * float64(m.StepsPerRotation) / 60

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
	acc := rpms * float64(m.StepsPerRotation) / 60

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
	goal := int32(pos * float64(m.StepsPerRotation))

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
func (m *Motor) SetPower(ctx context.Context, powerPct float64) error {
	if math.Abs(powerPct) < 0.001 {
		return m.Stop(ctx)
	}
	return m.Jog(ctx, powerPct*m.MaxRPM)
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
func (m *Motor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
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
func (m *Motor) GoTo(ctx context.Context, rpm float64, position float64) error {
	ctx, done := m.opMgr.New(ctx)
	defer done()

	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	if err := m.doGoTo(rpm, position); err != nil {
		return err
	}

	return m.opMgr.WaitForSuccess(
		ctx,
		time.Millisecond*10,
		m.isStopped,
	)
}

// GoTillStop moves a motor until stopped by the controller (due to switch or function) or stopFunc.
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	if err := m.Jog(ctx, rpm); err != nil {
		return err
	}

	ctx, done := m.opMgr.New(ctx)
	defer done()

	defer func() {
		if err := m.Stop(ctx); err != nil {
			m.c.logger.Error(err)
		}
	}()

	startTime := time.Now()
	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return errors.New("context cancelled during GoTillStop")
		}

		if stopFunc != nil && stopFunc(ctx) {
			break
		}

		m.c.mu.Lock()
		stopped, err := m.isStopped(ctx)
		m.c.mu.Unlock()
		if err != nil {
			return err
		}

		if stopped {
			break
		}

		if time.Since(startTime) >= homeTimeout {
			return errors.New("timed out during GoTillStop")
		}
	}

	return nil
}

// ResetZeroPosition defines the current position to be zero (+/- offset).
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64) error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	_, err := m.c.sendCmd(fmt.Sprintf("DP%s=%d", m.Axis, int(offset*float64(m.StepsPerRotation))))
	return err
}

// GetPosition reports the position in revolutions.
func (m *Motor) GetPosition(ctx context.Context) (float64, error) {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	return m.doPosition()
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (m *Motor) Stop(ctx context.Context) error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	ctx, done := m.opMgr.New(ctx)
	defer done()

	m.jogging = false
	_, err := m.c.sendCmd(fmt.Sprintf("ST%s", m.Axis))
	if err != nil {
		return err
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
func (m *Motor) IsPowered(ctx context.Context) (bool, error) {
	return m.IsMoving(ctx)
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
		if err := m.Stop(ctx); err != nil {
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
func (m *Motor) doGoTo(rpm float64, position float64) error {
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
	return position / float64(m.StepsPerRotation), nil
}

// Do executes additional commands beyond the Motor{} interface.
func (m *Motor) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
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

// GetFeatures returns the additional features supported by this motor.
func (m *Motor) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{motor.PositionReporting: true}, nil
}
