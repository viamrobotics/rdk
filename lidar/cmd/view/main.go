package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/multierr"
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
)

const fakeDev = "fake"

func main() {
	utils.ContextualMainQuit(mainWithArgs)
}

var (
	defaultPort  = 5555
	streamWidth  = 800
	streamHeight = 600

	// for saving to disk
	areaSizeMeters = 50
	unitsPerMeter  = 100 // cm

	logger = golog.Global
)

func mainWithArgs(ctx context.Context, args []string) error {
	parsed, err := parseFlags(args)
	if err != nil {
		return err
	}

	if len(parsed.LidarDevices) == 0 {
		parsed.LidarDevices, err = search.Devices()
		if err != nil {
			return fmt.Errorf("error searching for lidar devices: %w", err)
		}
		if len(parsed.LidarDevices) != 0 {
			logger.Debugf("detected %d lidar devices", len(parsed.LidarDevices))
			for _, desc := range parsed.LidarDevices {
				logger.Debugf("%s (%s)", desc.Type, desc.Path)
			}
		}
	}

	if len(parsed.LidarDevices) == 0 {
		parsed.LidarDevices = append(parsed.LidarDevices,
			lidar.DeviceDescription{Type: lidar.DeviceTypeFake, Path: "fake-0"})
	}

	return viewLidar(ctx, parsed.Port, parsed.LidarDevices, parsed.SaveToDisk)
}

// Arguments for the command (parsed).
type Arguments struct {
	Port         int
	LidarDevices []lidar.DeviceDescription
	SaveToDisk   string
}

func parseFlags(args []string) (Arguments, error) {
	cmdLine := flag.NewFlagSet(args[0], flag.ContinueOnError)

	var addressFlags utils.StringFlags
	cmdLine.Var(&addressFlags, "device", "lidar devices")
	var saveToDisk string
	cmdLine.StringVar(&saveToDisk, "save", "", "save data to disk (LAS)")
	if err := cmdLine.Parse(args[1:]); err != nil {
		return Arguments{}, err
	}

	port := defaultPort
	if cmdLine.NArg() >= 1 {
		portParsed, err := strconv.ParseInt(cmdLine.Arg(1), 10, 32)
		if err != nil {
			return Arguments{}, err
		}
		port = int(portParsed)
	}

	var deviceDescs []lidar.DeviceDescription
	if len(addressFlags) == 0 {
		return Arguments{Port: port, SaveToDisk: saveToDisk}, nil
	}
	for i, address := range addressFlags {
		addressParts := strings.Split(address, ":")
		if len(addressParts) != 2 {
			continue
		}
		port, err := strconv.ParseInt(addressParts[1], 10, 64)
		if err != nil {
			continue
		}
		switch address {
		case fakeDev:
			deviceDescs = append(deviceDescs,
				lidar.DeviceDescription{Type: lidar.DeviceTypeFake, Path: fmt.Sprintf("fake-%d", i)})
		default:
			deviceDescs = append(deviceDescs,
				lidar.DeviceDescription{Type: lidar.DeviceTypeWS, Host: addressParts[0], Port: int(port)})
		}
	}

	return Arguments{Port: port, LidarDevices: deviceDescs, SaveToDisk: saveToDisk}, nil
}

func viewLidar(ctx context.Context, port int, deviceDescs []lidar.DeviceDescription, saveToDisk string) (err error) {
	// setup lidar devices
	lidarDevices, err := lidar.CreateDevices(ctx, deviceDescs)
	if err != nil {
		return err
	}
	for _, lidarDev := range lidarDevices {
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
		area, err = slam.NewSquareArea(areaSizeMeters, unitsPerMeter)
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
		autoTiler.AddSource(lidar.NewImageSource(dev))
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
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			select {
			case <-ticker.C:
			case <-quitSignaler:
				logger.Debug("marking")
				if err := lidarCompass.Mark(ctx); err != nil {
					logger.Errorw("error marking", "error", err)
					continue
				}
				logger.Debug("marked")
			}
			heading, err := lidarCompass.Heading(ctx)
			if err != nil {
				logger.Errorw("failed to get lidar compass heading", "error", err)
				continue
			}
			logger.Infow("heading", "data", heading)
		}
	}()

	return compassDone, nil
}
