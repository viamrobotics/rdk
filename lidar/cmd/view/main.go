package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/multierr"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/lidar/search"
	"go.viam.com/robotcore/sensor/compass"
	compasslidar "go.viam.com/robotcore/sensor/compass/lidar"
	"go.viam.com/robotcore/slam"
	"go.viam.com/robotcore/utils"

	// register fake
	"go.viam.com/robotcore/robots/fake"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
)

const fakeDev = "fake"

func main() {
	utils.ContextualMain(mainWithArgs)
}

func mainWithArgs(ctx context.Context, args []string) error {
	cmdLine := flag.NewFlagSet(args[0], flag.ContinueOnError)

	var addressFlags utils.StringFlags
	cmdLine.Var(&addressFlags, "device", "lidar devices")
	var saveToDisk string
	cmdLine.StringVar(&saveToDisk, "save", "", "save data to disk (LAS)")
	if err := cmdLine.Parse(args[1:]); err != nil {
		return err
	}

	port := 5555
	if flag.NArg() >= 1 {
		portParsed, err := strconv.ParseInt(flag.Arg(1), 10, 32)
		if err != nil {
			golog.Global.Fatal(err)
		}
		port = int(portParsed)
	}

	deviceDescs, err := search.Devices()
	if err != nil {
		golog.Global.Debugw("error searching for lidar devices", "error", err)
	}
	if len(deviceDescs) != 0 {
		golog.Global.Debugf("detected %d lidar devices", len(deviceDescs))
		for _, desc := range deviceDescs {
			golog.Global.Debugf("%s (%s)", desc.Type, desc.Path)
		}
	}
	if len(deviceDescs) == 0 {
		deviceDescs = append(deviceDescs,
			lidar.DeviceDescription{Type: lidar.DeviceTypeFake, Path: "0"})
	}
	if len(addressFlags) != 0 {
		deviceDescs = nil
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
					lidar.DeviceDescription{Type: lidar.DeviceTypeFake, Path: fmt.Sprintf("%d", i)})
			default:
				deviceDescs = append(deviceDescs,
					lidar.DeviceDescription{Type: lidar.DeviceTypeWS, Host: addressParts[0], Port: int(port)})
			}
		}
	}

	if len(deviceDescs) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	if err := viewLidar(ctx, port, deviceDescs, saveToDisk); err != nil {
		golog.Global.Fatal(err)
	}
	return nil
}

func viewLidar(ctx context.Context, port int, deviceDescs []lidar.DeviceDescription, saveToDisk string) (err error) {
	lidarDevices, err := lidar.CreateDevices(context.Background(), deviceDescs)
	if err != nil {
		return err
	}
	for _, lidarDev := range lidarDevices {
		info, infoErr := lidarDev.Info(context.Background())
		if infoErr != nil {
			return infoErr
		}
		golog.Global.Infow("device", "info", info)
		dev := lidarDev
		defer func() {
			err = multierr.Combine(err, dev.Stop(context.Background()))
		}()
	}

	var lar *slam.LocationAwareRobot
	var area *slam.SquareArea
	if saveToDisk != "" {
		areaSizeMeters := 50
		areaScale := 100 // cm
		area, err = slam.NewSquareArea(areaSizeMeters, areaScale)
		if err != nil {
			return err
		}

		var err error
		lar, err = slam.NewLocationAwareRobot(
			context.Background(),
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

	remoteView.SetOnClickHandler(func(x, y int) {
		golog.Global.Debugw("click", "x", x, "y", y)
	})

	server := gostream.NewViewServer(port, remoteView, golog.Global)
	if err := server.Start(); err != nil {
		return err
	}

	autoTiler := gostream.NewAutoTiler(800, 600)
	for _, dev := range lidarDevices {
		autoTiler.AddSource(lidar.NewImageSource(dev))
		break
	}

	bestResolution := math.MaxFloat64
	bestResolutionDeviceNum := 0
	for i, lidarDev := range lidarDevices {
		angRes, err := lidarDev.AngularResolution(context.Background())
		if err != nil {
			return err
		}
		if angRes < bestResolution {
			bestResolution = angRes
			bestResolutionDeviceNum = i
		}
	}
	bestResolutionDevice := lidarDevices[bestResolutionDeviceNum]
	desc := deviceDescs[bestResolutionDeviceNum]
	golog.Global.Debugf("using lidar %q as a relative compass with angular resolution %f", desc.Path, bestResolution)
	var lidarCompass compass.RelativeDevice = compasslidar.From(bestResolutionDevice)
	compassDone := make(chan struct{})
	go func() {
		defer close(compassDone)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			time.Sleep(time.Second)
			heading, err := lidarCompass.Heading(ctx)
			if err != nil {
				golog.Global.Errorw("failed to get lidar compass heading", "error", err)
				continue
			}
			golog.Global.Infow("heading", "data", heading)
		}
	}()

	markDone := make(chan struct{})
	if signaler := utils.ContextMainQuitSignal(ctx); signaler != nil {
		go func() {
			defer close(markDone)
			for {
				select {
				case <-ctx.Done():
					return
				case <-signaler:
				}
				golog.Global.Debug("marking")
				if err := lidarCompass.Mark(ctx); err != nil {
					golog.Global.Errorw("error marking", "error", err)
					continue
				}
				golog.Global.Debug("marked")
			}
		}()
	} else {
		close(markDone)
	}

	utils.ContextMainReadyFunc(ctx)()

	gostream.StreamSource(ctx, autoTiler, remoteView)

	err = server.Stop(context.Background())
	<-compassDone
	<-markDone
	if saveToDisk != "" {
		if err := lar.Stop(); err != nil {
			golog.Global.Errorw("error stopping location aware robot", "error", err)
		}
		if err := area.WriteToFile(saveToDisk); err != nil {
			golog.Global.Errorw("error saving to disk", err)
		}
	}
	return err
}
