package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/echolabsinc/robotcore/lidar"
	"github.com/echolabsinc/robotcore/lidar/rplidar"
	"github.com/echolabsinc/robotcore/utils/stream"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/vpx"
)

func main() {
	flag.Parse()

	devPath := "/dev/ttyUSB2"
	if flag.NArg() >= 1 {
		devPath = flag.Arg(0)
	}
	port := 5555
	if flag.NArg() >= 2 {
		portParsed, err := strconv.ParseInt(flag.Arg(1), 10, 32)
		if err != nil {
			golog.Global.Fatal(err)
		}
		port = int(portParsed)
	}
	lidarDev, err := rplidar.NewRPLidar(devPath)
	if err != nil {
		golog.Global.Fatal(err)
	}

	golog.Global.Infof("RPLIDAR S/N: %s", lidarDev.SerialNumber())
	golog.Global.Infof("\n"+
		"Firmware Ver: %s\n"+
		"Hardware Rev: %d\n",
		lidarDev.FirmwareVersion(),
		lidarDev.HardwareRevision())

	lidarDev.Start()
	defer lidarDev.Stop()

	config := vpx.DefaultRemoteViewConfig
	config.Debug = false
	remoteView, err := gostream.NewRemoteView(config)
	if err != nil {
		golog.Global.Fatal(err)
	}

	remoteView.SetOnClickHandler(func(x, y int) {
		golog.Global.Debugw("click", "x", x, "y", y)
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

	stream.MatSource(cancelCtx, lidar.MatSource{lidarDev}, remoteView, 33*time.Millisecond, golog.Global)

	if err := server.Stop(context.Background()); err != nil {
		golog.Global.Error(err)
	}
}
