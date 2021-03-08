package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"go.viam.com/robotcore/base"
	"go.viam.com/robotcore/base/augment"
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
	var baseType string
	var lidarAddressFlags utils.StringFlags
	var compassAddressFlag string
	var lidarOffsetFlags utils.StringFlags
	hostname, err := os.Hostname()
	if err != nil {
		golog.Global.Fatal(err)
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

	areaSizeMeters := 50
	areaScale := 100 // cm
	area, err := slam.NewSquareArea(areaSizeMeters, areaScale)
	if err != nil {
		golog.Global.Fatal(err)
	}

	var baseDevice base.Device
	switch baseType {
	case fakeDev:
		baseDevice = &fake.Base{}
	case "hello":
		robot, err := hellorobot.New()
		if err != nil {
			golog.Global.Fatal(err)
		}
		baseDevice, err = robot.Base()
		if err != nil {
			golog.Global.Fatal(err)
		}
	default:
		golog.Global.Fatal(fmt.Errorf("do not know how to make a %q base", baseType))
	}

	var compassSensor compass.Device
	if compassAddressFlag != "" {
		sensor, err := compass.NewWSDevice(context.Background(), compassAddressFlag)
		if err != nil {
			golog.Global.Fatal(err)
		}
		compassSensor = sensor
	}

	var deviceOffests []slam.DeviceOffset
	for _, flags := range lidarOffsetFlags {
		if flags == "" {
			golog.Global.Fatal("offset format is angle,x,y")
		}
		split := strings.Split(flags, ",")
		if len(split) != 3 {
			golog.Global.Fatal("offset format is angle,x,y")
		}
		angle, err := strconv.ParseFloat(split[0], 64)
		if err != nil {
			golog.Global.Fatal(err)
		}
		distX, err := strconv.ParseFloat(split[1], 64)
		if err != nil {
			golog.Global.Fatal(err)
		}
		distY, err := strconv.ParseFloat(split[2], 64)
		if err != nil {
			golog.Global.Fatal(err)
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
		deviceDescs = nil
		for i, address := range lidarAddressFlags {
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

	if len(deviceOffests) != 0 && len(deviceOffests) >= len(deviceDescs) {
		golog.Global.Fatal(fmt.Errorf("can only have up to %d device offsets", len(deviceDescs)-1))
	}

	port := 5555
	if flag.NArg() >= 1 {
		portParsed, err := strconv.ParseInt(flag.Arg(1), 10, 32)
		if err != nil {
			golog.Global.Fatal(err)
		}
		port = int(portParsed)
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

	if compassSensor == nil {
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
		compassSensor = compasslidar.From(bestResolutionDevice)
	}

	if compassSensor != nil {
		baseDevice = augment.Device(baseDevice, compassSensor)
	}

	lar, err := slam.NewLocationAwareRobot(
		baseDevice,
		area,
		lidarDevices,
		deviceOffests,
		compassSensor,
	)
	if err != nil {
		golog.Global.Fatal(err)
	}
	if err := lar.Start(); err != nil {
		golog.Global.Fatal(err)
	}
	defer lar.Stop()
	areaViewer := &slam.AreaViewer{area}

	config := x264.DefaultViewConfig
	config.StreamName = "robot view"
	remoteView, err := gostream.NewView(config)
	if err != nil {
		golog.Global.Fatal(err)
	}
	lar.RegisterCommands(remoteView.CommandRegistry())

	clientWidth := 800
	clientHeight := 600

	remoteView.SetOnClickHandler(func(x, y int) {
		golog.Global.Debugw("click", "x", x, "y", y)
		resp, err := lar.HandleClick(x, y, clientWidth, clientHeight)
		if err != nil {
			remoteView.SendTextToAll(err.Error())
			return
		}
		if resp != "" {
			remoteView.SendTextToAll(resp)
		}
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
	}()

	robotViewMatSource := gostream.ResizeImageSource{lar, clientWidth, clientHeight}
	worldViewMatSource := gostream.ResizeImageSource{areaViewer, clientWidth, clientHeight}
	started := make(chan struct{})
	go func() {
		close(started)
		gostream.StreamNamedSource(cancelCtx, robotViewMatSource, "robot perspective", remoteView)
	}()
	<-started
	gostream.StreamNamedSource(cancelCtx, worldViewMatSource, "world (published)", remoteView)

	if err := server.Stop(context.Background()); err != nil {
		golog.Global.Error(err)
	}
}
