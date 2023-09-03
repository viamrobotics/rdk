// Package dimensionengineering contains implementations of the dimensionengineering motor controls
package dimensionengineering

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/jacobsa/go-serial/serial"
	"github.com/pkg/errors"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

// https://www.dimensionengineering.com/datasheets/Sabertooth2x60.pdf
var model = resource.DefaultModelFamily.WithModel("de-sabertooth")

// controllers is global to all instances, mapped by serial device.
var (
	globalMu       sync.Mutex
	controllers    map[string]*controller
	validBaudRates = []uint{115200, 38400, 19200, 9600, 2400}
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
	resource.Named
	resource.AlwaysRebuild

	logger golog.Logger
	// A reference to the actual controller that needs to be commanded for the motor to run
	c *controller
	// which channel the motor is connected to on the controller
	Channel int
	// Simply indicates if the RDK _thinks_ the motor is moving, because this controller has no feedback, this may not reflect reality
	isOn bool
	// The current power setting the RDK _thinks_ the motor is running, because this controller has no feedback, this may not reflect reality
	currentPowerPct float64
	// dirFlip means that the motor is wired "backwards" from what we expect forward/backward to mean,
	// so we need to "flip" the direction sent by control
	dirFlip bool
	// the minimum power that can be set for the motor to prevent stalls
	minPowerPct float64
	// the maximum power that can be set for the motor
	maxPowerPct float64
	// the freewheel RPM of the motor
	maxRPM float64

	// A manager to ensure only a single operation is happening at any given time since commands could overlap on the serial port
	opMgr *operation.SingleOperationManager
}

// Config adds DimensionEngineering-specific config options.
type Config struct {
	// path to /dev/ttyXXXX file
	SerialPath string `json:"serial_path"`

	// The baud rate of the controller
	BaudRate int `json:"serial_baud_rate,omitempty"`

	// Valid values are 128-135
	SerialAddress int `json:"serial_address"`

	// Valid values are 1/2
	MotorChannel int `json:"motor_channel"`

	// Flip the direction of the signal sent to the controller.
	// Due to wiring/motor orientation, "forward" on the controller may not represent "forward" on the robot
	DirectionFlip bool `json:"dir_flip,omitempty"`

	// A value to control how quickly the controller ramps to a particular setpoint
	RampValue int `json:"controller_ramp_value,omitempty"`

	// The maximum freewheel rotational velocity of the motor after the final drive (maximum effective wheel speed)
	MaxRPM float64 `json:"max_rpm,omitempty"`

	// The name of the encoder used for this motor
	Encoder string `json:"encoder,omitempty"`

	// The lowest power percentage to allow for this motor. This is used to prevent motor stalls and overheating. Default is 0.0
	MinPowerPct float64 `json:"min_power_pct,omitempty"`

	// The max power percentage to allow for this motor. Default is 0.0
	MaxPowerPct float64 `json:"max_power_pct,omitempty"`

	// The number of ticks per rotation of this motor from the encoder
	TicksPerRotation int `json:"ticks_per_rotation,omitempty"`

	// TestChan is a fake "serial" path for test use only
	TestChan chan []byte `json:"-,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.SerialPath == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "serial_path")
	}

	return nil, nil
}

func init() {
	controllers = make(map[string]*controller)

	resource.RegisterComponent(motor.API, model, resource.Registration[motor.Motor, *Config]{
		Constructor: func(ctx context.Context, _ resource.Dependencies, conf resource.Config, logger golog.Logger) (motor.Motor, error) {
			newConf, err := resource.NativeConfig[*Config](conf)
			if err != nil {
				return nil, err
			}
			return NewMotor(ctx, newConf, conf.ResourceName(), logger)
		},
	})
}

func newController(c *Config, logger golog.Logger) (*controller, error) {
	ctrl := new(controller)
	ctrl.activeAxes = make(map[int]bool)
	ctrl.serialDevice = c.SerialPath
	ctrl.logger = logger
	ctrl.address = c.SerialAddress

	if c.TestChan != nil {
		ctrl.testChan = c.TestChan
	} else {
		serialOptions := serial.OpenOptions{
			PortName:          c.SerialPath,
			BaudRate:          uint(c.BaudRate),
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

func (cfg *Config) populateDefaults() {
	if cfg.BaudRate == 0 {
		cfg.BaudRate = 9600
	}

	if cfg.MaxPowerPct == 0.0 {
		cfg.MaxPowerPct = 1.0
	}
}

func (cfg *Config) validateValues() error {
	errs := make([]string, 0)
	if cfg.MotorChannel != 1 && cfg.MotorChannel != 2 {
		errs = append(errs, fmt.Sprintf("invalid channel %v, acceptable values are 1 and 2", cfg.MotorChannel))
	}
	if cfg.SerialAddress < 128 || cfg.SerialAddress > 135 {
		errs = append(errs, "invalid address, acceptable values are 128 thru 135")
	}
	if !rutils.ValidateBaudRate(validBaudRates, cfg.BaudRate) {
		errs = append(errs, fmt.Sprintf("invalid baud_rate, acceptable values are %v", validBaudRates))
	}
	if cfg.BaudRate != 2400 && cfg.BaudRate != 9600 && cfg.BaudRate != 19200 && cfg.BaudRate != 38400 && cfg.BaudRate != 115200 {
		errs = append(errs, "invalid baud_rate, acceptable values are 2400, 9600, 19200, 38400, 115200")
	}
	if cfg.MinPowerPct < 0.0 || cfg.MinPowerPct > cfg.MaxPowerPct {
		errs = append(errs, "invalid min_power_pct, acceptable values are 0 to max_power_pct")
	}
	if cfg.MaxPowerPct > 1.0 {
		errs = append(errs, "invalid max_power_pct, acceptable values are min_power_pct to 100.0")
	}
	if len(errs) > 0 {
		return fmt.Errorf("error validating sabertooth controller config: %s", strings.Join(errs, "\r\n"))
	}
	return nil
}

// NewMotor returns a Sabertooth driven motor.
func NewMotor(ctx context.Context, c *Config, name resource.Name, logger golog.Logger) (motor.Motor, error) {
	globalMu.Lock()
	defer globalMu.Unlock()

	// populate the default values into the config
	c.populateDefaults()

	// Validate the actual config values make sense
	err := c.validateValues()
	if err != nil {
		return nil, err
	}
	ctrl, ok := controllers[c.SerialPath]
	if !ok {
		newCtrl, err := newController(c, logger)
		if err != nil {
			return nil, err
		}
		controllers[c.SerialPath] = newCtrl
		ctrl = newCtrl
	}

	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()

	// is on a known/supported amplifier only when map entry exists
	claimed, ok := ctrl.activeAxes[c.MotorChannel]
	if !ok {
		return nil, fmt.Errorf("invalid Sabertooth motor axis: %d", c.MotorChannel)
	}
	if claimed {
		return nil, fmt.Errorf("axis %d is already in use", c.MotorChannel)
	}
	ctrl.activeAxes[c.MotorChannel] = true

	m := &Motor{
		Named:       name.AsNamed(),
		c:           ctrl,
		Channel:     c.MotorChannel,
		dirFlip:     c.DirectionFlip,
		minPowerPct: c.MinPowerPct,
		maxPowerPct: c.MaxPowerPct,
		maxRPM:      c.MaxRPM,
		opMgr:       operation.NewSingleOperationManager(),
		logger:      logger,
	}

	if err := m.configure(c); err != nil {
		return nil, err
	}

	if c.RampValue > 0 {
		setRampCmd, err := newCommand(c.SerialAddress, setRamping, c.MotorChannel, byte(c.RampValue))
		if err != nil {
			return nil, err
		}

		err = m.c.sendCmd(setRampCmd)
		if err != nil {
			return nil, err
		}
	}

	return m, nil
}

// IsPowered returns if the motor is currently on or off.
func (m *Motor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	return m.isOn, m.currentPowerPct, nil
}

// Close stops the motor and marks the axis inactive.
func (m *Motor) Close(ctx context.Context) error {
	active := m.isAxisActive()
	if !active {
		return nil
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
			return nil
		}
	}
	if m.c.port != nil {
		err = m.c.port.Close()
		if err != nil {
			m.c.logger.Error(fmt.Errorf("error closing serial connection: %w", err))
		}
	}
	globalMu.Lock()
	defer globalMu.Unlock()
	delete(controllers, m.c.serialDevice)
	return nil
}

func (m *Motor) isAxisActive() bool {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()
	return m.c.activeAxes[m.Channel]
}

// Must be run inside a lock.
func (m *Motor) configure(c *Config) error {
	// Turn off the motor with opMixedDrive and a value of 64 (stop)
	cmd, err := newCommand(m.c.address, singleForward, c.MotorChannel, 0x00)
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
// of power between -1 and 1.
func (m *Motor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	if math.Abs(powerPct) < m.minPowerPct {
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
	m.isOn = true
	m.currentPowerPct = powerPct

	rawSpeed := powerPct * maxSpeed
	switch speed := math.Abs(rawSpeed); {
	case speed < 0.1:
		m.c.logger.Warn("motor speed is nearly 0 rev_per_min")
	case m.maxRPM > 0 && speed > m.maxRPM-0.1:
		m.c.logger.Warnf("motor speed is nearly the max rev_per_min (%f)", m.maxRPM)
	default:
	}
	if math.Signbit(rawSpeed) {
		rawSpeed *= -1
	}

	// Jog
	var cmd commandCode
	if powerPct < 0 {
		// If dirFlip is set, we actually want to reverse the command
		if m.dirFlip {
			cmd = singleForward
		} else {
			cmd = singleBackwards
		}
	} else {
		// If dirFlip is set, we actually want to reverse the command
		if m.dirFlip {
			cmd = singleBackwards
		} else {
			cmd = singleForward
		}
	}
	c, err := newCommand(m.c.address, cmd, m.Channel, byte(int(rawSpeed)))
	if err != nil {
		return errors.Wrap(err, "error in SetPower")
	}
	err = m.c.sendCmd(c)
	return err
}

// GoFor moves an inputted number of revolutions at the given rpm, no encoder is present
// for this so power is determined via a linear relationship with the maxRPM and the distance
// traveled is a time based estimation based on desired RPM.
func (m *Motor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	if m.maxRPM == 0 {
		return motor.NewZeroRPMError()
	}

	powerPct, waitDur := goForMath(m.maxRPM, rpm, revolutions)
	err := m.SetPower(ctx, powerPct, extra)
	if err != nil {
		return errors.Wrap(err, "error in GoFor")
	}

	if revolutions == 0 {
		return nil
	}

	if m.opMgr.NewTimedWaitOp(ctx, waitDur) {
		return m.Stop(ctx, extra)
	}
	return nil
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
// towards the specified target/position.
func (m *Motor) GoTo(ctx context.Context, rpm, position float64, extra map[string]interface{}) error {
	return motor.NewGoToUnsupportedError(fmt.Sprintf("Channel %d on Sabertooth %d", m.Channel, m.c.address))
}

// ResetZeroPosition defines the current position to be zero (+/- offset).
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	return motor.NewResetZeroPositionUnsupportedError(fmt.Sprintf("Channel %d on Sabertooth %d",
		m.Channel, m.c.address))
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

	m.isOn = false
	m.currentPowerPct = 0.0
	cmd, err := newCommand(m.c.address, singleForward, m.Channel, 0)
	if err != nil {
		return err
	}

	err = m.c.sendCmd(cmd)
	return err
}

// IsMoving returns whether the motor is currently moving.
func (m *Motor) IsMoving(ctx context.Context) (bool, error) {
	return m.isOn, nil
}

// DoCommand executes additional commands beyond the Motor{} interface.
func (m *Motor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	name, ok := cmd["command"]
	if !ok {
		return nil, errors.New("missing 'command' value")
	}
	return nil, fmt.Errorf("no such command: %s", name)
}

// Properties returns the additional properties supported by this motor.
func (m *Motor) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{PositionReporting: false}, nil
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
	case setRamping:
		opcode = opRamping
	case setDeadband:
	case multiTurnRight:
	case multiTurnLeft:
	case multiTurn:
	default:
		return nil, fmt.Errorf("opcode %x not implemented", opcode)
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

// If revolutions is 0, the returned wait duration will be 0 representing that
// the motor should run indefinitely.
func goForMath(maxRPM, rpm, revolutions float64) (float64, time.Duration) {
	// need to do this so time is reasonable
	if rpm > maxRPM {
		rpm = maxRPM
	} else if rpm < -1*maxRPM {
		rpm = -1 * maxRPM
	}

	if revolutions == 0 {
		powerPct := rpm / maxRPM
		return powerPct, 0
	}

	dir := rpm * revolutions / math.Abs(revolutions*rpm)
	powerPct := math.Abs(rpm) / maxRPM * dir
	waitDur := time.Duration(math.Abs(revolutions/rpm)*60*1000) * time.Millisecond
	return powerPct, waitDur
}
