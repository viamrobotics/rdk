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
	"syscall"
	"time"

	"go.uber.org/multierr"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/lidar/search"
	"go.viam.com/robotcore/sensor/compass"
	compasslidar "go.viam.com/robotcore/sensor/compass/lidar"

	"github.com/edaniels/golog"
	"gonum.org/v1/gonum/stat"
)

const deviceFlagName = "device"

func main() {
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	var address string
	flag.StringVar(&address, deviceFlagName, "", "lidar device")
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
		deviceDesc, err := lidar.ParseDeviceFlag(address, deviceFlagName)
		if err != nil {
			golog.Global.Fatal(err)
		}
		deviceDescs = append(deviceDescs, deviceDesc)
	}

	if len(deviceDescs) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	if err := readCompass(deviceDescs); err != nil {
		golog.Global.Fatal(err)
	}
}

func readCompass(deviceDescs []lidar.DeviceDescription) (err error) {
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

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	quitC := make(chan os.Signal, 2)
	signal.Notify(quitC, os.Interrupt, syscall.SIGQUIT)
	go func() {
		for {
			<-quitC
			golog.Global.Debug("marking")
			if err := lidarCompass.Mark(cancelCtx); err != nil {
				golog.Global.Errorw("error marking", "error", err)
				continue
			}
			golog.Global.Debug("marked")
		}
	}()

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
			if cancelCtx.Err() != context.Canceled {
				return cancelCtx.Err()
			}
			return nil
		default:
		}
		time.Sleep(100 * time.Millisecond)
		var heading float64
		var err error
		if avgCount != 0 && avgCount%avgCountLimit == 0 {
			golog.Global.Debugf("variance %f", stat.Variance(headings, nil))
			headings = nil
			golog.Global.Debug("getting median")
			heading, err = compass.MedianHeading(context.Background(), lidarCompass)
			if err != nil {
				golog.Global.Errorw("failed to get lidar compass heading", "error", err)
				continue
			}
			golog.Global.Infow("median heading", "data", heading)
		} else {
			heading, err = lidarCompass.Heading(context.Background())
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
