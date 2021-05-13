// Package main contains a command to view a gRPC based lidar.Lidar client
// acting as a compass.
package main

import (
	"context"
	"errors"
	_ "net/http/pprof"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/core/config"
	"go.viam.com/core/lidar"
	"go.viam.com/core/lidar/search"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/sensor/compass"
	compasslidar "go.viam.com/core/sensor/compass/lidar"
	"go.viam.com/core/utils"

	// register
	_ "go.viam.com/core/lidar/client"

	"go.uber.org/multierr"
	"gonum.org/v1/gonum/stat"
)

var logger = golog.NewDevelopmentLogger("lidar_client")

func main() {
	utils.ContextualMainQuit(mainWithArgs, logger)
}

// Arguments for the command.
type Arguments struct {
	Lidar *config.Component `flag:"device,usage=lidar"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	var components []config.Component
	if argsParsed.Lidar == nil {
		components = search.Devices()
		if len(components) != 0 {
			logger.Debugf("detected %d lidars", len(components))
			for _, comp := range components {
				logger.Debug(comp)
			}
		}
	} else {
		if argsParsed.Lidar.Type != config.ComponentTypeLidar {
			return errors.New("device must be a lidar component")
		}
		components = []config.Component{*argsParsed.Lidar}
	}

	if len(components) == 0 {
		return errors.New("no suitable lidar found")
	}

	return readCompass(ctx, components, logger)
}

func readCompass(ctx context.Context, lidarComponents []config.Component, logger golog.Logger) (err error) {
	r, err := robotimpl.New(ctx, &config.Config{Components: lidarComponents}, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, r.Close())
	}()
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
		logger.Infow("device", "info", info)
		dev := lidarDev
		defer func() {
			err = multierr.Combine(err, dev.Stop(context.Background()))
		}()
	}

	bestRes, bestResDevice, bestResDeviceNum, err := lidar.BestAngularResolution(ctx, lidars)
	if err != nil {
		return err
	}
	bestResComp := lidarComponents[bestResDeviceNum]

	logger.Debugf("using lidar %q as a relative compass with angular resolution %f", bestResComp, bestRes)
	var lidarCompass compass.RelativeCompass = compasslidar.From(bestResDevice)

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
