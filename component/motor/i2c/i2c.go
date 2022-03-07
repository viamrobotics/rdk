// write a motor driver for the hydrogarden pump
package i2c

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

type I2CConfig struct {
	motor.Config
	I2CBusName string `json:"i2c_bus_name"`
	I2CAddress string `json:"i2c_address"`
}

const modelName = "i2c"

func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			actualBoard, motorConfig, err := getBoardFromRobotConfig(r, config)
			if err != nil {
				return nil, err
			}

			return NewMotor(ctx, actualBoard, *motorConfig, logger)
		},
	}
	registry.RegisterComponent(motor.Subtype, modelName, _motor)
	motor.RegisterConfigAttributeConverter(modelName)
}

// A Motor represents a motor connected via the I2C protocol.
type Motor struct {
	board      board.Board
	bus        board.I2C
	I2CAddress uint8
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
)

func getBoardFromRobotConfig(r robot.Robot, config config.Component) (board.Board, *motor.Config, error) {
	motorConfig, ok := config.ConvertedAttributes.(*motor.Config)
	if !ok {
		return nil, nil, utils.NewUnexpectedTypeError(motorConfig, config.ConvertedAttributes)
	}
	if motorConfig.BoardName == "" {
		return nil, nil, errors.New("expected board name in config for motor")
	}
	b, err := board.FromRobot(r, motorConfig.BoardName)
	if err != nil {
		return nil, nil, err
	}
	return b, motorConfig, nil
}

// NewMotor returns a motor with I2C protocol.
func NewMotor(ctx context.Context, b board.Board, c motor.Config, logger golog.Logger) (*Motor, error) {
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("board %s is not local", c.BoardName)
	}
	busName := "bus1"
	bus, ok := localB.I2CByName(busName)
	if !ok {
		return nil, errors.Errorf("can't find I2C bus (%s) requested by Motor", busName)
	}

	m := &Motor{
		board:      b,
		bus:        bus,
		I2CAddress: 103,
		logger:     logger,
	}

	return m, nil
}

func (m *Motor) writeReg(ctx context.Context, command []byte) error {

	// no
	handle, err := m.bus.OpenHandle(0x67)
	if err != nil {
		fmt.Println("broken at openhandle")
		return err
	}
	defer func() {
		if err := handle.Close(); err != nil {
			m.logger.Error(err)
		}
	}()

	return handle.Write(ctx, command)
}

// SetPower sets the percentage of power the motor should employ between -1 and 1.
// Negative power implies a backward directional rotational
func (m *Motor) SetPower(ctx context.Context, powerPct float64) error {
	// var base = 16
	// var size = 16
	// command := uint16(0)
	command := []byte{}
	if powerPct == 0 {
		fmt.Println("power to 0")
		// value, _ := strconv.ParseUint(find, base, size)
		// command = uint16(value) // done!
		command = []byte(dispenseTilStop)
	} else {
		fmt.Println("power to something else")
		// value, _ := strconv.ParseUint("dfgdhshgs", base, size)
		// command = uint16(value) // done!
		command = []byte(stop)
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

func (m *Motor) GetPosition(ctx context.Context) (float64, error) {
	return 0, nil
}

func (m *Motor) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
	return nil, nil
}

func (m *Motor) Stop(ctx context.Context) error {
	command := []byte(stop)
	fmt.Println(command)
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return errors.New(writeErr.Error())
	}
	return nil
}

func (m *Motor) IsPowered(ctx context.Context) (bool, error) {
	return false, nil
}

// allows you to set and change the I2C address of the device
func (m *Motor) SetAddress() {

}

func (m *Motor) ControlLED(command string) error {
	return nil
}
