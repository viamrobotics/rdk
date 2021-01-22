package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/echolabsinc/robotcore/base"
	"github.com/echolabsinc/robotcore/lidar"
	"github.com/echolabsinc/robotcore/utils/stream"
	"github.com/echolabsinc/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/vpx"
	"github.com/james-bowman/sparse"
	"gocv.io/x/gocv"
)

type FakeLidar struct {
	posX, posY int
	started    bool
}

func (fl *FakeLidar) Start() {
	fl.started = true
}

func (fl *FakeLidar) Stop() {
	fl.started = false
}

func (fl *FakeLidar) Close() {

}

func (fl *FakeLidar) Scan() (lidar.Measurements, error) {
	if !fl.started {
		return nil, nil
	}
	h := fnv.New64()
	if _, err := h.Write([]byte(fmt.Sprintf("%d,%d", fl.posX, fl.posY))); err != nil {
		return nil, err
	}
	r := rand.NewSource(int64(h.Sum64()))
	measurements := make(lidar.Measurements, 0, 360)
	getFloat64 := func() float64 {
	again:
		f := float64(r.Int63()) / (1 << 63)
		if f == 1 {
			goto again // resample
		}
		return f
	}
	for i := 0; i < cap(measurements); i++ {
		measurements = append(measurements, lidar.NewMeasurement(getFloat64()*360, getFloat64()*float64(fl.Range())))
	}
	return measurements, nil
}

func (fl *FakeLidar) Range() int {
	return 25
}

func (fl *FakeLidar) Bounds() (image.Point, error) {
	return image.Point{fl.Range(), fl.Range()}, nil
}

func main() {
	flag.Parse()

	devPath := "/dev/ttyUSB2"
	if flag.NArg() >= 1 {
		devPath = flag.Arg(0)
	}
	_ = devPath // TODO(erd): remove
	port := 5555
	if flag.NArg() >= 2 {
		portParsed, err := strconv.ParseInt(flag.Arg(1), 10, 32)
		if err != nil {
			golog.Global.Fatal(err)
		}
		port = int(portParsed)
	}

	lidarDev := &FakeLidar{}
	// lidarDev, err := rplidar.NewRPLidar(devPath)
	// if err != nil {
	// 	golog.Global.Fatal(err)
	// }

	// golog.Global.Infof("RPLIDAR S/N: %s", lidarDev.SerialNumber())
	// golog.Global.Infof("\n"+
	// 	"Firmware Ver: %s\n"+
	// 	"Hardware Rev: %d\n",
	// 	lidarDev.FirmwareVersion(),
	// 	lidarDev.HardwareRevision())

	lidarDev.Start()
	defer lidarDev.Stop()

	// The room is 600m^2 tracked in centimeters
	// 0 means no detected obstacle
	// 1 means a detected obstacle
	// TODO(erd): where is center? is a hack to just square the whole thing?
	squareMeters := 600
	squareMillis := squareMeters * 100
	roomPoints := sparse.NewDOK(squareMillis, squareMillis)
	centerX := squareMillis / 2
	centerY := centerX

	base := &fakeBase{centerX, centerY, squareMillis, squareMillis}
	roomPointsMu := &sync.Mutex{}
	lar := &LocationAwareLidar{base, lidarDev, roomPoints, roomPointsMu, 100}

	config := vpx.DefaultRemoteViewConfig
	config.Debug = false
	config.StreamName = "base view"
	baseView, err := gostream.NewRemoteView(config)
	if err != nil {
		golog.Global.Fatal(err)
	}

	baseView.SetOnDataHandler(func(data []byte) {
		golog.Global.Debugw("data", "raw", string(data))
		if err := lar.handleData(data, baseView.SendText); err != nil {
			baseView.SendText(err.Error())
		}
	})

	baseView.SetOnClickHandler(func(x, y int) {
		golog.Global.Debugw("click", "x", x, "y", y)
		if err := lar.handleClick(x, y, 800, 600, baseView.SendText); err != nil {
			baseView.SendText(err.Error())
		}
	})

	config.StreamNumber = 1
	config.StreamName = "world view"
	worldView, err := gostream.NewRemoteView(config)
	if err != nil {
		golog.Global.Fatal(err)
	}

	server := gostream.NewRemoteViewServer(port, baseView, golog.Global)
	server.AddView(worldView)
	server.Run()

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancelFunc()
	}()

	baseViewMatSource := stream.ResizeMatSource{lar, 800, 600}
	worldViewMatSource := stream.ResizeMatSource{&worldViewer{roomPoints, roomPointsMu}, 800, 600}
	go stream.MatSource(cancelCtx, worldViewMatSource, worldView, time.Second, golog.Global)
	stream.MatSource(cancelCtx, baseViewMatSource, baseView, 33*time.Millisecond, golog.Global)

	if err := server.Stop(context.Background()); err != nil {
		golog.Global.Error(err)
	}
}

type worldViewer struct {
	roomPoints   *sparse.DOK
	roomPointsMu *sync.Mutex
}

func (wv *worldViewer) NextColorDepthPair() (gocv.Mat, vision.DepthMap, error) {
	x, y := wv.roomPoints.Dims()
	// TODO(erd): any way to make this really fast? Allocate these in advance in
	// a goroutine? Pool?
	out := gocv.NewMatWithSize(x, y, gocv.MatTypeCV8UC3)

	wv.roomPointsMu.Lock()
	defer wv.roomPointsMu.Unlock()
	wv.roomPoints.DoNonZero(func(x, y int, _ float64) {
		p := image.Point{x, y}
		gocv.Circle(&out, p, 8, color.RGBA{R: 255}, 1)
	})

	return out, vision.DepthMap{}, nil
}

func (wv *worldViewer) Close() {
}

type fakeBase struct {
	posX, posY     int
	boundX, boundY int
}

func (fb *fakeBase) MoveStraight(distanceMM int, speed int, block bool) error {
	return nil
}

func (fb *fakeBase) Spin(degrees int, power int, block bool) error {
	return nil
}

func (fb *fakeBase) Stop() error {
	return nil
}

func (fb *fakeBase) String() string {
	return fmt.Sprintf("pos: (%d, %d)", fb.posX, fb.posY)
}

type moveDir string

const (
	moveDirUp    = moveDir("up")
	moveDirLeft  = moveDir("left")
	moveDirDown  = moveDir("down")
	moveDirRight = moveDir("right")
)

func (fb *fakeBase) Move(dir moveDir, amount int) error {
	errMsg := fmt.Errorf("cannot move %q; stuck", dir)
	switch dir {
	case moveDirUp:
		if fb.posY-amount < 0 {
			return errMsg
		}
		golog.Global.Debugw("up", "amount", amount)
		fb.posY -= amount
	case moveDirLeft:
		if fb.posX-amount < 0 {
			return errMsg
		}
		golog.Global.Debugw("left", "amount", amount)
		fb.posX -= amount
	case moveDirDown:
		if fb.posY+amount >= fb.boundY {
			return errMsg
		}
		golog.Global.Debugw("down", "amount", amount)
		fb.posY += amount
	case moveDirRight:
		if fb.posX+amount >= fb.boundX {
			return errMsg
		}
		golog.Global.Debugw("right", "amount", amount)
		fb.posX += amount
	default:
		return fmt.Errorf("unknown direction %q", dir)
	}
	return nil
}

func (fb *fakeBase) Close() {

}

type LocationAwareLidar struct {
	base         base.Base
	device       lidar.Device
	roomPoints   *sparse.DOK
	roomPointsMu *sync.Mutex
	scaleDown    int
}

func (lar *LocationAwareLidar) update() {
	basePosX := lar.base.(*fakeBase).posX
	basePosY := lar.base.(*fakeBase).posY
	lar.device.(*FakeLidar).posX = basePosX
	lar.device.(*FakeLidar).posY = basePosY
	measurements, err := lar.device.Scan()
	if err != nil {
		panic(err)
	}

	dimX, dimY := lar.roomPoints.Dims()
	lar.roomPointsMu.Lock()
	defer lar.roomPointsMu.Unlock()
	for _, next := range measurements {
		x, y := next.Coords()
		detectedX := basePosX + int(x*float64(lar.scaleDown))
		detectedY := basePosY + int(y*float64(lar.scaleDown))
		if detectedX < 0 || detectedX >= dimX {
			continue
		}
		if detectedY < 0 || detectedY >= dimY {
			continue
		}
		lar.roomPoints.Set(detectedX, detectedY, 1)
	}
}

func (lar *LocationAwareLidar) NextColorDepthPair() (gocv.Mat, vision.DepthMap, error) {
	lar.update()
	bounds, err := lar.device.Bounds()
	if err != nil {
		return gocv.Mat{}, vision.DepthMap{}, err
	}
	scaleDown := lar.scaleDown
	bounds.X *= scaleDown
	bounds.Y *= scaleDown
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	out := gocv.NewMatWithSize(bounds.X, bounds.Y, gocv.MatTypeCV8UC3)

	var drawLine bool
	// drawLine = true

	basePosX := lar.base.(*fakeBase).posX
	basePosY := lar.base.(*fakeBase).posY
	minX := basePosX - bounds.X/2
	maxX := basePosX + bounds.X/2
	minY := basePosY - bounds.Y/2
	maxY := basePosY + bounds.Y/2

	// TODO(erd): any way to get a submatrix? may need to segment each one
	// if this starts going slower. fast as long as there are not many points
	lar.roomPoints.DoNonZero(func(x, y int, _ float64) {
		if x < minX || x > maxX || y < minY || y > maxY {
			return
		}
		distX := basePosX - x
		distY := basePosY - y
		relX := centerX - distX
		relY := centerY - distY

		p := image.Point{relX, relY}
		if drawLine {
			gocv.Line(&out, image.Point{centerX, centerY}, p, color.RGBA{R: 255}, 1)
		} else {
			gocv.Circle(&out, p, 4, color.RGBA{R: 255}, 1)
		}
	})

	return out, vision.DepthMap{}, nil
}

func (lar *LocationAwareLidar) handleData(data []byte, respondMsg func(msg string)) error {
	if bytes.HasPrefix(data, []byte("move: ")) {
		dir := moveDir(bytes.TrimPrefix(data, []byte("move: ")))
		if err := lar.base.(*fakeBase).Move(dir, lar.device.Range()*lar.scaleDown); err != nil {
			return err
		}
		respondMsg(fmt.Sprintf("moved %q", dir))
		respondMsg(lar.base.(*fakeBase).String())
	} else if bytes.Equal(data, []byte("pos")) {
		respondMsg(lar.base.(*fakeBase).String())
	} else if bytes.Equal(data, []byte("lidar_stop")) {
		lar.device.Stop()
		respondMsg("lidar stopped")
	} else if bytes.Equal(data, []byte("lidar_start")) {
		lar.device.Start()
		respondMsg("lidar started")
	}
	return nil
}

func (lar *LocationAwareLidar) handleClick(x, y, sX, sY int, respondMsg func(msg string)) error {
	centerX := sX / 2
	centerY := sX / 2
	var dir moveDir
	if x < centerX {
		if y < centerY {
			dir = moveDirUp
		} else {
			dir = moveDirLeft
		}
	} else {
		if y < centerY {
			dir = moveDirDown
		} else {
			dir = moveDirRight
		}
	}
	if err := lar.base.(*fakeBase).Move(dir, lar.device.Range()*lar.scaleDown); err != nil {
		return err
	}
	respondMsg(fmt.Sprintf("moved %q", dir))
	respondMsg(lar.base.(*fakeBase).String())
	return nil
}

func (lar *LocationAwareLidar) Close() {

}
