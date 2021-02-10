package main

import (
	"context"
	"flag"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/viamrobotics/robotcore/lidar"
	"github.com/viamrobotics/robotcore/lidar/rplidar"
	"github.com/viamrobotics/robotcore/lidar/search"
	"github.com/viamrobotics/robotcore/sensor/compass"
	compasslidar "github.com/viamrobotics/robotcore/sensor/compass/lidar"

	"github.com/edaniels/golog"
)

func main() {
	var devicePath string
	flag.StringVar(&devicePath, "device", "", "lidar device")
	flag.Parse()

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
	if len(devicePath) != 0 {
		deviceDescs = nil
		switch devicePath {
		default:
			deviceDescs = append(deviceDescs,
				lidar.DeviceDescription{Type: rplidar.DeviceType, Path: devicePath})
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

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	termC := make(chan os.Signal, 2)
	signal.Notify(termC, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-termC
		cancelFunc()
	}()

	avgCount := 0
	avgCountLimit := 10
	for {
		select {
		case <-cancelCtx.Done():
			return
		default:
		}
		time.Sleep(100 * time.Millisecond)
		var heading float64
		var err error
		if avgCount != 0 && avgCount%avgCountLimit == 0 {
			golog.Global.Debug("getting average")
			// heading, err = compass.AverageHeading(lidarCompass)
		} else {
			heading, err = lidarCompass.Heading()
		}
		avgCount++
		if err != nil {
			golog.Global.Errorw("failed to get lidar compass heading", "error", err)
			continue
		}
		golog.Global.Infow("heading", "data", heading)
	}
}
