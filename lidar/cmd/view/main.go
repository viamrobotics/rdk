package main

import (
	"context"
	"fmt"
	"image"
	"time"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/lidar/search"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/sensor/compass"
	compasslidar "go.viam.com/robotcore/sensor/compass/lidar"
	"go.viam.com/robotcore/slam"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	"go.uber.org/multierr"
)

func main() {
	utils.ContextualMainQuit(mainWithArgs, logger)
}

var (
	defaultPort  = 5555
	streamWidth  = 800
	streamHeight = 800

	// for saving to disk
	areaSizeMeters = 50
	unitsPerMeter  = 100 // cm

	logger = golog.NewDevelopmentLogger("lidar_view")
)

// Arguments for the command.
type Arguments struct {
	Port         utils.NetPortFlag         `flag:"0"`
	LidarDevices []lidar.DeviceDescription `flag:"device,usage=lidar devices"`
	SaveToDisk   string                    `flag:"save,usage=save data to disk (LAS)"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}
	if argsParsed.Port == 0 {
		argsParsed.Port = utils.NetPortFlag(defaultPort)
	}

	if len(argsParsed.LidarDevices) == 0 {
		argsParsed.LidarDevices = search.Devices()
		if len(argsParsed.LidarDevices) != 0 {
			logger.Debugf("detected %d lidar devices", len(argsParsed.LidarDevices))
			for _, desc := range argsParsed.LidarDevices {
				logger.Debugf("%s (%s)", desc.Type, desc.Path)
			}
		}
	}

	if len(argsParsed.LidarDevices) == 0 {
		argsParsed.LidarDevices = append(argsParsed.LidarDevices,
			lidar.DeviceDescription{Type: lidar.DeviceTypeFake, Path: "0"})
	}

	return viewLidar(ctx, int(argsParsed.Port), argsParsed.LidarDevices, argsParsed.SaveToDisk)
}

func viewLidar(ctx context.Context, port int, deviceDescs []lidar.DeviceDescription, saveToDisk string) (err error) {
	// setup lidar devices
	lidarDevices, err := lidar.CreateDevices(ctx, deviceDescs)
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
		if info != nil {
			logger.Infow("device", "info", info)
		}
		dev := lidarDev
		defer func() {
			err = multierr.Combine(err, dev.Stop(context.Background()))
		}()
	}

	var lar *slam.LocationAwareRobot
	var area *slam.SquareArea
	if saveToDisk != "" {
		area, err = slam.NewSquareArea(areaSizeMeters, unitsPerMeter, logger)
		if err != nil {
			return err
		}

		var err error
		lar, err = slam.NewLocationAwareRobot(
			ctx,
			&fake.Base{},
			area,
			lidarDevices,
			nil,
			nil,
			logger,
		)
		if err != nil {
			return err
		}
		if err := lar.Start(); err != nil {
			return err
		}
	}

	remoteView, err := gostream.NewView(x264.DefaultViewConfig)
	if err != nil {
		return err
	}
	server := gostream.NewViewServer(port, remoteView, logger)
	if err := server.Start(); err != nil {
		return err
	}

	autoTiler := gostream.NewAutoTiler(streamWidth, streamHeight)
	for _, dev := range lidarDevices {
		autoTiler.AddSource(lidar.NewImageSource(image.Point{streamWidth, streamWidth}, dev))
		break
	}

	compassDone, err := startCompass(ctx, lidarDevices, deviceDescs)
	if err != nil {
		return err
	}

	utils.ContextMainReadyFunc(ctx)()

	gostream.StreamSource(ctx, autoTiler, remoteView)
	err = server.Stop(context.Background())
	<-compassDone
	if err != nil {
		return err
	}
	if saveToDisk == "" {
		return nil
	}
	if err := lar.Stop(); err != nil {
		return fmt.Errorf("error stopping location aware robot: %w", err)
	}
	if err := area.WriteToFile(saveToDisk); err != nil {
		return fmt.Errorf("error saving to disk: %w", err)
	}
	return nil
}

func startCompass(ctx context.Context, lidarDevices []lidar.Device, lidarDeviceDescs []lidar.DeviceDescription) (<-chan struct{}, error) {
	bestRes, bestResDevice, bestResDeviceNum, err := lidar.BestAngularResolution(ctx, lidarDevices)
	if err != nil {
		return nil, err
	}
	bestResDesc := lidarDeviceDescs[bestResDeviceNum]

	logger.Debugf("using lidar %q as a relative compass with angular resolution %f", bestResDesc.Path, bestRes)
	var lidarCompass compass.RelativeDevice = compasslidar.From(bestResDevice)
	compassDone := make(chan struct{})

	quitSignaler := utils.ContextMainQuitSignal(ctx)
	go func() {
		defer close(compassDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
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
					heading, err := lidarCompass.Heading(ctx)
					if err != nil {
						logger.Errorw("failed to get lidar compass heading", "error", err)
					} else {
						logger.Infow("heading", "data", heading)
					}
				case <-quitSignaler:
					logger.Debug("marking")
					if err := lidarCompass.Mark(ctx); err != nil {
						logger.Errorw("error marking", "error", err)
					} else {
						logger.Debug("marked")
					}
				}
				return true
			}()
			if !cont {
				return
			}
		}
	}()

	return compassDone, nil
}
