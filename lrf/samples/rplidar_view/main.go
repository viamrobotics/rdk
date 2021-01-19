package main

import (
	"context"
	"flag"
	"image"
	"image/color"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/echolabsinc/robotcore/lrf/rplidar"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/vpx"
	"gocv.io/x/gocv"
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

	devRange := float64(lidarDev.Range())
	measurements, err := lidarDev.Scan()
	if err != nil {
		golog.Global.Fatal(err)
	}
	for _, m := range measurements {
		if m.Distance() > devRange {
			devRange = m.Distance()
		}
	}
	width := int(math.Ceil(devRange * 100)) // 1 pixel/cm
	height := width
	centerX := width / 2
	centerY := height / 2

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

	gostream.StreamFunc(
		cancelCtx,
		func() image.Image {
			out := gocv.NewMatWithSize(width, height, gocv.MatTypeCV8UC3)
			defer out.Close()

			measurements, err := lidarDev.Scan()
			if err == nil {
				var drawLine bool
				// drawLine = true

				for _, next := range measurements {
					x, y := next.Coords()
					// m->cm
					scale := 100.0
					p := image.Point{centerX + int(x*scale), centerY + int(y*scale)} // scale to cm
					if drawLine {
						gocv.Line(&out, image.Point{centerX, centerY}, p, color.RGBA{R: 255}, 1)
					} else {
						gocv.Circle(&out, p, 4, color.RGBA{R: 255}, 1)
					}
				}
			} else {
				golog.Global.Error(err)
			}

			img, err := out.ToImage()
			if err != nil {
				golog.Global.Fatal(err)
			}
			return img
		},
		remoteView,
		time.Millisecond*33,
	)

	if err := server.Stop(context.Background()); err != nil {
		golog.Global.Error(err)
	}
}
