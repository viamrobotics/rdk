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
)

// controllers is global to all instances, mapped by serial device.
var (
	globalMu    sync.Mutex
	controllers map[string]*controller
)

// controller is common across all DMC4000 motor instances sharing a controller.
type controller struct {
	mu         sync.Mutex
	port       io.ReadWriteCloser
	logger     golog.Logger
	activeAxes map[string]bool
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

// Config allows setting controller-wide options.
type Config struct {
	motor.Config
	SerialDevice string `json:"serial_device"`
	Axis         string `json:"axis"`
	HomeRPM      string `json:"home_rpm"`
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
	switch axis {
	case "A":
		fallthrough
	case "B":
		fallthrough
	case "C":
		fallthrough
	case "D":
		fallthrough
	case "E":
		fallthrough
	case "F":
		fallthrough
	case "G":
		fallthrough
	case "H":
		return true
	default:
		return false
	}
}

// NewMotor returns a DMC4000 driven motor.
func NewMotor(ctx context.Context, r robot.Robot, c *Config, logger golog.Logger) (*Motor, error) {
	if c.SerialDevice == "" {
		// TODO Search routine
		return nil, errors.New("couldn't find DMC4000 serial connection")
	}

	globalMu.Lock()
	controller, ok := controllers[c.SerialDevice]
	if !ok {
		controllers[c.SerialDevice] = controller
		controller = controllers[c.SerialDevice]
		controller.activeAxes = make(map[string]bool)
	}
	globalMu.Unlock()

	if !validAxis(c.Axis) {
		return nil, fmt.Errorf("invalid dmc4000 motor axis: %s", c.Axis)
	}

	controller.mu.Lock()
	defer controller.mu.Unlock()
	claimed, ok := controller.activeAxes[c.Axis]
	if !ok || !claimed {
		controller.activeAxes[c.Axis] = true
	}

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
	controller.port = port

	m := &Motor{
		c:                controller,
		Axis:             c.Axis,
		StepsPerRotation: c.TicksPerRotation,
		MaxRPM:           c.MaxRPM,
		MaxAcceleration:  c.MaxAcceleration,
	}

	return m, nil
}

// Close stops the motor and marks the axis inactive.
func (m *Motor) Close() {
	// TODO actual close/shutdown routines
	m.c.activeAxes[m.Axis] = false
}

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

	if bytes.LastIndexByte(ret, []byte(":")[0]) == len(ret) {
		return string(bytes.TrimSpace(ret[:len(ret)-1])), nil
	}

	if bytes.LastIndexByte(ret, []byte("?")[0]) == len(ret) {
		errorDetail, err := c.sendCmd("TC1")
		if err != nil {
			return string(ret), fmt.Errorf("error when trying to get error code from previous command (%s): %w", cmd, err)
		}
		return string(bytes.TrimSpace(ret[:len(ret)-1])), fmt.Errorf("cmd (%s) returned error: %s", cmd, errorDetail)
	}

	return string(ret), fmt.Errorf("unknown error after cmd (%s), response: %s", cmd, string(ret))
}

// func (c *controller) homeAxis(axis string) error {
// 	return nil
// }

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
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	speed := m.rpmToV(math.Abs(powerPct) * m.MaxRPM)

	if math.Signbit(powerPct) {
		speed *= -1
	}

	// Acceleration
	_, err := m.c.sendCmd(fmt.Sprintf("AC%s= %d", m.Axis, m.rpmsToA(m.MaxAcceleration)))
	if err != nil {
		return err
	}

	// Deceleration
	_, err = m.c.sendCmd(fmt.Sprintf("DC%s= %d", m.Axis, m.rpmsToA(m.MaxAcceleration)))
	if err != nil {
		return err
	}

	// Speed
	_, err = m.c.sendCmd(fmt.Sprintf("JG%s= %d", m.Axis, speed))
	if err != nil {
		return err
	}

	return nil
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
			return nil
		}

		// check velocity
		m.c.mu.Lock()
		ret, err := m.c.sendCmd(fmt.Sprintf("TV%s", m.Axis))
		m.c.mu.Unlock()
		if err != nil {
			return err
		}
		vel, err := strconv.Atoi(ret)
		if err != nil {
			return err
		}

		// stopped or decellerating
		if math.Abs(float64(vel)) == 0 {
			// extra check that stop was actually commanded
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

			if sc != 0 && sc != 30 && sc != 50 && sc != 60 && sc != 100 {
				break
			}

			return fmt.Errorf("stop code (%d) indicates motor still running", sc)
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
	_, err := m.c.sendCmd(fmt.Sprintf("DP%s= %d", m.Axis, int(offset*float64(m.StepsPerRotation))))
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
	ret, err := m.c.sendCmd(fmt.Sprintf("TV%s", m.Axis))
	if err != nil {
		return false, err
	}
	speed, err := strconv.Atoi(ret)
	if err != nil {
		return false, err
	}
	return speed != 0, nil
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

func (m *Motor) startHome() error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	// Acceleration
	_, err := m.c.sendCmd(fmt.Sprintf("AC%s= %d", m.Axis, m.rpmsToA(m.MaxAcceleration)))
	if err != nil {
		return err
	}

	// Deceleration
	_, err = m.c.sendCmd(fmt.Sprintf("DC%s= %d", m.Axis, m.rpmsToA(m.MaxAcceleration)))
	if err != nil {
		return err
	}

	// Speed (stage 1)
	_, err = m.c.sendCmd(fmt.Sprintf("SP%s= %d", m.Axis, m.rpmToV(m.HomeRPM)))
	if err != nil {
		return err
	}

	// Speed (stage 2)
	_, err = m.c.sendCmd(fmt.Sprintf("HV%s= %d", m.Axis, m.rpmToV(m.HomeRPM/10)))
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
	_, err := m.c.sendCmd(fmt.Sprintf("PT%s= 1", m.Axis))
	if err != nil {
		return err
	}

	// Acceleration
	_, err = m.c.sendCmd(fmt.Sprintf("AC%s= %d", m.Axis, m.rpmsToA(m.MaxAcceleration)))
	if err != nil {
		return err
	}

	// Deceleration
	_, err = m.c.sendCmd(fmt.Sprintf("DC%s= %d", m.Axis, m.rpmsToA(m.MaxAcceleration)))
	if err != nil {
		return err
	}

	// Speed
	_, err = m.c.sendCmd(fmt.Sprintf("SP%s= %d", m.Axis, m.rpmToV(rpm)))
	if err != nil {
		return err
	}

	// Position target
	_, err = m.c.sendCmd(fmt.Sprintf("PA%s= %d", m.Axis, m.posToSteps(position)))
	if err != nil {
		return err
	}
	return nil
}

// Must be run inside a lock.
func (m *Motor) doPosition() (float64, error) {
	ret, err := m.c.sendCmd("TP" + m.Axis)
	if err != nil {
		return 0, err
	}
	position, err := strconv.ParseFloat(ret, 64)
	if err != nil {
		return 0, err
	}
	return position / float64(m.StepsPerRotation), nil
}
