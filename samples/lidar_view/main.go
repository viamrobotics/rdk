package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/viamrobotics/robotcore/lidar"
	"github.com/viamrobotics/robotcore/lidar/rplidar"
	"github.com/viamrobotics/robotcore/lidar/search"
	"github.com/viamrobotics/robotcore/sensor/compass"
	compasslidar "github.com/viamrobotics/robotcore/sensor/compass/lidar"
	"github.com/viamrobotics/robotcore/slam"
	"github.com/viamrobotics/robotcore/utils"

	// register fake
	"github.com/viamrobotics/robotcore/robots/fake"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/vpx"
)

const fakeDev = "fake"

func main() {
	var devicePathFlags utils.StringFlags
	flag.Var(&devicePathFlags, "device", "lidar devices")
	var saveToDisk string
	flag.StringVar(&saveToDisk, "save", "", "save data to disk (LAS)")
	flag.Parse()

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
	if len(devicePathFlags) != 0 {
		deviceDescs = nil
		for i, devicePath := range devicePathFlags {
			switch devicePath {
			case fakeDev:
				deviceDescs = append(deviceDescs,
					lidar.DeviceDescription{Type: lidar.DeviceTypeFake, Path: fmt.Sprintf("%d", i)})
			default:
				deviceDescs = append(deviceDescs,
					lidar.DeviceDescription{Type: rplidar.DeviceType, Path: devicePath})
			}
		}
	}

	if len(deviceDescs) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	lidarDevices, err := lidar.CreateDevices(deviceDescs)
	if err != nil {
		golog.Global.Fatal(err)
	}
	for i, lidarDev := range lidarDevices {
		if rpl, ok := lidarDev.(*rplidar.RPLidar); ok {
			golog.Global.Infow("rplidar",
				"dev_path", deviceDescs[i].Path,
				"model", rpl.Model(),
				"serial", rpl.SerialNumber(),
				"firmware_ver", rpl.FirmwareVersion(),
				"hardware_rev", rpl.HardwareRevision())
		}
		defer lidarDev.Stop()
	}

	var lar *slam.LocationAwareRobot
	var area *slam.SquareArea
	if saveToDisk != "" {
		areaSizeMeters := 50
		areaScale := 100 // cm
		area = slam.NewSquareArea(areaSizeMeters, areaScale)
		areaCenter := area.Center()
		areaSize, areaSizeScale := area.Size()

		var err error
		lar, err = slam.NewLocationAwareRobot(
			&fake.Base{},
			image.Point{areaCenter.X, areaCenter.Y},
			lidarDevices,
			nil,
			area,
			image.Point{areaSize * areaSizeScale, areaSize * areaSizeScale},
			nil,
		)
		if err != nil {
			golog.Global.Fatal(err)
		}
		if err := lar.Start(); err != nil {
			panic(err)
		}
	}

	config := vpx.DefaultRemoteViewConfig
	config.Debug = false
	remoteView, err := gostream.NewRemoteView(config)
	if err != nil {
		golog.Global.Fatal(err)
	}

	remoteView.SetOnClickHandler(func(x, y int) {
		golog.Global.Debugw("click", "x", x, "y", y)
	})

	server := gostream.NewRemoteViewServer(port, remoteView, golog.Global)
	server.Run()

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancelFunc()
		if saveToDisk != "" {
			lar.Stop()
			if err := area.WriteToFile(saveToDisk); err != nil {
				golog.Global.Fatal(err)
			}
		}
	}()

	autoTiler := gostream.NewAutoTiler(1280, 720)
	for _, dev := range lidarDevices {
		autoTiler.AddSource(lidar.NewImageSource(dev))
		break
	}

	bestResolution := math.MaxFloat64
	bestResolutionDeviceNum := 0
	for i, lidarDev := range lidarDevices {
		if lidarDev.AngularResolution() < bestResolution {
			bestResolution = lidarDev.AngularResolution()
			bestResolutionDeviceNum = i
		}
	}
	bestResolutionDevice := lidarDevices[bestResolutionDeviceNum]
	desc := deviceDescs[bestResolutionDeviceNum]
	golog.Global.Debugf("using lidar %q as a relative compass with angular resolution %f", desc.Path, bestResolution)
	var lidarCompass compass.RelativeDevice = compasslidar.From(bestResolutionDevice)
	go func() {
		for {
			select {
			case <-cancelCtx.Done():
				return
			default:
			}
			time.Sleep(time.Second)
			heading, err := lidarCompass.Heading()
			if err != nil {
				golog.Global.Errorw("failed to get lidar compass heading", "error", err)
				continue
			}
			golog.Global.Infow("heading", "data", heading)
		}
	}()
	quitC := make(chan os.Signal, 2)
	signal.Notify(quitC, os.Interrupt, syscall.SIGQUIT)
	go func() {
		for {
			<-quitC
			golog.Global.Debug("marking")
			lidarCompass.Mark()
			golog.Global.Debug("marked")
		}
	}()

	gostream.StreamSource(cancelCtx, autoTiler, remoteView, 33*time.Millisecond)

	if err := server.Stop(context.Background()); err != nil {
		golog.Global.Error(err)
	}
}
