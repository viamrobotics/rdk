// Package vforcematrixtraditional implements the Viam Force Matrix.
package vforcematrixtraditional

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/forcematrix"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/slipdetection"
	rdkutils "go.viam.com/rdk/utils"
)

// model is used to register the sensor to a model name.
const model = "forcematrixtraditional_v1"

// ForceMatrixConfig describes the configuration of a forcematrixtraditional_v1.
type ForceMatrixConfig struct {
	BoardName           string   `json:"board"` // used to control gpio pins & read out pressure values
	ColumnGPIOPins      []string `json:"column_gpio_pins_left_to_right"`
	RowAnalogChannels   []string `json:"row_analog_channels_top_to_bottom"`
	SlipDetectionWindow int      `json:"slip_detection_window"`
	NoiseThreshold      float64  `json:"slip_detection_signal_to_noise_cutoff"`
}

// Validate ensures all parts of the config are valid.
func (config *ForceMatrixConfig) Validate(path string) error {
	if config.BoardName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "board")
	}
	if len(config.ColumnGPIOPins) == 0 {
		return utils.NewConfigValidationError(path, errors.New("column_gpio_pins_left_to_right has to be an array of length > 0"))
	}
	if len(config.RowAnalogChannels) == 0 {
		return utils.NewConfigValidationError(path, errors.New("row_analog_channels_top_to_bottom has to be an array of length > 0"))
	}
	if config.SlipDetectionWindow == 0 || config.SlipDetectionWindow > forcematrix.MatrixStorageSize {
		return utils.NewConfigValidationError(path,
			errors.Errorf("slip_detection_window has to be: 0 < slip_detection_window <= %v",
				forcematrix.MatrixStorageSize))
	}
	return nil
}

// init registers the forcematrix sensor type.
func init() {
	registry.RegisterComponent(forcematrix.Subtype, model, registry.Component{
		Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			forceMatrixConfig, ok := config.ConvertedAttributes.(*ForceMatrixConfig)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(forceMatrixConfig, config.ConvertedAttributes)
			}
			return newForceMatrix(r, forceMatrixConfig)
		},
	})

	config.RegisterComponentAttributeMapConverter(forcematrix.SubtypeName,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf ForceMatrixConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&ForceMatrixConfig{})
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

// newForceMatrix returns a new ForceMatrixTraditional given gpio pins and analog channels.
func newForceMatrix(r robot.Robot, c *ForceMatrixConfig) (*ForceMatrixTraditional, error) {
	b, err := board.FromRobot(r, c.BoardName)
	if err != nil {
		return nil, err
	}

	analogReaders := make([]board.AnalogReader, 0, len(c.RowAnalogChannels))
	for _, readerName := range c.RowAnalogChannels {
		reader, exists := b.AnalogReaderByName(readerName)
		if !exists {
			return nil, errors.Errorf("expected to find analog reader called %q", readerName)
		}
		analogReaders = append(analogReaders, reader)
	}

	return &ForceMatrixTraditional{
		columnGpioPins:      c.ColumnGPIOPins,
		analogChannels:      c.RowAnalogChannels,
		analogReaders:       analogReaders,
		board:               b,
		previousMatrices:    make([][][]int, 0, forcematrix.MatrixStorageSize),
		slipDetectionWindow: c.SlipDetectionWindow,
		noiseThreshold:      c.NoiseThreshold,
	}, nil
}

// addToPreviousMatricesWindow adds a matrix reading to the readings history queue.
func (fsm *ForceMatrixTraditional) addToPreviousMatricesWindow(matrix [][]int) {
	if len(fsm.previousMatrices) > forcematrix.MatrixStorageSize {
		fsm.previousMatrices = fsm.previousMatrices[1:]
	}
	fsm.previousMatrices = append(fsm.previousMatrices, matrix)
}

// ReadMatrix returns a matrix of measurements from the force sensor.
func (fsm *ForceMatrixTraditional) ReadMatrix(ctx context.Context) ([][]int, error) {
	numRows := len(fsm.analogReaders)
	numCols := len(fsm.columnGpioPins)

	matrix := make([][]int, numRows)
	for row := range matrix {
		matrix[row] = make([]int, numCols)
	}
	for col := 0; col < len(fsm.columnGpioPins); col++ {
		// set the correct GPIO to high
		p, err := fsm.board.GPIOPinByName(fsm.columnGpioPins[col])
		if err != nil {
			return nil, err
		}
		if err := p.Set(ctx, true); err != nil {
			return nil, err
		}

		// set all other GPIO pins to low
		for c, pin := range fsm.columnGpioPins {
			if c != col {
				p, err := fsm.board.GPIOPinByName(pin)
				if err != nil {
					return nil, err
				}
				if err := p.Set(ctx, false); err != nil {
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

// GetPreviousMatrices is an accessor for the history of matrix readings stored
// on the sensor required for slip detection (see slipdetector.ReadingsHistoryProvider).
func (fsm *ForceMatrixTraditional) GetPreviousMatrices() [][][]int {
	return fsm.previousMatrices
}

// DetectSlip is used to determine whether the object in contact
// with the sensor matrix is slipping.
func (fsm *ForceMatrixTraditional) DetectSlip(ctx context.Context) (bool, error) {
	return slipdetection.DetectSlip(fsm, &(fsm.mu), 0, fsm.noiseThreshold, fsm.slipDetectionWindow)
}
