package main

import (
	"context"
	"flag"
	"log"
	"math"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/viamrobotics/robotcore/lidar"
	"github.com/viamrobotics/robotcore/lidar/search"
	"github.com/viamrobotics/robotcore/sensor/compass"
	compasslidar "github.com/viamrobotics/robotcore/sensor/compass/lidar"
	"github.com/viamrobotics/rplidar"

	"github.com/edaniels/golog"
	rplidarws "github.com/viamrobotics/rplidar/ws"
	"gonum.org/v1/gonum/stat"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	var address string
	flag.StringVar(&address, "device", "", "lidar device")
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
	if len(address) != 0 {
		deviceDescs = nil
		addressParts := strings.Split(address, ":")
		if len(addressParts) != 2 {
			golog.Global.Error("invalid address")
		}
		port, err := strconv.ParseInt(addressParts[1], 10, 64)
		if err != nil {
			golog.Global.Error("invalid address")
		}
		deviceDescs = append(deviceDescs,
			lidar.DeviceDescription{Type: rplidar.DeviceType, Host: addressParts[0], Port: int(port)})
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
	var headings []float64
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
			golog.Global.Debugf("variance %f", stat.Variance(headings, nil))
			headings = nil
			golog.Global.Debug("getting median")
			heading, err = compass.MedianHeading(lidarCompass)
			if err != nil {
				golog.Global.Errorw("failed to get lidar compass heading", "error", err)
				continue
			}
			golog.Global.Infow("median heading", "data", heading)
		} else {
			heading, err = lidarCompass.Heading()
			if err != nil {
				golog.Global.Errorw("failed to get lidar compass heading", "error", err)
				continue
			}
			headings = append(headings, heading)
			golog.Global.Infow("heading", "data", heading)
		}
		avgCount++
	}
}
