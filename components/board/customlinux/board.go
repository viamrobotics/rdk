//go:build linux

// Package customlinux implements a board running Linux.
// This is an Experimental package
package customlinux

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
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
	mu     sync.Mutex
	logger golog.Logger
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

	boardConfig := createGenericLinuxConfig(newConf)
	sysfsB, err := genericlinux.NewSysfsBoard(ctx, conf.ResourceName().AsNamed(), &boardConfig, gpioMappings, logger)
	if err != nil {
		return nil, err
	}

	gb, ok := sysfsB.(*genericlinux.SysfsBoard)
	if !ok {
		return nil, errors.New("error creating board object")
	}

	b := customLinuxBoard{SysfsBoard: gb, logger: logger}
	if err := b.Reconfigure(ctx, nil, conf); err != nil {
		return nil, err
	}
	return &b, nil
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

func createGenericLinuxConfig(conf *Config) genericlinux.Config {
	return genericlinux.Config{
		I2Cs:              conf.I2Cs,
		SPIs:              conf.SPIs,
		Analogs:           conf.Analogs,
		DigitalInterrupts: conf.DigitalInterrupts,
	}
}

// Reconfigure reconfigures the board with interrupt pins, spi and i2c, and analogs.
func (b *customLinuxBoard) Reconfigure(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	boardConfig := createGenericLinuxConfig(newConf)

	return b.ReconfigureParsedConfig(ctx, &boardConfig)
}
