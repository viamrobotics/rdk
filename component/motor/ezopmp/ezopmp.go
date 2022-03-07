// write a motor driver for the hydrogarden pump
package ezopmp

import (
	"context"
	"fmt"
	"math"
	"strconv"

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
	BusName    string `json:"bus_name"`
	I2CAddress byte   `json:"i2c_address"`
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
	board      board.Board
	bus        board.I2C
	I2CAddress byte
	logger     golog.Logger
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
		board:      b,
		bus:        bus,
		I2CAddress: c.I2CAddress,
		logger:     logger,
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

func (m *Motor) readReg(ctx context.Context, command []byte) error {
	handle, err := m.bus.OpenHandle(m.I2CAddress)
	if err != nil {
		return err
	}
	defer func() {
		if err := handle.Close(); err != nil {
			m.logger.Error(err)
		}
	}()

	readInput, err := strconv.Atoi(string(command))
	readVal, err := handle.Read(ctx, readInput)
	fmt.Println(readVal)
	return err
}

// SetPower sets the percentage of power the motor should employ between -1 and 1.
// Negative power implies a backward directional rotational
// for this pump, it goes between 0.5ml to 105ml/min
func (m *Motor) SetPower(ctx context.Context, powerPct float64) error {
	command := []byte{}
	powerVal := (powerPct / 100 * 104.5) + 0.5
	if powerPct < 0 {
		stringVal := "DC," + fmt.Sprintf("%f", -1*math.Round(powerVal*100)/100) + ",*"
		fmt.Println(stringVal)
		command = []byte(stringVal)
	} else if powerPct == 0 {
		command = []byte(stop)
	} else {
		stringVal := "DC," + fmt.Sprintf("%f", math.Round(powerVal*100)/100) + ",*"
		fmt.Println(stringVal)
		command = []byte(stringVal)
	}
	fmt.Println(command)
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return errors.New(writeErr.Error())
	}
	return nil
}

func (m *Motor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	return nil
}

func (m *Motor) GoTo(ctx context.Context, rpm float64, positionRevolutions float64) error {
	return nil
}

func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64) error {
	return nil
}

// for this pump, it will return the volume dispensed
func (m *Motor) GetPosition(ctx context.Context) (float64, error) {
	return 0, nil
}

// GetFeatures returns the status of optional features on the motor.
func (m *Motor) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: true,
	}, nil
}

func (m *Motor) Stop(ctx context.Context) error {
	command := []byte(stop)
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return errors.New(writeErr.Error())
	}
	return nil
}

func (m *Motor) IsPowered(ctx context.Context) (bool, error) {
	// command := []byte(status)
	// readErr := m.read
	return false, nil
}

// allows you to set and change the I2C address of the device
func (m *Motor) SetAddress() {

}

func (m *Motor) ControlLED(command string) error {
	return nil
}
