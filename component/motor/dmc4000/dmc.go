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
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/jacobsa/go-serial/serial"
	"github.com/mitchellh/mapstructure"
	"go.viam.com/utils"

	// "go.viam.com/utils/usb".
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

const (
	modelName = "DMC4000"
	// D4140 amplifier model.
	D4140 = "D4140"
)

// controllers is global to all instances, mapped by serial device.
var (
	globalMu    sync.Mutex
	controllers map[string]*controller
)

// controller is common across all DMC4000 motor instances sharing a controller.
type controller struct {
	mu             sync.Mutex
	port           io.ReadWriteCloser
	serialDevice   string
	logger         golog.Logger
	activeAxes     map[string]bool
	amplifierModel string
}

// Motor is a single axis/motor/component instance.
type Motor struct {
	c                *controller
	Axis             string
	StepsPerRotation int
	MaxRPM           float64
	MaxAcceleration  float64
	HomeRPM          float64
}

// Config adds DMC-specific config options.
type Config struct {
	motor.Config
	SerialDevice string  `json:"serial_device"` // path to /dev/ttyXXXX file
	Axis         string  `json:"axis"`          // A-H
	HomeRPM      float64 `json:"home_rpm"`      // Speed for Home()

	// Model of the built-in amplifier (different for steppers, brushed/etc)
	// D4140 (stepper) is the only one supported currently
	AmplifierModel string `json:"amplifier_model"`
	// Set the per phase current (when using stepper amp)
	// https://www.galil.com/download/comref/com4103/index.html#amplifier_gain.html
	AmplifierGain int `json:"amplifier_gain"`
	// Can reduce current when holding
	// https://www.galil.com/download/comref/com4103/index.html#low_current_stepper_mode.html
	LowCurrent int `json:"low_current"`
}

func init() {
	controllers = make(map[string]*controller)

	_motor := registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			m, err := NewMotor(ctx, r, config.ConvertedAttributes.(*Config), logger)
			if err != nil {
				return nil, err
			}
			return m, nil
		},
	}
	registry.RegisterComponent(motor.Subtype, modelName, _motor)

	config.RegisterComponentAttributeMapConverter(
		config.ComponentTypeMotor,
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
		}, &Config{})
}

func validAxis(axis string) bool {
	c := []byte(axis)[0]
	// ascii A through H
	if c >= 65 && c <= 72 && len(axis) == 1 {
		return true
	}
	return false
}

// NewMotor returns a DMC4000 driven motor.
func NewMotor(ctx context.Context, r robot.Robot, c *Config, logger golog.Logger) (*Motor, error) {
	if c.SerialDevice == "" {
		// TODO Search routine
		return nil, errors.New("couldn't find DMC4000 serial connection")
	}

	globalMu.Lock()
	ctrl, ok := controllers[c.SerialDevice]
	if !ok {
		ctrl = new(controller)
		controllers[c.SerialDevice] = ctrl
		ctrl.activeAxes = make(map[string]bool)
		ctrl.serialDevice = c.SerialDevice
		ctrl.amplifierModel = c.AmplifierModel
		ctrl.logger = logger

		if ctrl.amplifierModel == "" {
			ctrl.amplifierModel = D4140 // only one supported for now
		}

		// TODO (James): Search for usb/serial device when not set

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
	globalMu.Unlock()

	if ctrl.amplifierModel != D4140 {
		return nil, errors.New("only amplifier model D4140 (stepper motor) is supported")
	}

	if !validAxis(c.Axis) {
		return nil, fmt.Errorf("invalid dmc4000 motor axis: %s", c.Axis)
	}

	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()
	claimed, ok := ctrl.activeAxes[c.Axis]
	if !ok || !claimed {
		ctrl.activeAxes[c.Axis] = true
	}

	m := &Motor{
		c:                ctrl,
		Axis:             c.Axis,
		StepsPerRotation: c.TicksPerRotation,
		MaxRPM:           c.MaxRPM,
		MaxAcceleration:  c.MaxAcceleration,
		HomeRPM:          c.HomeRPM,
	}

	if m.MaxRPM <= 0 {
		m.MaxRPM = 1000
	}

	if m.MaxAcceleration <= 0 {
		m.MaxAcceleration = 1000
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
	err := m.Stop(context.Background())
	if err != nil {
		m.c.logger.Error(err)
	}

	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	delete(m.c.activeAxes, m.Axis)
	for _, active := range m.c.activeAxes {
		if active {
			return
		}
	}
	err = m.c.port.Close()
	if err != nil {
		m.c.logger.Error(err)
	}
	globalMu.Lock()
	defer globalMu.Unlock()
	delete(controllers, m.c.serialDevice)
}

// Must be run inside a lock.
func (m *Motor) configure(c *Config) error {
	switch m.c.amplifierModel {
	case D4140:
		m.StepsPerRotation *= 64 // fixed microstepping

		// Stepper type, with optional reversing
		motorType := "2" // string because no trailing zeros
		if c.DirFlip {
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
		return fmt.Errorf("unsupported amplifier model: %s", m.c.amplifierModel)
	}
}

// Must be run inside a lock.
func (c *controller) sendCmd(cmd string) (string, error) {
	_, err := c.port.Write([]byte(cmd + "\r\n"))
	if err != nil {
		return "", err
	}

	var ret []byte
	for {
		buf := make([]byte, 256)
		n, err := c.port.Read(buf)
		if err != nil {
			return string(ret), err
		}
		ret = append(ret, buf[:n]...)
		if bytes.ContainsAny(buf[:n], ":?") {
			break
		}
	}

	if bytes.LastIndexByte(ret, []byte(":")[0]) == len(ret)-1 {
		return string(bytes.TrimSpace(ret[:len(ret)-1])), nil
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
func (m *Motor) rpmToV(rpm float64) int32 {
	rpm = math.Abs(rpm)
	if rpm > m.MaxRPM {
		rpm = m.MaxRPM
	}
	speed := int32(rpm * float64(m.StepsPerRotation) / 60)

	// Hard limits from controller
	if speed > 3000000 {
		speed = 3000000
	}
	return speed
}

// Convert rpm/s to DMC4000 counts/sec^2.
func (m *Motor) rpmsToA(rpms float64) int32 {
	rpms = math.Abs(rpms)
	if rpms > m.MaxAcceleration {
		rpms = m.MaxAcceleration
	}
	acc := int32(rpms * float64(m.StepsPerRotation) / 60)

	// Hard limits from controller
	if acc > 1073740800 {
		acc = 1073740800
	} else if acc < 1024 {
		acc = 1024
	}
	return acc
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

// SetPower sets the percentage of power the motor should employ between 0-1.
func (m *Motor) SetPower(ctx context.Context, powerPct float64) error {
	return errors.New("power not supported for stepper motors")
}

// Go instructs the motor to go in a specific direction at a percentage
// of power between -1 and 1. Scaled to maxRPM.
func (m *Motor) Go(ctx context.Context, powerPct float64) error {
	if math.Abs(powerPct) < 0.001 {
		return m.Stop(ctx)
	}

	return m.GoFor(ctx, powerPct*m.MaxRPM, 100000)
}

// GoFor instructs the motor to go in a specific direction for a specific amount of
// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
// can be assigned negative values to move in a backwards direction. Note: if both are
// negative the motor will spin in the forward direction.
func (m *Motor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
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
	return m.doGoTo(rpm, goal)
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
// towards the specified target/position.
func (m *Motor) GoTo(ctx context.Context, rpm float64, position float64) error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	return m.doGoTo(rpm, position)
}

// GoTillStop moves a motor until stopped by the controller (due to switch or function) or stopFunc.
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	if err := m.GoFor(ctx, rpm, 10000); err != nil {
		return err
	}
	defer func() {
		if err := m.Stop(ctx); err != nil {
			m.c.logger.Error(err)
		}
	}()

	var fails int
	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return errors.New("context cancelled during GoTillStop")
		}

		if stopFunc != nil && stopFunc(ctx) {
			break
		}

		m.c.mu.Lock()
		stopped, err := m.isStopped()
		m.c.mu.Unlock()
		if err != nil {
			return err
		}

		if stopped {
			break
		}

		if fails >= 6000 {
			return errors.New("timed out during GoTillStop")
		}
		fails++
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

// Position reports the position in revolutions.
func (m *Motor) Position(ctx context.Context) (float64, error) {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	return m.doPosition()
}

// PositionSupported returns whether or not the motor supports reporting of its position.
func (m *Motor) PositionSupported(ctx context.Context) (bool, error) {
	return true, nil
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (m *Motor) Stop(ctx context.Context) error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	_, err := m.c.sendCmd(fmt.Sprintf("ST%s", m.Axis))
	return err
}

// IsOn returns whether or not the motor is currently moving.
func (m *Motor) IsOn(ctx context.Context) (bool, error) {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	stopped, err := m.isStopped()
	if err != nil {
		return false, err
	}
	return !stopped, nil
}

// Must be run inside a lock.
func (m *Motor) isStopped() (bool, error) {
	// check that stop was actually commanded
	ret, err := m.c.sendCmd(fmt.Sprintf("SC%s", m.Axis))
	if err != nil {
		return false, err
	}
	sc, err := strconv.Atoi(ret)
	if err != nil {
		return false, err
	}

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
	// start homing (self-locking)
	if err := m.startHome(); err != nil {
		return err
	}

	// wait for routine to finish
	var fails int
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

		if sc == 10 {
			return nil
		}

		if fails >= 6000 {
			return errors.New("timed out during Home")
		}
		fails++
	}
}

// Does its own locking.
func (m *Motor) startHome() error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	// Speed (stage 1)
	_, err := m.c.sendCmd(fmt.Sprintf("SP%s=%d", m.Axis, m.rpmToV(m.HomeRPM)))
	if err != nil {
		return err
	}

	// Speed (stage 2)
	_, err = m.c.sendCmd(fmt.Sprintf("HV%s=%d", m.Axis, m.rpmToV(m.HomeRPM/10)))
	if err != nil {
		return err
	}

	_, err = m.c.sendCmd(fmt.Sprintf("HM%s", m.Axis))
	if err != nil {
		return err
	}

	_, err = m.c.sendCmd(fmt.Sprintf("BG%s", m.Axis))
	if err != nil {
		return err
	}

	return nil
}

// Must be run inside a lock.
func (m *Motor) doGoTo(rpm float64, position float64) error {
	// Position tracking mode
	_, err := m.c.sendCmd(fmt.Sprintf("PT%s=1", m.Axis))
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
