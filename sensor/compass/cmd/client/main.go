package main

import (
	"context"
	"time"

	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
)

var logger = golog.Global

func main() {
	utils.ContextualMainQuit(mainWithArgs)
}

// Arguments for the command.
type Arguments struct {
	DeviceAddress string `flag:"device,required,usage=device address"`
}

func mainWithArgs(ctx context.Context, args []string) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	return readCompass(ctx, argsParsed.DeviceAddress)
}

func readCompass(ctx context.Context, deviceAddress string) error {
	sensor, err := compass.NewWSDevice(ctx, deviceAddress)
	if err != nil {
		return err
	}

	tickRate := 100 * time.Millisecond
	ticker := time.NewTicker(tickRate)
	defer ticker.Stop()

	count := 0
	start := time.Now()
	defer func() {
		logger.Infow("stats", "rate", float64(count)/time.Since(start).Seconds())
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		heading, err := sensor.Heading(context.Background())
		if err != nil {
			logger.Errorw("failed to get sensor heading", "error", err)
			continue
		}
		logger.Infow("heading", "data", heading)
	}
}
