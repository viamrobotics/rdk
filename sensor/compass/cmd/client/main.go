package main

import (
	"context"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/rlog"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/utils"
)

var logger = rlog.Logger.Named("compass_client")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

// Arguments for the command.
type Arguments struct {
	DeviceAddress string `flag:"device,required,usage=device address"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
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

	var once bool
	for {
		cont := func() bool {
			defer utils.ContextMainIterFunc(ctx)()
			if !once {
				once = true
				defer utils.ContextMainReadyFunc(ctx)()
			}
			select {
			case <-ctx.Done():
				return false
			default:
			}
			select {
			case <-ctx.Done():
				return false
			case <-ticker.C:
			}
			heading, err := sensor.Heading(context.Background())
			if err != nil {
				logger.Errorw("failed to get sensor heading", "error", err)
			} else {
				logger.Infow("heading", "data", heading)
			}
			return true
		}()
		if !cont {
			break
		}
	}
	return nil
}
