package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"go.uber.org/multierr"
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/lidar/search"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/robots/hellorobot"
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
	params, err := parseFlags()
	if err != nil {
		flag.Usage()
		os.Exit(1)
	}

	if err := runSlam(params); err != nil {
		golog.Global.Fatal(err)
	}
}

type runParams struct {
	port           int
	baseType       string
	deviceDescs    []lidar.DeviceDescription
	deviceOffests  []slam.DeviceOffset
	compassAddress string
}

const lidarFlagName = "lidar"

func parseFlags() (runParams, error) {
	var baseType string
	var lidarAddressFlags utils.StringFlags
	var compassAddressFlag string
	var lidarOffsetFlags utils.StringFlags
	hostname, err := os.Hostname()
	if err != nil {
		return runParams{}, err
	}
	if runtime.GOOS == "linux" && strings.Contains(hostname, "stretch") {
		flag.StringVar(&baseType, "base-type", "hello", "type of mobile base")
	} else {
		flag.StringVar(&baseType, "base-type", fakeDev, "type of mobile base")
	}
	flag.Var(&lidarAddressFlags, "lidar", "lidar devices")
	flag.StringVar(&compassAddressFlag, "compass", "", "compass devices")
	flag.Var(&lidarOffsetFlags, "lidar-offset", "lidar device offets relative to first")
	flag.Parse()

	var deviceOffests []slam.DeviceOffset
	for _, flags := range lidarOffsetFlags {
		if flags == "" {
			return runParams{}, errors.New("offset format is angle,x,y")
		}
		split := strings.Split(flags, ",")
		if len(split) != 3 {
			return runParams{}, errors.New("offset format is angle,x,y")
		}
		angle, err := strconv.ParseFloat(split[0], 64)
		if err != nil {
			return runParams{}, err
		}
		distX, err := strconv.ParseFloat(split[1], 64)
		if err != nil {
			return runParams{}, err
		}
		distY, err := strconv.ParseFloat(split[2], 64)
		if err != nil {
			return runParams{}, err
		}
		deviceOffests = append(deviceOffests, slam.DeviceOffset{angle, distX, distY})
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
	if len(lidarOffsetFlags) == 0 && len(deviceDescs) == 0 {
		deviceDescs = append(deviceDescs,
			lidar.DeviceDescription{Type: lidar.DeviceTypeFake, Path: "0"})
	}
	if len(lidarAddressFlags) != 0 {
		deviceDescs, err = lidar.ParseDeviceFlags(lidarAddressFlags, lidarFlagName)
		if err != nil {
			return runParams{}, err
		}
	}

	if len(deviceDescs) == 0 {
		return runParams{}, errors.New("no device descriptions parsed")
	}

	if len(deviceOffests) != 0 && len(deviceOffests) >= len(deviceDescs) {
		return runParams{}, fmt.Errorf("can only have up to %d device offsets", len(deviceDescs)-1)
	}

	port := 5555
	if flag.NArg() >= 1 {
		portParsed, err := strconv.ParseInt(flag.Arg(1), 10, 32)
		if err != nil {
			return runParams{}, err
		}
		port = int(portParsed)
	}

	return runParams{
		port:           port,
		baseType:       baseType,
		deviceDescs:    deviceDescs,
		deviceOffests:  deviceOffests,
		compassAddress: compassAddressFlag,
	}, nil
}

func runSlam(params runParams) (err error) {
	areaSizeMeters := 50
	unitsPerMeter := 100 // cm
	area, err := slam.NewSquareArea(areaSizeMeters, unitsPerMeter)
	if err != nil {
		return err
	}

	var baseDevice api.Base
	switch params.baseType {
	case fakeDev:
		baseDevice = &fake.Base{}
	case "hello":
		robot, err := hellorobot.New()
		if err != nil {
			return err
		}
		baseDevice, err = robot.Base()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("do not know how to make a %q base", params.baseType)
	}

	var compassSensor compass.Device
	if params.compassAddress != "" {
		sensor, err := compass.NewWSDevice(context.Background(), params.compassAddress)
		if err != nil {
			return err
		}
		compassSensor = sensor
	}

	lidarDevices, err := lidar.CreateDevices(context.Background(), params.deviceDescs)
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

	if compassSensor == nil {
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
		desc := params.deviceDescs[bestResolutionDeviceNum]
		golog.Global.Debugf("using lidar %q as a relative compass with angular resolution %f", desc.Path, bestResolution)
		compassSensor = compasslidar.From(bestResolutionDevice)
	}

	if compassSensor != nil {
		baseDevice = compass.BaseWithCompass(baseDevice, compassSensor)
	}

	lar, err := slam.NewLocationAwareRobot(
		context.Background(),
		baseDevice,
		area,
		lidarDevices,
		params.deviceOffests,
		compassSensor,
	)
	if err != nil {
		return err
	}
	if err := lar.Start(); err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(lar.Stop())
	}()
	areaViewer := &slam.AreaViewer{area}

	config := x264.DefaultViewConfig
	config.StreamName = "robot view"
	remoteView, err := gostream.NewView(config)
	if err != nil {
		return err
	}
	lar.RegisterCommands(remoteView.CommandRegistry())

	clientWidth := 800
	clientHeight := 600

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancelFunc()
	}()

	remoteView.SetOnClickHandler(func(x, y int) {
		golog.Global.Debugw("click", "x", x, "y", y)
		resp, err := lar.HandleClick(cancelCtx, x, y, clientWidth, clientHeight)
		if err != nil {
			remoteView.SendTextToAll(err.Error())
			return
		}
		if resp != "" {
			remoteView.SendTextToAll(resp)
		}
	})

	server := gostream.NewViewServer(params.port, remoteView, golog.Global)
	if err := server.Start(); err != nil {
		return err
	}

	robotViewMatSource := gostream.ResizeImageSource{lar, clientWidth, clientHeight}
	worldViewMatSource := gostream.ResizeImageSource{areaViewer, clientWidth, clientHeight}
	started := make(chan struct{})
	go func() {
		close(started)
		gostream.StreamNamedSource(cancelCtx, robotViewMatSource, "robot perspective", remoteView)
	}()
	<-started
	gostream.StreamNamedSource(cancelCtx, worldViewMatSource, "world (published)", remoteView)

	return server.Stop(context.Background())
}
