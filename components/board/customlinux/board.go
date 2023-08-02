//go:build linux

// Package customlinux implements a board running Linux.
// This is an Experimental package
package customlinux

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"periph.io/x/host/v3"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
	"go.viam.com/rdk/resource"
)

const modelName = "customlinux"

func init() {
	if _, err := host.Init(); err != nil {
		golog.Global().Debugw("error initializing host", "error", err)
	}

	resource.RegisterComponent(
		board.API,
		resource.DefaultModelFamily.WithModel(modelName),
		resource.Registration[board.Board, *Config]{
			Constructor: createNewBoard,
		})
}

// customLinuxBoard wraps the genericlinux board type so that both can implement their own Reconfigure function.
type customLinuxBoard struct {
	*genericlinux.SysfsBoard
	logger golog.Logger
}

func createNewBoard(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (board.Board, error) {
	return genericlinux.newBoard(ctx, conf, pinDefsFromFile, logger)
}

// This is a ConfigConverter which loads pin definitions from a file, assuming that the config
// passed in is a customlinux.Config underneath.
func pinDefsFromFile(conf resource.Config) (*genericlinux.UnderlyingConfig, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	pinDefs, err := parsePinConfig(newConf.PinConfigFilePath)
	if err != nil {
		return nil, err
	}

	gpioMappings, err := genericlinux.GetGPIOBoardMappingFromPinDefs(pinDefs)
	if err != nil {
		return nil, err
	}

	return &genericlinux.UnderlyingConfig{
		I2Cs:              newConf.I2Cs,
		SPIs:              newConf.SPIs,
		Analogs:           newConf.Analogs,
		DigitalInterrupts: newConf.DigitalInterrupts,
		GpioMappings:      gpioMappings,
	}, nil
}

func parsePinConfig(filePath string) ([]genericlinux.PinDefinition, error) {
	pinData, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, err
	}

	return parseRawPinData(pinData, filePath)
}

// filePath passed in for logging purposes.
func parseRawPinData(pinData []byte, filePath string) ([]genericlinux.PinDefinition, error) {
	var parsedPinData genericlinux.PinDefinitions
	if err := json.Unmarshal(pinData, &parsedPinData); err != nil {
		return nil, err
	}

	var err error
	for _, pin := range parsedPinData.Pins {
		err = multierr.Combine(err, pin.Validate(filePath))
	}
	if err != nil {
		return nil, err
	}
	return parsedPinData.Pins, nil
}

func createGenericLinuxConfig(conf *Config) genericlinux.Config {
	return genericlinux.Config{
		I2Cs:              conf.I2Cs,
		SPIs:              conf.SPIs,
		Analogs:           conf.Analogs,
		DigitalInterrupts: conf.DigitalInterrupts,
	}
}

// Reconfigure reconfigures the board with interrupt pins, spi and i2c, and analogs.
// WARNING: does not update pin definitions when the config file changes.
// TODO[RSDK-4092]: implement reconfiguration when pin definitions change.
func (b *customLinuxBoard) Reconfigure(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	boardConfig := createGenericLinuxConfig(newConf)

	return b.ReconfigureParsedConfig(ctx, &boardConfig)
}
