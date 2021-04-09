package main

import (
	"context"
	"errors"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/sensor/compass/gy511"
	"go.viam.com/robotcore/serial"
	"go.viam.com/robotcore/utils"

	"go.uber.org/multierr"
)

var logger = golog.NewDevelopmentLogger("gy511_client")

func main() {
	utils.ContextualMainQuit(mainWithArgs, logger)
}

// Arguments for the command.
type Arguments struct {
	Calibrate bool `flag:"calibrate,usage=calibrate compass"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	devices := serial.SearchDevices(serial.SearchFilter{Type: serial.DeviceTypeArduino})
	if len(devices) == 0 {
		return errors.New("no suitable device found")
	}

	return readCompass(ctx, devices[0], argsParsed.Calibrate)
}

func readCompass(ctx context.Context, serialDeviceDesc serial.DeviceDescription, calibrate bool) (err error) {
	sensor, err := gy511.New(ctx, serialDeviceDesc.Path, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, sensor.Close())
	}()

	if calibrate {
		utils.ContextMainReadyFunc(ctx)()
		if err := sensor.StartCalibration(ctx); err != nil {
			return err
		}
		quitSignaler := utils.ContextMainQuitSignal(ctx)
		<-quitSignaler
		if err := sensor.StopCalibration(ctx); err != nil {
			return err
		}
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	var once bool
	for {
		err := func() error {
			defer utils.ContextMainIterFunc(ctx)()
			if !once {
				once = true
				defer utils.ContextMainReadyFunc(ctx)()
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				readings, err := sensor.Readings(ctx)
				if err != nil {
					logger.Errorw("failed to get sensor reading", "error", err)
				} else {
					logger.Infow("readings", "data", readings)
				}
				heading, err := sensor.Heading(ctx)
				if err != nil {
					logger.Errorw("failed to get sensor heading", "error", err)
				} else {
					logger.Infow("heading", "data", heading)
				}
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}
}
