//go:build linux

// Package genericlinux is for Linux boards, and this particular file is for GPIO pins using the
// ioctl interface, indirectly by way of mkch's gpio package.
package genericlinux

import (
	"context"
	"strconv"
	"sync"

	"github.com/mkch/gpio"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
)

type digitalInterrupt struct {
	interrupt  board.DigitalInterrupt
	line       *gpio.LineWithEvent
	cancelCtx  context.Context
	cancelFunc func()
}

func createDigitalInterrupt(ctx context.Context, config board.DigitalInterruptConfig,
	gpioMappings map[int]GPIOBoardMapping, activeBackgroundWorkers *sync.WaitGroup,
) (*digitalInterrupt, error) {
	pinInt, err := strconv.Atoi(config.Pin)
	if err != nil {
		return nil, errors.Errorf("pin numbers must be numerical, not '%s'", config.Pin)
	}
	mapping, ok := gpioMappings[pinInt]
	if !ok {
		return nil, errors.Errorf("Unknown interrupt pin %s", config.Pin)
	}

	chip, err := gpio.OpenChip(mapping.GPIOChipDev)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(chip.Close)

	line, err := chip.OpenLineWithEvents(
		uint32(mapping.GPIO), gpio.Input, gpio.BothEdges, "viam-interrupt")
	if err != nil {
		return nil, err
	}

	interrupt, err := board.CreateDigitalInterrupt(config)
	if err != nil {
		return nil, multierr.Combine(err, line.Close())
	}

	cancelCtx, cancelFunc := context.WithCancel(ctx)
	result := digitalInterrupt{
		interrupt:  interrupt,
		line:       line,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}
	result.startMonitor(activeBackgroundWorkers)
	return &result, nil
}

func (di *digitalInterrupt) startMonitor(activeBackgroundWorkers *sync.WaitGroup) {
	activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			select {
			case <-di.cancelCtx.Done():
				return
			case event := <-di.line.Events():
				utils.UncheckedError(di.interrupt.Tick(
					di.cancelCtx, event.RisingEdge, uint64(event.Time.UnixNano())))
			}
		}
	}, activeBackgroundWorkers.Done)
}

func (di *digitalInterrupt) Close() error {
	// We shut down the background goroutine that monitors this interrupt, but don't need to wait
	// for it to finish shutting down because it doesn't use anything in the line itself (just a
	// channel of events that the line generates). It will shut down sometime soon, and if that's
	// after the line is closed, that's fine.
	di.cancelFunc()
	return di.line.Close()
}
