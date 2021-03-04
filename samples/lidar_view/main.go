package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

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
					lidar.DeviceDescription{Type: lidar.DeviceTypeWS, Host: addressParts[0], Port: int(port)})
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
		info, err := lidarDev.Info(context.Background())
		if err != nil {
			golog.Global.Fatal(err)
		}
		golog.Global.Infow("device", "info", info)
		defer lidarDev.Stop(context.Background())
	}

	var lar *slam.LocationAwareRobot
	var area *slam.SquareArea
	if saveToDisk != "" {
		areaSizeMeters := 50
		areaScale := 100 // cm
		area, err = slam.NewSquareArea(areaSizeMeters, areaScale)
		if err != nil {
			golog.Global.Fatal(err)
		}

		var err error
		lar, err = slam.NewLocationAwareRobot(
			&fake.Base{},
			area,
			lidarDevices,
			nil,
			nil,
		)
		if err != nil {
			golog.Global.Fatal(err)
		}
		if err := lar.Start(); err != nil {
			panic(err)
		}
	}

	remoteView, err := gostream.NewView(vpx.DefaultViewConfig)
	if err != nil {
		golog.Global.Fatal(err)
	}

	remoteView.SetOnClickHandler(func(x, y int) {
		golog.Global.Debugw("click", "x", x, "y", y)
	})

	server := gostream.NewViewServer(port, remoteView, golog.Global)
	if err := server.Start(); err != nil {
		golog.Global.Fatal(err)
	}

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

	gostream.StreamSource(cancelCtx, autoTiler, remoteView)

	if err := server.Stop(context.Background()); err != nil {
		golog.Global.Error(err)
	}
	<-compassDone
	<-markDone
}
