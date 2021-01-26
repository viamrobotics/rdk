package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/echolabsinc/robotcore/lidar"
	"github.com/echolabsinc/robotcore/lidar/rplidar"
	"github.com/echolabsinc/robotcore/lidar/samples/rplidar_move/support"
	"github.com/echolabsinc/robotcore/utils/stream"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/vpx"
	"github.com/james-bowman/sparse"
)

type stringFlags []string

func (sf *stringFlags) Set(value string) error {
	*sf = append(*sf, value)
	return nil
}

func (sf *stringFlags) String() string {
	return fmt.Sprint([]string(*sf))
}

func main() {
	var devicePathFlags stringFlags
	flag.Var(&devicePathFlags, "device", "lidar device")
	flag.Parse()

	devicePaths := []string{"/dev/ttyUSB2", "/dev/ttyUSB3"}
	if len(devicePathFlags) != 0 {
		devicePaths = []string(devicePathFlags)
	}

	if len(devicePaths) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	port := 5555
	if flag.NArg() >= 1 {
		portParsed, err := strconv.ParseInt(flag.Arg(1), 10, 32)
		if err != nil {
			golog.Global.Fatal(err)
		}
		port = int(portParsed)
	}

	golog.Global.Debugw("registering devices")
	var lidarDevices []lidar.Device
	for _, devPath := range devicePaths {
		if devPath == "fake" {
			lidarDevices = append(lidarDevices, &support.FakeLidar{})
			continue
		}
		lidarDev, err := rplidar.NewRPLidar(devPath)
		if err != nil {
			golog.Global.Fatal(err)
		}
		lidarDevices = append(lidarDevices, lidarDev)
	}

	for i, lidarDev := range lidarDevices {
		if rpl, ok := lidarDev.(*rplidar.RPLidar); ok {
			golog.Global.Infow("rplidar",
				"dev_path", devicePaths[i],
				"model", rpl.Model(),
				"serial", rpl.SerialNumber(),
				"firmware_ver", rpl.FirmwareVersion(),
				"hardware_rev", rpl.HardwareRevision())
		}
		lidarDev.Start()
		defer lidarDev.Stop()
	}

	golog.Global.Debugw("setting up room")

	// The room is 600m^2 tracked in centimeters
	// 0 means no detected obstacle
	// 1 means a detected obstacle
	// TODO(erd): where is center? is a hack to just square the whole thing?
	squareMeters := 600
	squareMillis := squareMeters * 100
	roomPoints := sparse.NewDOK(squareMillis, squareMillis)
	centerX := squareMillis / 2
	centerY := centerX

	base := &support.FakeBase{centerX, centerY, squareMillis, squareMillis}
	roomPointsMu := &sync.Mutex{}
	baseRoomPoints := make([]*sparse.DOK, 0, len(lidarDevices))
	for range lidarDevices {
		baseRoomPoints = append(baseRoomPoints, sparse.NewDOK(squareMillis, squareMillis))
	}
	lar := &support.LocationAwareLidar{
		Base:               base,
		Devices:            lidarDevices,
		RoomPointsCombined: roomPoints,
		RoomPoints:         baseRoomPoints,
		RoomPointsMu:       roomPointsMu,
		ScaleDown:          100,
	}

	config := vpx.DefaultRemoteViewConfig
	config.Debug = false
	config.StreamName = "base view"
	baseView, err := gostream.NewRemoteView(config)
	if err != nil {
		golog.Global.Fatal(err)
	}

	baseView.SetOnDataHandler(func(data []byte) {
		golog.Global.Debugw("data", "raw", string(data))
		if err := lar.HandleData(data, baseView.SendText); err != nil {
			baseView.SendText(err.Error())
		}
	})

	baseView.SetOnClickHandler(func(x, y int) {
		golog.Global.Debugw("click", "x", x, "y", y)
		if err := lar.HandleClick(x, y, 800, 600, baseView.SendText); err != nil {
			baseView.SendText(err.Error())
		}
	})

	config.StreamNumber = 1
	config.StreamName = "world view"
	worldRemoteView, err := gostream.NewRemoteView(config)
	if err != nil {
		golog.Global.Fatal(err)
	}

	worldView := &support.WorldViewer{roomPoints, roomPointsMu, 100}
	worldRemoteView.SetOnDataHandler(func(data []byte) {
		golog.Global.Debugw("data", "raw", string(data))
		if bytes.HasPrefix(data, []byte("set_scale ")) {
			newScaleStr := string(bytes.TrimPrefix(data, []byte("set_scale ")))
			newScale, err := strconv.ParseInt(newScaleStr, 10, 32)
			if err != nil {
				worldRemoteView.SendText(err.Error())
				return
			}
			worldView.Scale = int(newScale)
			worldRemoteView.SendText(fmt.Sprintf("scale set to %d", newScale))
		}
	})

	server := gostream.NewRemoteViewServer(port, baseView, golog.Global)
	server.AddView(worldRemoteView)
	server.Run()

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancelFunc()
	}()

	lar.Cull()

	baseViewMatSource := stream.ResizeMatSource{lar, 800, 600}
	worldViewMatSource := stream.ResizeMatSource{worldView, 800, 600}
	go stream.MatSource(cancelCtx, worldViewMatSource, worldRemoteView, 33*time.Millisecond, golog.Global)
	stream.MatSource(cancelCtx, baseViewMatSource, baseView, 33*time.Millisecond, golog.Global)

	if err := server.Stop(context.Background()); err != nil {
		golog.Global.Error(err)
	}
}
