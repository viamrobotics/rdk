package vforcematrixtraditional

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/forcematrix"
	"go.viam.com/core/slipdetection"
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
	gpioPins                []string
	analogChannels          []string
	analogReaders           []board.AnalogReader
	board                   board.Board
	previousMatrices        [][][]int // a window of previous matrix readings
	mu                      sync.Mutex
	slipDetectionResolution int // how far back in the window of previous readings to look
	// for slip detection
}

// New returns a new ForceMatrixTraditional given gpio pins and analog channels.
func New(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (*ForceMatrixTraditional, error) {
	boardName := config.Attributes.String("board")
	b, exists := r.BoardByName(boardName)
	if !exists {
		return nil, errors.Errorf("need a board for force sensor, named (%v)", boardName)
	}

	gpioPins := config.Attributes.StringSlice("column_gpio_pins_left_to_right")
	analogChannels := config.Attributes.StringSlice("row_analog_channels_top_to_bottom")

	analogReaders := make([]board.AnalogReader, 0, len(analogChannels))
	for _, readerName := range analogChannels {
		reader, exists := b.AnalogReaderByName(readerName)
		if !exists {
			return nil, errors.Errorf("expected to find analog reader called %q", readerName)
		}
		analogReaders = append(analogReaders, reader)
	}
	slipDetectionResolution := config.Attributes.Int("slip_detection_resolution", forcematrix.MatrixStorageSize)
	previousMatrices := make([][][]int, 0)

	return &ForceMatrixTraditional{
		gpioPins:                gpioPins,
		analogChannels:          analogChannels,
		analogReaders:           analogReaders,
		board:                   b,
		previousMatrices:        previousMatrices,
		slipDetectionResolution: slipDetectionResolution,
	}, nil
}

// addToPreviousMatrixQueue adds a matrix reading to the readings history queue
func (fsm *ForceMatrixTraditional) addToPreviousMatrixQueue(matrix [][]int) {
	if len(fsm.previousMatrices) > forcematrix.MatrixStorageSize {
		fsm.previousMatrices = fsm.previousMatrices[1:]
	}
	fsm.previousMatrices = append(fsm.previousMatrices, matrix)
}

// Matrix returns a matrix of measurements from the force sensor.
func (fsm *ForceMatrixTraditional) Matrix(ctx context.Context) ([][]int, error) {
	matrix := make([][]int, len(fsm.gpioPins))
	for i := 0; i < len(fsm.gpioPins); i++ {
		if err := fsm.board.GPIOSet(ctx, fsm.gpioPins[i], true); err != nil {
			return nil, err
		}

		for j, pin := range fsm.gpioPins {
			if i != j {
				err := fsm.board.GPIOSet(ctx, pin, false)
				if err != nil {
					return nil, err
				}
			}
		}

		for _, analogReader := range fsm.analogReaders {
			val, err := analogReader.Read(ctx)
			if err != nil {
				return nil, err
			}
			matrix[i] = append(matrix[i], val)
		}
	}
	fsm.addToPreviousMatrixQueue(matrix)
	return matrix, nil
}

// Readings returns a flattened matrix of measurements from the force sensor.
func (fsm *ForceMatrixTraditional) Readings(ctx context.Context) ([]interface{}, error) {
	matrix, err := fsm.Matrix(ctx)
	if err != nil {
		return nil, err
	}
	readings := make([]interface{}, 0, len(fsm.analogChannels)*len(fsm.analogReaders))
	for i := 0; i < len(fsm.analogChannels); i++ {
		for j := 0; j < len(fsm.analogReaders); j++ {
			readings = append(readings, matrix[i][j])
		}
	}
	return readings, nil
}

// GetPreviousMatrices is an accessor for the history of matrix readings stored
// on the sensor required for slip detection (see slipdetector.SlipDetector)
func (fsm *ForceMatrixTraditional) GetPreviousMatrices() [][][]int {
	return fsm.previousMatrices
}

// IsSlipping examines is used to determine whether the object in contact
// with the sensor matrix is slipping
func (fsm *ForceMatrixTraditional) IsSlipping(ctx context.Context) (bool, error) {
	return slipdetection.DetectSlip(fsm, &fsm.mu, 0, fsm.slipDetectionResolution)

}

// Desc returns that this is a forcematrix sensor type.
func (fsm *ForceMatrixTraditional) Desc() sensor.Description {
	return sensor.Description{forcematrix.Type, ""}
}
