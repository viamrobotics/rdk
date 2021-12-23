package vforcematrixtraditional

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/sensor/forcematrix"
	"go.viam.com/rdk/slipdetection"
)

// ModelName is used to register the sensor to a model name.
const ModelName = "forcematrixtraditional_v1"

// init registers the forcematrix sensor type
func init() {
	registry.RegisterSensor(forcematrix.Type, ModelName, registry.Sensor{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
			return New(ctx, r, config, logger)
		}})
}

// ForceMatrixTraditional represents a force matrix without a mux.
type ForceMatrixTraditional struct {
	columnGpioPins      []string
	analogChannels      []string
	analogReaders       []board.AnalogReader
	board               board.Board
	previousMatrices    [][][]int // a window of previous matrix readings
	mu                  sync.Mutex
	slipDetectionWindow int // how far back in the window of previous readings to look
	// for slip detection
	noiseThreshold float64 // sensitivity threshold for determining noise
}

// New returns a new ForceMatrixTraditional given gpio pins and analog channels.
func New(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (*ForceMatrixTraditional, error) {
	boardName := config.Attributes.String("board")
	b, exists := r.BoardByName(boardName)
	if !exists {
		return nil, errors.Errorf("need a board for force sensor, named (%v)", boardName)
	}

	columnGpioPins := config.Attributes.StringSlice("column_gpio_pins_left_to_right")
	analogChannels := config.Attributes.StringSlice("row_analog_channels_top_to_bottom")

	analogReaders := make([]board.AnalogReader, 0, len(analogChannels))
	for _, readerName := range analogChannels {
		reader, exists := b.AnalogReaderByName(readerName)
		if !exists {
			return nil, errors.Errorf("expected to find analog reader called %q", readerName)
		}
		analogReaders = append(analogReaders, reader)
	}
	noiseThreshold := config.Attributes.Float64("slip_detection_signal_to_noise_cutoff", 0)
	slipDetectionWindow := config.Attributes.Int("slip_detection_window", forcematrix.MatrixStorageSize)
	if slipDetectionWindow > forcematrix.MatrixStorageSize {
		return nil, errors.Errorf("slip_detection_window has to be <= %v", forcematrix.MatrixStorageSize)
	}
	previousMatrices := make([][][]int, 0)

	return &ForceMatrixTraditional{
		columnGpioPins:      columnGpioPins,
		analogChannels:      analogChannels,
		analogReaders:       analogReaders,
		board:               b,
		previousMatrices:    previousMatrices,
		slipDetectionWindow: slipDetectionWindow,
		noiseThreshold:      noiseThreshold,
	}, nil
}

// addToPreviousMatricesWindow adds a matrix reading to the readings history queue
func (fsm *ForceMatrixTraditional) addToPreviousMatricesWindow(matrix [][]int) {
	if len(fsm.previousMatrices) > forcematrix.MatrixStorageSize {
		fsm.previousMatrices = fsm.previousMatrices[1:]
	}
	fsm.previousMatrices = append(fsm.previousMatrices, matrix)
}

// Matrix returns a matrix of measurements from the force sensor.
func (fsm *ForceMatrixTraditional) Matrix(ctx context.Context) ([][]int, error) {
	numRows := len(fsm.analogReaders)
	numCols := len(fsm.columnGpioPins)

	matrix := make([][]int, numRows)
	for row := range matrix {
		matrix[row] = make([]int, numCols)
	}
	for col := 0; col < len(fsm.columnGpioPins); col++ {
		// set the correct GPIO to high
		if err := fsm.board.GPIOSet(ctx, fsm.columnGpioPins[col], true); err != nil {
			return nil, err
		}

		// set all other GPIO pins to low
		for c, pin := range fsm.columnGpioPins {
			if c != col {
				err := fsm.board.GPIOSet(ctx, pin, false)
				if err != nil {
					return nil, err
				}
			}
		}

		// read out the pressure values
		for row, analogReader := range fsm.analogReaders {
			val, err := analogReader.Read(ctx)
			if err != nil {
				return nil, err
			}
			matrix[row][col] = val
		}
	}
	fsm.addToPreviousMatricesWindow(matrix)
	return matrix, nil
}

// Readings returns a flattened matrix of measurements from the force sensor.
func (fsm *ForceMatrixTraditional) Readings(ctx context.Context) ([]interface{}, error) {
	matrix, err := fsm.Matrix(ctx)
	if err != nil {
		return nil, err
	}

	numRows := len(fsm.analogReaders)
	numCols := len(fsm.columnGpioPins)

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
func (fsm *ForceMatrixTraditional) GetPreviousMatrices() [][][]int {
	return fsm.previousMatrices
}

// IsSlipping is used to determine whether the object in contact
// with the sensor matrix is slipping
func (fsm *ForceMatrixTraditional) IsSlipping(ctx context.Context) (bool, error) {
	return slipdetection.DetectSlip(fsm, &(fsm.mu), 0, fsm.noiseThreshold, fsm.slipDetectionWindow)

}

// Desc returns that this is a forcematrix sensor type.
func (fsm *ForceMatrixTraditional) Desc() sensor.Description {
	return sensor.Description{forcematrix.Type, ""}
}
