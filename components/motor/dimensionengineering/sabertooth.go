// Package dimensionengineering contains implementations of the dimensionengineering motor controls
package dimensionengineering

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/jacobsa/go-serial/serial"
	"github.com/mitchellh/mapstructure"
	utils "go.viam.com/utils"
	"go.viam.com/utils/usb"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	// https://www.dimensionengineering.com/datasheets/Sabertooth2x60.pdf
	modelName = "de-sabertooth"
)

// controllers is global to all instances, mapped by serial device.
var (
	globalMu    sync.Mutex
	controllers map[string]*controller
	usbFilter   = usb.SearchFilter{}
)

// controller is common across all Sabertooth motor instances sharing a controller.
type controller struct {
	mu           sync.Mutex
	port         io.ReadWriteCloser
	serialDevice string
	logger       golog.Logger
	activeAxes   map[int]bool
	testChan     chan []byte
	address      int // 128-135
}

// Motor is a single axis/motor/component instance.
type Motor struct {
	c       *controller
	Channel int
	jogging bool
	opMgr   operation.SingleOperationManager
}

// GoFor Not supported.
func (m *Motor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	return errors.New("not supported")
}

// IsPowered returns if the motor is currently on or off.
func (m *Motor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, error) {
	return m.jogging, nil
}

// Config adds DimensionEngineering-specific config options.
type Config struct {
	SerialDevice string `json:"serial_device"` // path to /dev/ttyXXXX file
	Channel      int    `json:"channel"`       // 1/2
	// TestChan is a fake "serial" path for test use only
	TestChan chan []byte `json:"-,omitempty"`
	Address  int         `json:"address"` // 128-135
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) error {
	if cfg.SerialDevice == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "serial_device")
	}
	if cfg.Channel < 1 || cfg.Channel > 2 {
		return utils.NewConfigValidationFieldRequiredError(path, "channel")
	}
	if cfg.Address < 128 || cfg.Address > 135 {
		return utils.NewConfigValidationFieldRequiredError(path, "address")
	}

	return nil
}

func init() {
	controllers = make(map[string]*controller)

	_motor := registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			conf, ok := config.ConvertedAttributes.(*Config)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(conf, config.ConvertedAttributes)
			}
			return NewMotor(ctx, conf, logger)
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

// NewMotor returns a Sabertooth driven motor.
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
			return nil, errors.New("couldn't find Sabertooth serial connection")
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
	claimed, ok := ctrl.activeAxes[c.Channel]
	if !ok {
		return nil, fmt.Errorf("invalid Sabertooth motor axis: %d", c.Channel)
	}
	if claimed {
		return nil, fmt.Errorf("axis %d is already in use", c.Channel)
	}
	ctrl.activeAxes[c.Channel] = true

	m := &Motor{
		c:       ctrl,
		Channel: c.Channel,
	}

	if err := m.configure(c); err != nil {
		return nil, err
	}

	return m, nil
}

// Close stops the motor and marks the axis inactive.
func (m *Motor) Close() {
	m.c.mu.Lock()
	active := m.c.activeAxes[m.Channel]
	m.c.mu.Unlock()
	if !active {
		return
	}
	err := m.Stop(context.Background(), nil)
	if err != nil {
		m.c.logger.Error(err)
	}

	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	m.c.activeAxes[m.Channel] = false
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
	if c.Address < 128 || c.Address > 135 {
		return nil, errors.New("address is out of range. valid addresses are 127 to 135")
	}
	ctrl := new(controller)
	ctrl.activeAxes = make(map[int]bool)
	ctrl.serialDevice = c.SerialDevice
	ctrl.logger = logger
	ctrl.address = c.Address

	if c.TestChan != nil {
		ctrl.testChan = c.TestChan
	} else {
		serialOptions := serial.OpenOptions{
			PortName:          c.SerialDevice,
			BaudRate:          9600,
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

	ctrl.activeAxes[1] = false
	ctrl.activeAxes[2] = false

	return ctrl, nil
}

// Must be run inside a lock.
func (m *Motor) configure(c *Config) error {
	// Turn off the motor with opMixedDrive and a value of 64 (stop)
	cmd, err := newCommand(m.c.address, singleForward, c.Channel, 0x00)
	if err != nil {
		return err
	}
	err = m.c.sendCmd(cmd)
	return err
}

// Must be run inside a lock.
func (c *controller) sendCmd(cmd *command) error {
	packet := cmd.ToPacket()
	if c.testChan != nil {
		c.testChan <- packet
		return nil
	}
	_, err := c.port.Write(packet)
	return err
}

// SetPower instructs the motor to go in a specific direction at a percentage
// of power between -1 and 1. Scaled to MaxRPM.
func (m *Motor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	if math.Abs(powerPct) < 0.001 {
		return m.Stop(ctx, extra)
	}
	if powerPct > 1 {
		powerPct = 1
	} else if powerPct < -1 {
		powerPct = -1
	}

	m.opMgr.CancelRunning(ctx)
	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	m.jogging = true

	rawSpeed := powerPct * maxSpeed
	if math.Signbit(rawSpeed) {
		rawSpeed *= -1
	}

	// Jog
	var cmd commandCode
	if powerPct < 0 {
		cmd = singleBackwards
	} else {
		cmd = singleForward
	}
	c, err := newCommand(m.c.address, cmd, m.Channel, byte(int(rawSpeed)))
	if err != nil {
		return err
	}
	err = m.c.sendCmd(c)
	return err
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
// towards the specified target/position.
func (m *Motor) GoTo(ctx context.Context, rpm, position float64, extra map[string]interface{}) error {
	return errors.New("not supported")
}

// GoTillStop moves a motor until stopped by the controller (due to switch or function) or stopFunc.
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return motor.NewGoTillStopUnsupportedError("(name unavailable)")
}

// ResetZeroPosition defines the current position to be zero (+/- offset).
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	return errors.New("not supported")
}

// Position reports the position in revolutions.
func (m *Motor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 0, nil
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (m *Motor) Stop(ctx context.Context, extra map[string]interface{}) error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	_, done := m.opMgr.New(ctx)
	defer done()

	m.jogging = false
	cmd, err := newCommand(m.c.address, singleForward, m.Channel, 0)
	if err != nil {
		return err
	}

	err = m.c.sendCmd(cmd)
	return err
}

// IsMoving returns whether the motor is currently moving.
func (m *Motor) IsMoving(ctx context.Context) (bool, error) {
	return m.jogging, nil
}

// DoCommand executes additional commands beyond the Motor{} interface.
func (m *Motor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	name, ok := cmd["command"]
	if !ok {
		return nil, errors.New("missing 'command' value")
	}
	return nil, fmt.Errorf("no such command: %s", name)
}

// Properties returns the additional features supported by this motor.
func (m *Motor) Properties(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{"PositionReporting": false}, nil
}

type command struct {
	Address  byte
	Op       byte
	Data     byte
	Checksum byte
}

func newCommand(controllerAddress int, motorMode commandCode, channel int, data byte) (*command, error) {
	var opcode opCode
	switch motorMode {
	case singleForward:
		switch channel {
		case 1:
			opcode = opMotor1Forward
		case 2:
			opcode = opMotor2Forward
		default:
			return nil, errors.New("invalid motor channel")
		}
	case singleBackwards:
		switch channel {
		case 1:
			opcode = opMotor1Backwards
		case 2:
			opcode = opMotor2Backwards
		default:
			return nil, errors.New("invalid motor channel")
		}
	case singleDrive:
		switch channel {
		case 1:
			opcode = opMotor1Drive
		case 2:
			opcode = opMotor2Drive
		default:
			return nil, errors.New("invalid motor channel")
		}
	case multiForward:
		opcode = opMultiDriveForward
	case multiBackward:
		opcode = opMultiDriveForward
	case multiDrive:
		opcode = opMultiDrive
	case multiTurnRight:
	case multiTurnLeft:
	case multiTurn:
	default:
		return nil, errors.New("opcode not implemented")
	}
	sum := byte(controllerAddress) + byte(opcode) + data
	checksum := sum & 0x7F
	return &command{
		Address:  byte(controllerAddress),
		Op:       byte(opcode),
		Data:     data,
		Checksum: checksum,
	}, nil
}

func (c *command) ToPacket() []byte {
	return []byte{c.Address, c.Op, c.Data, c.Checksum}
}
