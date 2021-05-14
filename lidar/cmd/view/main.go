// Package main provides a command for viewing the output of a LiDAR.
package main

import (
	"context"
	"image"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/core/config"
	"go.viam.com/core/lidar"
	"go.viam.com/core/lidar/search"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/sensor/compass"
	compasslidar "go.viam.com/core/sensor/compass/lidar"
	"go.viam.com/core/slam"
	"go.viam.com/core/utils"

	// register
	_ "go.viam.com/core/lidar/client"

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
	areaSizeMeters = 50.
	unitsPerMeter  = 100. // cm

	logger = golog.NewDevelopmentLogger("lidar_view")
)

// Arguments for the command.
type Arguments struct {
	Port       utils.NetPortFlag  `flag:"0"`
	Lidars     []config.Component `flag:"device,usage=lidars"`
	SaveToDisk string             `flag:"save,usage=save data to disk (LAS)"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}
	if argsParsed.Port == 0 {
		argsParsed.Port = utils.NetPortFlag(defaultPort)
	}
	for _, comp := range argsParsed.Lidars {
		if comp.Type != config.ComponentTypeLidar {
			return errors.New("only lidar components can be in device flag")
		}
	}

	if len(argsParsed.Lidars) == 0 {
		argsParsed.Lidars = search.Devices()
		if len(argsParsed.Lidars) != 0 {
			logger.Debugf("detected %d lidars", len(argsParsed.Lidars))
			for _, comp := range argsParsed.Lidars {
				logger.Debug(comp)
			}
		}
	}

	if len(argsParsed.Lidars) == 0 {
		argsParsed.Lidars = append(argsParsed.Lidars,
			config.Component{
				Type:  config.ComponentTypeLidar,
				Host:  "0",
				Model: string(lidar.TypeFake),
			})
	}

	return viewLidar(ctx, int(argsParsed.Port), argsParsed.Lidars, argsParsed.SaveToDisk, logger)
}

func viewLidar(ctx context.Context, port int, components []config.Component, saveToDisk string, logger golog.Logger) (err error) {
	r, err := robotimpl.New(ctx, &config.Config{Components: components}, logger)
	if err != nil {
		return err
	}
	lidarNames := r.LidarNames()
	lidars := make([]lidar.Lidar, 0, len(lidarNames))
	for _, name := range lidarNames {
		lidars = append(lidars, r.LidarByName(name))
	}
	for _, lidarDev := range lidars {
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
			lidars,
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
	for _, dev := range lidars {
		autoTiler.AddSource(lidar.NewImageSource(image.Point{streamWidth, streamWidth}, dev))
		break
	}

	compassDone, err := startCompass(ctx, lidars, components)
	if err != nil {
		return multierr.Combine(err, server.Stop(context.Background()))
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
		return errors.Errorf("error stopping location aware robot: %w", err)
	}
	if err := area.WriteToFile(saveToDisk); err != nil {
		return errors.Errorf("error saving to disk: %w", err)
	}
	return nil
}

func startCompass(ctx context.Context, lidars []lidar.Lidar, lidarComponents []config.Component) (<-chan struct{}, error) {
	bestRes, bestResDevice, bestResDeviceNum, err := lidar.BestAngularResolution(ctx, lidars)
	if err != nil {
		return nil, err
	}
	bestResComp := lidarComponents[bestResDeviceNum]

	logger.Debugf("using lidar %q as a relative compass with angular resolution %f", bestResComp, bestRes)
	var lidarCompass compass.RelativeCompass = compasslidar.From(bestResDevice)
	compassDone := make(chan struct{})

	quitSignaler := utils.ContextMainQuitSignal(ctx)
	utils.PanicCapturingGo(func() {
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
	})

	return compassDone, nil
}
