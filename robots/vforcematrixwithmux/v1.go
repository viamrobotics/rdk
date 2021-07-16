package vforcematrixwithmux

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/forcematrix"
)

// ModelName is used to register the sensor to a model name
const ModelName = "forcematrixwithmux_v1"

// init registers the forcematrix mux sensor type.
func init() {
	registry.RegisterSensor(forcematrix.Type, ModelName, registry.Sensor{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
			return NewMux(ctx, r, config, logger)
		}})
}

// ForceSensorMatrixWithMux represents a force sensor matrix that's wired up with a mux.
type ForceSensorMatrixWithMux struct {
	columnGpioPins []string
	muxGpioPins    []string // which GPIO pins are S2, S1, S0 connected to?
	ioPins         []int    // integers that indicate which Y pin we're connected to (Y0-Y7)
	analogChannel  string   // analog channel that the mux is connected to

	analogReader board.AnalogReader
	board        board.Board
}

// NewMux returns a new ForceSensorMatrixWithMux given column gpio pins, mux gpio pins, io pins, and
// an analog channel.
func NewMux(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (*ForceSensorMatrixWithMux, error) {
	boardName := config.Attributes.String("board")
	b, exists := r.BoardByName(boardName)
	if !exists {
		return nil, errors.Errorf("need a board for force sensor, named (%v)", boardName)
	}

	columnGpioPins := config.Attributes.StringSlice("column_gpio_pins_left_to_right")
	muxGpioPins := config.Attributes.StringSlice("mux_gpio_pins_s2_to_s0")
	ioPins := config.Attributes.IntSlice("io_pins_top_to_bottom")
	analogChannel := config.Attributes.String("analog_channel")
	reader, exists := b.AnalogReaderByName(analogChannel)

	if exists {
		return &ForceSensorMatrixWithMux{
			columnGpioPins: columnGpioPins,
			muxGpioPins:    muxGpioPins,
			ioPins:         ioPins,
			analogChannel:  analogChannel,
			analogReader:   reader,
			board:          b,
		}, nil
	}
	return nil, errors.Errorf("expected to find analog reader called %q", reader)
}

// setMuxGpioPins sets the gpio pins that control the mux based on its given logic table.
func (fmsm *ForceSensorMatrixWithMux) setMuxGpioPins(ctx context.Context, ioPin int) error {
	// The logicTable corresponds to select pins in the order of
	// [s2, s1, s0]
	var logicTable [3]bool
	switch ioPin {
	case 0:
		logicTable = [3]bool{false, false, false}
	case 1:
		logicTable = [3]bool{false, false, true}
	case 2:
		logicTable = [3]bool{false, true, false}
	case 3:
		logicTable = [3]bool{false, true, true}
	case 4:
		logicTable = [3]bool{true, false, false}
	case 5:
		logicTable = [3]bool{true, false, true}
	case 6:
		logicTable = [3]bool{true, true, false}
	case 7:
		logicTable = [3]bool{true, true, true}
	default:
		return errors.Errorf("wrong pin number: (%v), needs to be between 0 - 7", ioPin)
	}

	for i, muxGpioPin := range fmsm.muxGpioPins {
		if err := fmsm.board.GPIOSet(ctx, muxGpioPin, logicTable[i]); err != nil {
			return err
		}
	}

	return nil
}

// Matrix returns a matrix of measurements from the force sensor.
func (fmsm *ForceSensorMatrixWithMux) Matrix(ctx context.Context) ([][]int, error) {
	matrix := make([][]int, len(fmsm.columnGpioPins))
	for i := 0; i < len(fmsm.columnGpioPins); i++ {
		if err := fmsm.board.GPIOSet(ctx, fmsm.columnGpioPins[i], true); err != nil {
			return nil, err
		}

		for j, pin := range fmsm.columnGpioPins {
			if i != j {
				err := fmsm.board.GPIOSet(ctx, pin, false)
				if err != nil {
					return nil, err
				}
			}
		}

		for _, ioPin := range fmsm.ioPins {
			if err := fmsm.setMuxGpioPins(ctx, ioPin); err != nil {
				return nil, err
			}
			val, err := fmsm.analogReader.Read(ctx)
			if err != nil {
				return nil, err
			}
			matrix[i] = append(matrix[i], val)

		}
	}

	return matrix, nil
}

// Readings returns a flattened matrix of measurements from the force sensor.
func (fmsm *ForceSensorMatrixWithMux) Readings(ctx context.Context) ([]interface{}, error) {
	matrix, err := fmsm.Matrix(ctx)
	if err != nil {
		return nil, err
	}
	readings := make([]interface{}, 0, len(fmsm.columnGpioPins)*len(fmsm.ioPins))
	for i := 0; i < len(fmsm.columnGpioPins); i++ {
		for j := 0; j < len(fmsm.ioPins); j++ {
			readings = append(readings, matrix[i][j])
		}
	}
	return readings, nil
}

// Desc returns that this is a forcematrix mux sensor type.
func (fmsm *ForceSensorMatrixWithMux) Desc() sensor.Description {
	return sensor.Description{forcematrix.Type, ""}
}
