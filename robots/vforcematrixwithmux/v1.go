package vforcematrixwithmux

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	"go.viam.com/core/component/board"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/forcematrix"
	"go.viam.com/core/slipdetection"
)

// ModelName is used to register the sensor to a model name
const ModelName = "forcematrixwithmux_v1"

// init registers the forcematrix mux sensor type.
func init() {
	registry.RegisterSensor(forcematrix.Type, ModelName, registry.Sensor{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
			return New(ctx, r, config, logger)
		}})
}

// ForceMatrixWithMux represents a force matrix that's wired up with a mux.
type ForceMatrixWithMux struct {
	columnGpioPins      []string
	muxGpioPins         []string  // which GPIO pins are S2, S1, S0 connected to?
	ioPins              []int     // integers that indicate which Y pin we're connected to (Y0-Y7)
	analogChannel       string    // analog channel that the mux is connected to
	previousMatrices    [][][]int // a window of previous matrix readings
	mu                  sync.Mutex
	slipDetectionWindow int // how far back in the window of previous readings to look
	// for slip detection
	noiseThreshold float64 // sensitivity threshold for determining noise

	analogReader board.AnalogReader
	board        board.Board
	logger       golog.Logger
}

// New returns a new ForceMatrixWithMux given column gpio pins, mux gpio pins, io pins, and
// an analog channel.
func New(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (*ForceMatrixWithMux, error) {
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
	noiseThreshold := config.Attributes.Float64("slip_detection_signal_to_noise_cutoff", 0)
	slipDetectionWindow := config.Attributes.Int("slip_detection_window", forcematrix.MatrixStorageSize)
	if slipDetectionWindow > forcematrix.MatrixStorageSize {
		return nil, errors.Errorf("slip_detection_window has to be <= %v", forcematrix.MatrixStorageSize)
	}
	previousMatrices := make([][][]int, 0)

	if exists {
		return &ForceMatrixWithMux{
			columnGpioPins:      columnGpioPins,
			muxGpioPins:         muxGpioPins,
			ioPins:              ioPins,
			analogChannel:       analogChannel,
			analogReader:        reader,
			board:               b,
			previousMatrices:    previousMatrices,
			logger:              logger,
			slipDetectionWindow: slipDetectionWindow,
			noiseThreshold:      noiseThreshold,
		}, nil
	}

	return nil, errors.Errorf("expected to find analog reader called %q", reader)
}

// setMuxGpioPins sets the gpio pins that control the mux based on its given logic table.
func (fmsm *ForceMatrixWithMux) setMuxGpioPins(ctx context.Context, ioPin int) error {
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

// addToPreviousMatricesWindow adds a matrix reading to the readings history queue
func (fmsm *ForceMatrixWithMux) addToPreviousMatricesWindow(matrix [][]int) {
	if len(fmsm.previousMatrices) > forcematrix.MatrixStorageSize {
		fmsm.previousMatrices = fmsm.previousMatrices[1:]
	}
	fmsm.previousMatrices = append(fmsm.previousMatrices, matrix)
}

// Matrix returns a matrix of measurements from the force sensor.
func (fmsm *ForceMatrixWithMux) Matrix(ctx context.Context) ([][]int, error) {
	numRows := len(fmsm.ioPins)
	numCols := len(fmsm.columnGpioPins)

	matrix := make([][]int, numRows)
	for row := range matrix {
		matrix[row] = make([]int, numCols)
	}

	for col := 0; col < numCols; col++ {
		// set the correct GPIO to high
		if err := fmsm.board.GPIOSet(ctx, fmsm.columnGpioPins[col], true); err != nil {
			return nil, err
		}

		// set all other GPIO pins to low
		for c, pin := range fmsm.columnGpioPins {
			if c != col {
				err := fmsm.board.GPIOSet(ctx, pin, false)
				if err != nil {
					return nil, err
				}
			}
		}

		// read out the pressure values
		for row, ioPin := range fmsm.ioPins {
			if err := fmsm.setMuxGpioPins(ctx, ioPin); err != nil {
				return nil, err
			}
			val, err := fmsm.analogReader.Read(ctx)
			if err != nil {
				return nil, err
			}
			matrix[row][col] = val

		}
	}
	fmsm.addToPreviousMatricesWindow(matrix)
	return matrix, nil
}

// Readings returns a flattened matrix of measurements from the force sensor.
func (fmsm *ForceMatrixWithMux) Readings(ctx context.Context) ([]interface{}, error) {
	matrix, err := fmsm.Matrix(ctx)
	if err != nil {
		return nil, err
	}

	numRows := len(fmsm.ioPins)
	numCols := len(fmsm.columnGpioPins)

	readings := make([]interface{}, 0, numRows*numCols)
	for row := 0; row < numRows; row++ {
		for col := 0; col < numCols; col++ {
			readings = append(readings, matrix[row][col])
		}
	}
	return readings, nil
}

// GetPreviousMatrices is an accessor for the history of matrix readings stored
// on the sensor required for slip detection (see slipdetector.ReadingsHistoryProvider)
func (fmsm *ForceMatrixWithMux) GetPreviousMatrices() [][][]int {
	return fmsm.previousMatrices
}

// IsSlipping is used to determine whether the object in contact
// with the sensor matrix is slipping
func (fmsm *ForceMatrixWithMux) IsSlipping(ctx context.Context) (bool, error) {
	return slipdetection.DetectSlip(fmsm, &(fmsm.mu), 0, fmsm.noiseThreshold, fmsm.slipDetectionWindow)

}

// Desc returns that this is a forcematrix mux sensor type.
func (fmsm *ForceMatrixWithMux) Desc() sensor.Description {
	return sensor.Description{forcematrix.Type, ""}
}
