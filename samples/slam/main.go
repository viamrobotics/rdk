package main

import (
	"bytes"
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

	"github.com/echolabsinc/robotcore/base"
	"github.com/echolabsinc/robotcore/lidar"
	"github.com/echolabsinc/robotcore/lidar/rplidar"
	"github.com/echolabsinc/robotcore/lidar/usb"
	"github.com/echolabsinc/robotcore/robots/fake"
	"github.com/echolabsinc/robotcore/robots/hellorobot"
	"github.com/echolabsinc/robotcore/slam"
	"github.com/echolabsinc/robotcore/utils"

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
	if runtime.GOOS == "linux" && strings.Contains(hostname, "hello") {
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

	deviceDescs := usb.DetectDevices()
	if len(deviceDescs) != 0 {
		golog.Global.Debugf("detected %d lidar devices", len(deviceDescs))
		for _, desc := range deviceDescs {
			golog.Global.Debugf("%s (%s)", desc.Type, desc.Path)
		}
	}
	if len(deviceOffsetFlags) == 0 && len(deviceDescs) == 0 {
		deviceDescs = append(deviceDescs,
			lidar.DeviceDescription{lidar.DeviceTypeFake, "0"})
	}
	if len(devicePathFlags) != 0 {
		deviceDescs = nil
		for i, devicePath := range devicePathFlags {
			switch devicePath {
			case fakeDev:
				deviceDescs = append(deviceDescs,
					lidar.DeviceDescription{lidar.DeviceTypeFake, fmt.Sprintf("%d", i)})
			default:
				deviceDescs = append(deviceDescs,
					lidar.DeviceDescription{rplidar.DeviceType, devicePath})
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
	)
	if err != nil {
		panic(err)
	}
	lar.Start()
	defer lar.Stop()

	config := vpx.DefaultRemoteViewConfig
	config.Debug = false

	// setup robot view
	config.StreamName = "robot view"
	robotView, err := gostream.NewRemoteView(config)
	if err != nil {
		golog.Global.Fatal(err)
	}
	lar.RegisterCommands(robotView.CommandRegistry())

	robotView.SetOnClickHandler(func(x, y int) {
		golog.Global.Debugw("click", "x", x, "y", y)
		dir := slam.DirectionFromXY(x, y)
		amount := 100
		if err := lar.Move(&amount, &dir); err != nil {
			robotView.SendText(err.Error())
			return
		}
		robotView.SendText(fmt.Sprintf("moved %q", dir))
		robotView.SendText(lar.String())
	})

	// setup world view
	config.StreamNumber = 1
	config.StreamName = "area view"
	areaViewerRemoteView, err := gostream.NewRemoteView(config)
	if err != nil {
		golog.Global.Fatal(err)
	}
	areaViewer := &slam.AreaViewer{area, 100}
	areaViewerRemoteView.SetOnDataHandler(func(data []byte) {
		golog.Global.Debugw("data", "raw", string(data))
		if bytes.HasPrefix(data, []byte("set_scale ")) {
			newScaleStr := string(bytes.TrimPrefix(data, []byte("set_scale ")))
			newScale, err := strconv.ParseInt(newScaleStr, 10, 32)
			if err != nil {
				areaViewerRemoteView.SendText(err.Error())
				return
			}
			areaViewer.ViewScale = int(newScale)
			areaViewerRemoteView.SendText(fmt.Sprintf("scale set to %d", newScale))
		}
	})

	server := gostream.NewRemoteViewServer(port, robotView, golog.Global)
	server.AddView(areaViewerRemoteView)
	server.Run()

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancelFunc()
	}()

	clientWidth := 800
	clientHeight := 600
	frameSpeed := 33 * time.Millisecond

	robotViewMatSource := gostream.ResizeImageSource{lar, clientWidth, clientHeight}
	worldViewMatSource := gostream.ResizeImageSource{areaViewer, clientWidth, clientHeight}
	go gostream.StreamSource(cancelCtx, worldViewMatSource, areaViewerRemoteView, frameSpeed)
	gostream.StreamSource(cancelCtx, robotViewMatSource, robotView, frameSpeed)

	if err := server.Stop(context.Background()); err != nil {
		golog.Global.Error(err)
	}
}
