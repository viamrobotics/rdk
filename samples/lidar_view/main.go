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
	"strings"
	"syscall"
	"time"

	"github.com/viamrobotics/rplidar"
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
	"github.com/edaniels/gostream/codec/vpx"
	rplidarws "github.com/viamrobotics/rplidar/ws"
)

const fakeDev = "fake"

func main() {
	var addressFlags utils.StringFlags
	flag.Var(&addressFlags, "device", "lidar devices")
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
					lidar.DeviceDescription{Type: rplidar.DeviceType, Host: addressParts[0], Port: int(port)})
			}
		}
	}

	if len(deviceDescs) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	lidarDevices, err := lidar.CreateDevices(context.Background(), deviceDescs)
	if err != nil {
		golog.Global.Fatal(err)
	}
	for _, lidarDev := range lidarDevices {
		if rpl, ok := lidarDev.(*rplidarws.Device); ok {
			info, err := rpl.Info(context.Background())
			if err != nil {
				golog.Global.Fatal(err)
			}
			golog.Global.Infow("rplidar",
				"model", info.Model,
				"serial", info.SerialNumber,
				"firmware_ver", info.FirmwareVersion,
				"hardware_rev", info.HardwareRevision)
		}
		defer lidarDev.Stop(context.Background())
	}

	var lar *slam.LocationAwareRobot
	var area *slam.SquareArea
	if saveToDisk != "" {
		areaSizeMeters := 50
		areaScale := 100 // cm
		area = slam.NewSquareArea(areaSizeMeters, areaScale)
		areaCenter := area.Center()
		areaX, areaY := area.Dims()

		var err error
		lar, err = slam.NewLocationAwareRobot(
			&fake.Base{},
			image.Point{areaCenter.X, areaCenter.Y},
			lidarDevices,
			nil,
			area,
			image.Point{areaX, areaY},
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
		angRes, err := lidarDev.AngularResolution(context.Background())
		if err != nil {
			golog.Global.Fatal(err)
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
			case <-cancelCtx.Done():
				return
			default:
			}
			time.Sleep(time.Second)
			heading, err := lidarCompass.Heading(cancelCtx)
			if err != nil {
				golog.Global.Errorw("failed to get lidar compass heading", "error", err)
				continue
			}
			golog.Global.Infow("heading", "data", heading)
		}
	}()
	quitC := make(chan os.Signal, 2)
	signal.Notify(quitC, os.Interrupt, syscall.SIGQUIT)
	markDone := make(chan struct{})
	go func() {
		defer close(markDone)
		for {
			select {
			case <-cancelCtx.Done():
				return
			case <-quitC:
			}
			golog.Global.Debug("marking")
			lidarCompass.Mark()
			golog.Global.Debug("marked")
		}
	}()

	gostream.StreamSource(cancelCtx, autoTiler, remoteView, 1*time.Second)

	if err := server.Stop(context.Background()); err != nil {
		golog.Global.Error(err)
	}
	<-compassDone
	<-markDone
}
