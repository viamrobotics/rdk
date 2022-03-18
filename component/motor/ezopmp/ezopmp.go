// write a motor driver for the hydrogarden pump
package ezopmp

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

type EZOPMPConfig struct {
	motor.Config
	BusName     string `json:"bus_name"`
	I2CAddress  byte   `json:"i2c_address"`
	MaxReadBits int    `json:"max_read_bits"`
}

const modelName = "ezopmp"

func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			// actualBoard, _, err := getBoardFromRobotConfig(r, config)
			// if err != nil {
			// 	return nil, err
			// }

			return NewMotor(ctx, r, config.ConvertedAttributes.(*EZOPMPConfig), logger)
		},
	}
	registry.RegisterComponent(motor.Subtype, modelName, _motor)
	config.RegisterComponentAttributeMapConverter(
		config.ComponentTypeMotor,
		modelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf EZOPMPConfig
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Squash: true, Result: &conf})
			if err != nil {
				return nil, err
			}
			if err := decoder.Decode(attributes); err != nil {
				return nil, err
			}
			return &conf, nil
		}, &EZOPMPConfig{})
}

// A Motor represents a motor connected via the I2C protocol.
type Motor struct {
	board       board.Board
	bus         board.I2C
	I2CAddress  byte
	maxReadBits int
	logger      golog.Logger
	maxPowerPct float64
}

// avaiable commands
const (
	LEDOn                  = "L,1"
	LEDOff                 = "L,0"
	LEDState               = "L,?"
	find                   = "Find"
	singleReading          = "R"
	dispenseTilStop        = "D,*"
	reverseDispenseTilStop = "D,-*"
	dispenseStatus         = "D,?"
	stop                   = "X"
	pumpVoltage            = "PV,?"
	status                 = "Status"
	pause                  = "P"
	pauseStatus            = "P,?"
	totVolDispensed        = "TV,?"
	absoluteTotVolDisp     = "ATV,?"
	clear                  = "clear"
)

// NewMotor returns a motor with I2C protocol.
func NewMotor(ctx context.Context, r robot.Robot, c *EZOPMPConfig, logger golog.Logger) (*Motor, error) {
	b, err := board.FromRobot(r, c.BoardName)
	if err != nil {
		return nil, err
	}

	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("board %s is not local", c.BoardName)
	}
	bus, ok := localB.I2CByName(c.BusName)
	if !ok {
		return nil, errors.Errorf("can't find I2C bus (%s) requested by Motor", c.BusName)
	}

	m := &Motor{
		board:       b,
		bus:         bus,
		I2CAddress:  c.I2CAddress,
		maxReadBits: c.MaxReadBits,
		logger:      logger,
		maxPowerPct: 1.0,
	}

	return m, nil
}

func (m *Motor) writeReg(ctx context.Context, command []byte) error {
	handle, err := m.bus.OpenHandle(m.I2CAddress)
	if err != nil {
		return err
	}
	defer func() {
		if err := handle.Close(); err != nil {
			m.logger.Error(err)
		}
	}()

	return handle.Write(ctx, command)
}

func (m *Motor) readReg(ctx context.Context) ([]byte, error) {
	handle, err := m.bus.OpenHandle(m.I2CAddress)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := handle.Close(); err != nil {
			m.logger.Error(err)
		}
	}()

	readVal := []byte{254, 0}
	for readVal[0] == 254 {
		readVal, err = handle.Read(ctx, m.maxReadBits)
		if err != nil {
			return nil, err
		}
	}

	switch readVal[0] {
	case 1:
		return bytes.Trim(readVal[1:], "\x00"), nil
	case 2:
		return nil, errors.New("syntax error, code: 2")
	case 255:
		return nil, errors.New("no data to send, code: 255")
	case 254:
		return nil, errors.New("data not ready, code: 254")
	default:
		return nil, errors.Errorf("error code not understood %b", readVal[0])
	}
}

func (m *Motor) writeRegWithCheck(ctx context.Context, command []byte) error {
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return writeErr
	}
	_, readErr := m.readReg(ctx)
	if readErr != nil {
		return readErr
	}
	return nil
}

// SetPower sets the percentage of power the motor should employ between -1 and 1.
// Negative power implies a backward directional rotational
// for this pump, it goes between 0.5ml to 105ml/min
func (m *Motor) SetPower(ctx context.Context, powerPct float64) error {
	powerPct = math.Min(powerPct, m.maxPowerPct)
	powerPct = math.Max(powerPct, -1*m.maxPowerPct)
	var command []byte
	powerVal := (powerPct * 104.5) + 0.5
	if powerPct == 0 {
		command = []byte(stop)
	} else {
		stringVal := "DC," + fmt.Sprintf("%f", math.Round(powerVal*100)/100) + ",*"
		fmt.Println(stringVal)
		command = []byte(stringVal)
	}
	return m.writeRegWithCheck(ctx, command)
}

// rpm here will actually map onto the mm/s as calculated based on the datasheet of this motor
// the max rpm: 61, min rpm:1
// every second is assume to be 1 revolution because the rpm on this pump motor remains the same
func (m *Motor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	volPerMin := rpm * 9 * math.Pi * 60 / 1000
	time := revolutions / 60
	commandString := "DC," + fmt.Sprintf("%f", volPerMin) + "," + fmt.Sprintf("%f", time)
	command := []byte(commandString)
	return m.writeRegWithCheck(ctx, command)
}

// using the Dose Over Time Command in the EZO-PMP datasheet
// position = desired mm
// rpm = desired mm/s
func (m *Motor) GoTo(ctx context.Context, rpm float64, positionRevolutions float64) error {
	// if pos is negative, then we have to turn in megative rpm,
	// we cannot have negative time, that doesn't make any sense
	mLPerMin := rpm * 9 * math.Pi * 60 / 1000
	totTimeInMin := math.Abs(positionRevolutions/rpm) / 60
	if positionRevolutions < 0 {
		mLPerMin = mLPerMin * -1
	}
	commandString := "D," + fmt.Sprintf("%f", mLPerMin) + "," + fmt.Sprintf("%f", totTimeInMin)
	command := []byte(commandString)
	return m.writeRegWithCheck(ctx, command)
}

// in this case, we clear the amount of volume that has been dispensed
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64) error {
	command := []byte(clear)
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return writeErr
	}
	val, err := m.readReg(ctx)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println(val)
	return nil
}

// for this pump, it will return the volume dispensed
func (m *Motor) GetPosition(ctx context.Context) (float64, error) {
	command := []byte(totVolDispensed)
	//fmt.Println("getting position")
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return 0, writeErr
	}
	val, err := m.readReg(ctx)
	if err != nil {
		fmt.Println(err)
		return 0, err
	}
	splitMsg := strings.Split(string(val), ",")
	floatVal, err := strconv.ParseFloat(splitMsg[1], 64)
	return floatVal, err
}

// GetFeatures returns the status of optional features on the motor.
func (m *Motor) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: true,
	}, nil
}

func (m *Motor) Stop(ctx context.Context) error {
	command := []byte(stop)
	return m.writeRegWithCheck(ctx, command)
}

func (m *Motor) IsPowered(ctx context.Context) (bool, error) {
	command := []byte(dispenseStatus)
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return false, writeErr
	}
	val, err := m.readReg(ctx)
	if err != nil {
		fmt.Println(err)
		return false, err
	}

	splitMsg := strings.Split(string(val), ",")

	pumpStatus, err := strconv.ParseFloat(splitMsg[2], 64)
	if err != nil {
		return false, err
	}

	fmt.Println(pumpStatus)

	if pumpStatus == 1 {
		return true, nil
	}
	return false, nil
}
