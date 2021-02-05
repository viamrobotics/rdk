package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/viamrobotics/robotcore/base"
	"github.com/viamrobotics/robotcore/lidar"
	"github.com/viamrobotics/robotcore/lidar/rplidar"
	"github.com/viamrobotics/robotcore/lidar/search"
	"github.com/viamrobotics/robotcore/robots/fake"
	"github.com/viamrobotics/robotcore/robots/hellorobot"
	"github.com/viamrobotics/robotcore/sensor/compass"
	"github.com/viamrobotics/robotcore/sensor/compass/gy511"
	"github.com/viamrobotics/robotcore/serial"
	"github.com/viamrobotics/robotcore/slam"
	"github.com/viamrobotics/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/vpx"
)

const fakeDev = "fake"

func main() {
	var baseType string
	var devicePathFlags utils.StringFlags
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
	flag.Var(&devicePathFlags, "device", "lidar devices")
	flag.Var(&deviceOffsetFlags, "device-offset", "lidar device offets relative to first")
	flag.Parse()

	// The area is 600m^2 tracked in centimeters
	// 0 means no detected obstacle
	// 1 means a detected obstacle
	// TODO(erd): where is center? is a hack to just square the whole thing?
	areaSizeMeters := 600
	areaScale := 100 // cm
	area := slam.NewSquareArea(areaSizeMeters, areaScale)
	areaCenter := area.Center()
	areaSize, areaSizeScale := area.Size()

	var base base.Base
	switch baseType {
	case fakeDev:
		base = &fake.Base{}
	case "hello":
		robot := hellorobot.New()
		base = robot.Base()
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
		compassSensor, err = gy511.New(sensorDevices[0].Path)
		if err != nil {
			golog.Global.Fatal(err)
		}
		defer compassSensor.Close()
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

	lar, err := slam.NewLocationAwareRobot(
		base,
		image.Point{areaCenter.X, areaCenter.Y},
		lidarDevices,
		deviceOffests,
		area,
		image.Point{areaSize * areaSizeScale, areaSize * areaSizeScale},
		compassSensor,
	)
	if err != nil {
		panic(err)
	}
	lar.Start()
	defer lar.Stop()
	areaViewer := &slam.AreaViewer{area, 100}

	config := vpx.DefaultRemoteViewConfig
	config.Debug = false

	// setup robot view
	config.StreamName = "robot view"
	remoteView, err := gostream.NewRemoteView(config)
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
			remoteView.SendText(err.Error())
			return
		}
		if resp != "" {
			remoteView.SendText(resp)
		}
	})

	server := gostream.NewRemoteViewServer(port, remoteView, golog.Global)
	server.Run()

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancelFunc()
	}()

	frameSpeed := 33 * time.Millisecond

	robotViewMatSource := gostream.ResizeImageSource{lar, clientWidth, clientHeight}
	worldViewMatSource := gostream.ResizeImageSource{areaViewer, clientWidth, clientHeight}
	started := make(chan struct{})
	go gostream.StreamNamedSourceOnce(cancelCtx, func() { close(started) }, robotViewMatSource, "robot perspective", remoteView, frameSpeed)
	<-started
	gostream.StreamNamedSource(cancelCtx, worldViewMatSource, "world", remoteView, frameSpeed)

	if err := server.Stop(context.Background()); err != nil {
		golog.Global.Error(err)
	}
}
