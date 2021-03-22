package main

import (
	"context"
	"errors"
	_ "net/http/pprof"
	"time"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/lidar/search"
	"go.viam.com/robotcore/sensor/compass"
	compasslidar "go.viam.com/robotcore/sensor/compass/lidar"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"gonum.org/v1/gonum/stat"
)

var logger = golog.Global

func main() {
	utils.ContextualMainQuit(mainWithArgs)
}

// Arguments for the command.
type Arguments struct {
	LidarDevice *lidar.DeviceDescription `flag:"device,usage=lidar device"`
}

func mainWithArgs(ctx context.Context, args []string) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	var deviceDescs []lidar.DeviceDescription
	if argsParsed.LidarDevice == nil {
		deviceDescs = search.Devices()
		if len(deviceDescs) != 0 {
			logger.Debugf("detected %d lidar devices", len(deviceDescs))
			for _, desc := range deviceDescs {
				logger.Debugf("%s (%s)", desc.Type, desc.Path)
			}
		}
	} else {
		deviceDescs = []lidar.DeviceDescription{*argsParsed.LidarDevice}
	}

	if len(deviceDescs) == 0 {
		return errors.New("no suitable lidar device found")
	}

	return readCompass(ctx, deviceDescs)
}

func readCompass(ctx context.Context, lidarDeviceDescs []lidar.DeviceDescription) (err error) {
	lidarDevices, err := lidar.CreateDevices(ctx, lidarDeviceDescs)
	if err != nil {
		return err
	}
	for _, lidarDev := range lidarDevices {
		if err := lidarDev.Start(ctx); err != nil {
			return err
		}
		info, infoErr := lidarDev.Info(ctx)
		if infoErr != nil {
			return infoErr
		}
		logger.Infow("device", "info", info)
		dev := lidarDev
		defer func() {
			err = multierr.Combine(err, dev.Stop(context.Background()))
		}()
	}

	bestRes, bestResDevice, bestResDeviceNum, err := lidar.BestAngularResolution(ctx, lidarDevices)
	if err != nil {
		return err
	}
	bestResDesc := lidarDeviceDescs[bestResDeviceNum]

	logger.Debugf("using lidar %q as a relative compass with angular resolution %f", bestResDesc.Path, bestRes)
	var lidarCompass compass.RelativeDevice = compasslidar.From(bestResDevice)

	avgCount := 0
	avgCountLimit := 10
	var headings []float64
	quitSignaler := utils.ContextMainQuitSignal(ctx)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
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
				var heading float64
				var err error
				if avgCount != 0 && avgCount%avgCountLimit == 0 {
					logger.Debugf("variance %f", stat.Variance(headings, nil))
					headings = nil
					logger.Debug("getting median")
					heading, err = compass.MedianHeading(ctx, lidarCompass)
					if err != nil {
						logger.Errorw("failed to get lidar compass heading", "error", err)
					} else {
						logger.Infow("median heading", "data", heading)
					}
				} else {
					heading, err = lidarCompass.Heading(ctx)
					if err != nil {
						logger.Errorw("failed to get lidar compass heading", "error", err)
					} else {
						headings = append(headings, heading)
						logger.Infow("heading", "data", heading)
					}
				}
				avgCount++
			case <-quitSignaler:
				logger.Debug("marking")
				if err := lidarCompass.Mark(ctx); err != nil {
					logger.Errorw("error marking", "error", err)
				} else {
					logger.Debug("marked")
				}
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}
}
