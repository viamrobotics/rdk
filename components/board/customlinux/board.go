//go:build linux

// Package customlinux implements a board running linux
// This is an Experimental package
package customlinux

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"periph.io/x/host/v3"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
	"go.viam.com/rdk/resource"
)

func init() {
	if _, err := host.Init(); err != nil {
		golog.Global().Debugw("error initializing host", "error", err)
	}

	RegisterCustomBoard("customlinux")
}

// customLinuxBoard wraps the genericlinux board type so that both can implement their own Reconfigure function.
type customLinuxBoard struct {
	*genericlinux.SysfsBoard
}

// RegisterCustomBoard registers a sysfs based board using the pin mappings.
func RegisterCustomBoard(modelName string) {
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
	logger golog.Logger,
) (board.Board, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	pinDefs, err := parseBoardConfig(newConf.PinConfigFilePath)
	if err != nil {
		return nil, err
	}

	gpioMappings, err := genericlinux.GetGPIOBoardMappingFromPinDefs(pinDefs)
	if err != nil {
		return nil, err
	}

	boardConfig := genericlinux.Config{
		I2Cs:              newConf.I2Cs,
		SPIs:              newConf.SPIs,
		Analogs:           newConf.Analogs,
		DigitalInterrupts: newConf.DigitalInterrupts,
	}
	b, err := genericlinux.NewBoard(ctx, conf.ResourceName().AsNamed(), &boardConfig, gpioMappings, logger)
	if err != nil {
		return nil, err
	}

	gb, ok := b.(*genericlinux.SysfsBoard)
	if !ok {
		return nil, errors.New("error creating board object")
	}
	return &customLinuxBoard{gb}, nil
}

func parseBoardConfig(filePath string) ([]genericlinux.PinDefinition, error) {
	pinData, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, err
	}
	var parsedPinData GenericLinuxPins
	if err := json.Unmarshal(pinData, &parsedPinData); err != nil {
		return nil, err
	}

	pinDefs := make([]genericlinux.PinDefinition, len(parsedPinData.Pins))
	for i, pin := range parsedPinData.Pins {
		err = pin.Validate(filePath)
		if err != nil {
			return nil, err
		}

		pinName, err := strconv.Atoi(pin.Name)
		if err != nil {
			return nil, err
		}

		pinDefs[i] = genericlinux.PinDefinition{
			GPIOChipRelativeIDs: map[int]int{pin.Ngpio: pin.LineNumber}, // ngpio: relative id map
			PinNumberBoard:      pinName,
			PWMChipSysFSDir:     pin.PwmChipSysfsDir,
			PWMID:               pin.PwmID,
		}
	}

	return pinDefs, nil
}

// Reconfigure reconfigures the board with interrupt pins, spi and i2c, and analogs.
func (b *customLinuxBoard) Reconfigure(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	boardConfig := genericlinux.Config{
		I2Cs:              newConf.I2Cs,
		SPIs:              newConf.SPIs,
		Analogs:           newConf.Analogs,
		DigitalInterrupts: newConf.DigitalInterrupts,
	}
	return b.ReconfigureParsedConfig(ctx, &boardConfig)
}
