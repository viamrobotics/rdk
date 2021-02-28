package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"math"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"go.viam.com/robotcore/base"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/lidar/search"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/robots/hellorobot"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/sensor/compass/gy511"
	compasslidar "go.viam.com/robotcore/sensor/compass/lidar"
	"go.viam.com/robotcore/serial"
	"go.viam.com/robotcore/slam"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/vpx"
)

const fakeDev = "fake"

func main() {
	var baseType string
	var addressFlags utils.StringFlags
	var deviceOffsetFlags utils.StringFlags
	hostname, err := os.Hostname()
	if err != nil {
		golog.Global.Fatal(err)
	}
	if runtime.GOOS == "linux" && strings.Contains(hostname, "stretch") {
		flag.StringVar(&baseType, "base-type", "hello", "type of mobile base")
	} else {
		flag.StringVar(&baseType, "base-type", fakeDev, "type of mobile base")
	}
	flag.Var(&addressFlags, "device", "lidar devices")
	flag.Var(&deviceOffsetFlags, "device-offset", "lidar device offets relative to first")
	flag.Parse()

	areaSizeMeters := 50
	areaScale := 100 // cm
	area := slam.NewSquareArea(areaSizeMeters, areaScale)
	areaCenter := area.Center()

	var baseDevice base.Device
	switch baseType {
	case fakeDev:
		baseDevice = &fake.Base{}
	case "hello":
		robot := hellorobot.New()
		baseDevice = robot.Base()
	default:
		panic(fmt.Errorf("do not know how to make a %q base", baseType))
	}

	// TODO(erd): this will find too many
	sensorDevices, err := serial.SearchDevices(serial.SearchFilter{Type: serial.DeviceTypeArduino})
	if err != nil {
		golog.Global.Fatal(err)
	}
	var compassSensor compass.Device
	if len(sensorDevices) != 0 {
		var err error
		compassSensor, err = gy511.New(context.Background(), sensorDevices[0].Path)
		if err != nil {
			golog.Global.Fatal(err)
		}
		defer func() {
			if err := compassSensor.Close(context.Background()); err != nil {
				golog.Global.Error(err)
			}
		}()
	}

	var deviceOffests []slam.DeviceOffset
	for _, flags := range deviceOffsetFlags {
		if flags == "" {
			panic("offset format is angle,x,y")
		}
		split := strings.Split(flags, ",")
		if len(split) != 3 {
			panic("offset format is angle,x,y")
		}
		angle, err := strconv.ParseFloat(split[0], 64)
		if err != nil {
			panic(err)
		}
		distX, err := strconv.ParseFloat(split[1], 64)
		if err != nil {
			panic(err)
		}
		distY, err := strconv.ParseFloat(split[2], 64)
		if err != nil {
			panic(err)
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
	if len(deviceOffsetFlags) == 0 && len(deviceDescs) == 0 {
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

	if len(deviceOffests) != 0 && len(deviceOffests) >= len(deviceDescs) {
		panic(fmt.Errorf("can only have up to %d device offsets", len(deviceDescs)-1))
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
		baseDevice = base.Augment(baseDevice, compassSensor)
	}

	lar, err := slam.NewLocationAwareRobot(
		baseDevice,
		image.Point{areaCenter.X, areaCenter.Y},
		area,
		lidarDevices,
		deviceOffests,
		compassSensor,
	)
	if err != nil {
		panic(err)
	}
	if err := lar.Start(); err != nil {
		panic(err)
	}
	defer lar.Stop()
	areaViewer := &slam.AreaViewer{area}

	config := vpx.DefaultViewConfig
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
