//go:build linux

// Package customlinux implements a board running Linux.
// This is an Experimental package
package customlinux

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"go.uber.org/multierr"
	"periph.io/x/host/v3"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

const modelName = "customlinux"

func init() {
	if _, err := host.Init(); err != nil {
		logging.Global().Debugw("error initializing host", "error", err)
	}

	resource.RegisterComponent(
		board.API,
		resource.DefaultModelFamily.WithModel(modelName),
		resource.Registration[board.Board, *Config]{
			Constructor: createNewBoard,
		})
}

func createNewBoard(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (board.Board, error) {
	return genericlinux.NewBoard(ctx, conf, pinDefsFromFile, logger)
}

// This is a ConfigConverter which loads pin definitions from a file, assuming that the config
// passed in is a customlinux.Config underneath.
func pinDefsFromFile(conf resource.Config) (*genericlinux.LinuxBoardConfig, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	pinDefs, err := parsePinConfig(newConf.BoardDefsFilePath)
	if err != nil {
		return nil, err
	}

	gpioMappings, err := genericlinux.GetGPIOBoardMappingFromPinDefs(pinDefs)
	if err != nil {
		return nil, err
	}

	return &genericlinux.LinuxBoardConfig{
		I2Cs:         newConf.I2Cs,
		SPIs:         newConf.SPIs,
		GpioMappings: gpioMappings,
	}, nil
}

func parsePinConfig(filePath string) ([]genericlinux.PinDefinition, error) {
	pinData, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, err
	}

	return parseRawPinData(pinData, filePath)
}

// This function is separate from parsePinConfig to make it testable without interacting with the
// file system. The filePath is passed in just for logging purposes.
func parseRawPinData(pinData []byte, filePath string) ([]genericlinux.PinDefinition, error) {
	var parsedPinData genericlinux.PinDefinitions
	if err := json.Unmarshal(pinData, &parsedPinData); err != nil {
		return nil, err
	}

	var err error
	for name, pin := range parsedPinData.Pins {
		err = multierr.Combine(err, pin.Validate(filePath))

		// Until we get hardware PWM working reliably on lots of boards (likely by being able to
		// set the pin mode directly), we disable all hardware PWM on these boards.
		// The range operator makes a copy of the value, so to change the original, we need to look
		// it up in the map again.
		// TODO(RSDK-5105): Undo this when we're ready to support hardware PWM.
		parsedPinData.Pins[name].PwmChipSysfsDir = ""
		parsedPinData.Pins[name].PwmID = -1
	}
	if err != nil {
		return nil, err
	}
	return parsedPinData.Pins, nil
}
