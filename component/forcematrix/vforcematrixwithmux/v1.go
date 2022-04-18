// Package vforcematrixwithmux implements the Viam Force Matrix with Multiplexer.
package vforcematrixwithmux

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
const model = "forcematrixwithmux_v1"

// ForceMatrixConfig describes the configuration of a forcematrixwithmux_v1.
type ForceMatrixConfig struct {
	BoardName           string   `json:"board"` // used to control gpio pins & read out pressure values
	ColumnGPIOPins      []string `json:"column_gpio_pins_left_to_right"`
	MuxGPIOPins         []string `json:"mux_gpio_pins_s2_to_s0"`
	IOPins              []int    `json:"io_pins_top_to_bottom"`
	AnalogChannel       string   `json:"analog_channel"`
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
	if len(config.MuxGPIOPins) != 3 {
		return utils.NewConfigValidationError(path, errors.New("mux_gpio_pins_s2_to_s0 has to be an array of length 3"))
	}
	if len(config.IOPins) == 0 {
		return utils.NewConfigValidationError(path, errors.New("io_pins_top_to_bottom has to be an array of length > 0"))
	}
	if config.AnalogChannel == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "analog_channel")
	}
	if config.SlipDetectionWindow == 0 || config.SlipDetectionWindow > forcematrix.MatrixStorageSize {
		return utils.NewConfigValidationError(path,
			errors.Errorf("slip_detection_window has to be: 0 < slip_detection_window <= %v",
				forcematrix.MatrixStorageSize))
	}
	return nil
}

// init registers the forcematrix mux sensor type.
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
			return newForceMatrix(r, forceMatrixConfig, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(forcematrix.SubtypeName,
		model, func(attributes config.AttributeMap) (interface{}, error) {
			var conf ForceMatrixConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &ForceMatrixConfig{})
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

// newForceMatrix returns a new ForceMatrixWithMux given column gpio pins, mux gpio pins, io pins, and
// an analog channel.
func newForceMatrix(r robot.Robot, c *ForceMatrixConfig, logger golog.Logger) (*ForceMatrixWithMux, error) {
	b, err := board.FromRobot(r, c.BoardName)
	if err != nil {
		return nil, err
	}

	reader, exists := b.AnalogReaderByName(c.AnalogChannel)

	if exists {
		return &ForceMatrixWithMux{
			columnGpioPins:      c.ColumnGPIOPins,
			muxGpioPins:         c.MuxGPIOPins,
			ioPins:              c.IOPins,
			analogChannel:       c.AnalogChannel,
			analogReader:        reader,
			board:               b,
			previousMatrices:    make([][][]int, 0, forcematrix.MatrixStorageSize),
			logger:              logger,
			slipDetectionWindow: c.SlipDetectionWindow,
			noiseThreshold:      c.NoiseThreshold,
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
		p, err := fmsm.board.GPIOPinByName(muxGpioPin)
		if err != nil {
			return err
		}
		if err := p.Set(ctx, logicTable[i]); err != nil {
			return err
		}
	}

	return nil
}

// addToPreviousMatricesWindow adds a matrix reading to the readings history queue.
func (fmsm *ForceMatrixWithMux) addToPreviousMatricesWindow(matrix [][]int) {
	if len(fmsm.previousMatrices) > forcematrix.MatrixStorageSize {
		fmsm.previousMatrices = fmsm.previousMatrices[1:]
	}
	fmsm.previousMatrices = append(fmsm.previousMatrices, matrix)
}

// ReadMatrix returns a matrix of measurements from the force sensor.
func (fmsm *ForceMatrixWithMux) ReadMatrix(ctx context.Context) ([][]int, error) {
	numRows := len(fmsm.ioPins)
	numCols := len(fmsm.columnGpioPins)

	matrix := make([][]int, numRows)
	for row := range matrix {
		matrix[row] = make([]int, numCols)
	}

	for col := 0; col < numCols; col++ {
		// set the correct GPIO to high
		p, err := fmsm.board.GPIOPinByName(fmsm.columnGpioPins[col])
		if err != nil {
			return nil, err
		}
		if err := p.Set(ctx, true); err != nil {
			return nil, err
		}

		// set all other GPIO pins to low
		for c, pin := range fmsm.columnGpioPins {
			if c != col {
				p, err := fmsm.board.GPIOPinByName(pin)
				if err != nil {
					return nil, err
				}
				if err := p.Set(ctx, false); err != nil {
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

// GetPreviousMatrices is an accessor for the history of matrix readings stored
// on the sensor required for slip detection (see slipdetector.ReadingsHistoryProvider).
func (fmsm *ForceMatrixWithMux) GetPreviousMatrices() [][][]int {
	return fmsm.previousMatrices
}

// DetectSlip is used to determine whether the object in contact
// with the sensor matrix is slipping.
func (fmsm *ForceMatrixWithMux) DetectSlip(ctx context.Context) (bool, error) {
	return slipdetection.DetectSlip(fmsm, &(fmsm.mu), 0, fmsm.noiseThreshold, fmsm.slipDetectionWindow)
}
