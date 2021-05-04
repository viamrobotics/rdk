package main

import (
	"context"
	"time"

	"go.uber.org/multierr"

	"go.viam.com/robotcore/sensor/compass/client"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
)

var logger = golog.NewDevelopmentLogger("compass_client")

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

	return readCompass(ctx, argsParsed.DeviceAddress, logger)
}

func readCompass(ctx context.Context, deviceAddress string, logger golog.Logger) (err error) {
	sensor, err := client.New(ctx, deviceAddress, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, utils.TryClose(sensor))
	}()

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
			if !utils.SelectContextOrWaitChan(ctx, ticker.C) {
				return false
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
