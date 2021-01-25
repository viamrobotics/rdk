package main

import (
	"bytes"
	"context"
	"errors"
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
	"github.com/echolabsinc/robotcore/lidar/rplidar"
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
	seed       int64
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
	r := rand.NewSource(int64(h.Sum64()) + fl.seed)
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
			lidarDevices = append(lidarDevices, &FakeLidar{})
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

	base := &fakeBase{centerX, centerY, squareMillis, squareMillis}
	roomPointsMu := &sync.Mutex{}
	baseRoomPoints := make([]*sparse.DOK, 0, len(lidarDevices))
	for range lidarDevices {
		baseRoomPoints = append(baseRoomPoints, sparse.NewDOK(squareMillis, squareMillis))
	}
	lar := &LocationAwareLidar{
		base:               base,
		devices:            lidarDevices,
		roomPointsCombined: roomPoints,
		roomPoints:         baseRoomPoints,
		roomPointsMu:       roomPointsMu,
		scaleDown:          100,
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
	worldRemoteView, err := gostream.NewRemoteView(config)
	if err != nil {
		golog.Global.Fatal(err)
	}

	worldView := &worldViewer{roomPoints, roomPointsMu, 100}
	worldRemoteView.SetOnDataHandler(func(data []byte) {
		golog.Global.Debugw("data", "raw", string(data))
		if bytes.HasPrefix(data, []byte("set_scale ")) {
			newScaleStr := string(bytes.TrimPrefix(data, []byte("set_scale ")))
			newScale, err := strconv.ParseInt(newScaleStr, 10, 32)
			if err != nil {
				worldRemoteView.SendText(err.Error())
				return
			}
			worldView.scale = int(newScale)
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

	lar.cull()

	baseViewMatSource := stream.ResizeMatSource{lar, 800, 600}
	worldViewMatSource := stream.ResizeMatSource{worldView, 800, 600}
	go stream.MatSource(cancelCtx, worldViewMatSource, worldRemoteView, 33*time.Millisecond, golog.Global)
	stream.MatSource(cancelCtx, baseViewMatSource, baseView, 33*time.Millisecond, golog.Global)

	if err := server.Stop(context.Background()); err != nil {
		golog.Global.Error(err)
	}
}

type worldViewer struct {
	roomPoints   *sparse.DOK
	roomPointsMu *sync.Mutex
	scale        int
}

func (wv *worldViewer) NextColorDepthPair() (gocv.Mat, vision.DepthMap, error) {
	x, y := wv.roomPoints.Dims()
	// TODO(erd): any way to make this really fast? Allocate these in advance in
	// a goroutine? Pool?
	out := gocv.NewMatWithSize(x/wv.scale, y/wv.scale, gocv.MatTypeCV8UC3)

	wv.roomPointsMu.Lock()
	defer wv.roomPointsMu.Unlock()
	wv.roomPoints.DoNonZero(func(x, y int, _ float64) {
		p := image.Point{x / wv.scale, y / wv.scale}
		gocv.Circle(&out, p, 1, color.RGBA{R: 255}, 1)
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
	mu                 sync.Mutex
	base               base.Base
	devices            []lidar.Device
	clientDeviceNum    int
	roomPointsCombined *sparse.DOK
	roomPoints         []*sparse.DOK
	roomPointsMu       *sync.Mutex
	scaleDown          int
}

func (lar *LocationAwareLidar) cull() {
	bounds, err := lar.devices[0].Bounds()
	if err != nil {
		panic(err)
	}
	scaleDown := lar.scaleDown
	bounds.X *= scaleDown
	bounds.Y *= scaleDown

	// TODO(erd): cancellation
	// TODO(erd): combined
	ticker := time.NewTicker(time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
			}

			basePosX := lar.base.(*fakeBase).posX
			basePosY := lar.base.(*fakeBase).posY
			minX := basePosX - bounds.X/2
			maxX := basePosX + bounds.X/2
			minY := basePosY - bounds.Y/2
			maxY := basePosY + bounds.Y/2

			// decrement observable area which will be refreshed by scans
			// within the area (assuming the lidar is active)
			func() {
				lar.roomPointsMu.Lock()
				defer lar.roomPointsMu.Unlock()
				lar.roomPointsCombined.DoNonZero(func(x, y int, v float64) {
					if x < minX || x > maxX || y < minY || y > maxY {
						return
					}
					lar.roomPointsCombined.Set(x, y, v-1)
				})
			}()
		}
	}()
}

func (lar *LocationAwareLidar) update() {
	basePosX := lar.base.(*fakeBase).posX
	basePosY := lar.base.(*fakeBase).posY

	for _, dev := range lar.devices {
		if fake, ok := dev.(*FakeLidar); ok {
			fake.posX = basePosX
			fake.posY = basePosY
		}
	}
	// var allMeasurements []lidar.Measurements
	// for _, dev := range lar.devices {
	measurements, err := lar.devices[0].Scan()
	if err != nil {
		golog.Global.Debugw("bad scan", "error", err)
		return
	}
	// }

	// TODO(erd): combined
	dimX, dimY := lar.roomPointsCombined.Dims()
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
		// TTL 3 seconds
		// TODO(erd): should we also add here as a sense of permanency
		// Want to also combine this with occlusion, right. So if there's
		// a wall detected, and we're pretty confident it's staying there,
		// it being occluded should give it a low chance of it being removed.
		// Realistically once the bounds of a location are determined, most
		// environments would only have it deform over very long periods of time.
		// Probably longer than the lifetime of the application itself.
		lar.roomPointsCombined.Set(detectedX, detectedY, 3) // TODO(erd): move to configurable
	}
}

func (lar *LocationAwareLidar) roomToView() (image.Point, *sparse.DOK, error) {
	devNum := lar.getClientDeviceNum()
	if devNum == -1 {
		// TODO(erd): combined
		bounds, err := lar.devices[0].Bounds()
		if err != nil {
			return image.Point{}, nil, err
		}
		return bounds, lar.roomPointsCombined, nil
	}
	dev := lar.devices[devNum]
	bounds, err := dev.Bounds()
	if err != nil {
		return image.Point{}, nil, err
	}
	return bounds, lar.roomPoints[devNum], nil
}

func (lar *LocationAwareLidar) NextColorDepthPair() (gocv.Mat, vision.DepthMap, error) {
	lar.update()

	// select device and sparse
	bounds, room, err := lar.roomToView()
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
	lar.roomPointsMu.Lock()
	defer lar.roomPointsMu.Unlock()
	room.DoNonZero(func(x, y int, _ float64) {
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

func (lar *LocationAwareLidar) getClientDeviceNum() int {
	lar.mu.Lock()
	defer lar.mu.Unlock()
	return lar.clientDeviceNum
}

func (lar *LocationAwareLidar) setClientDeviceNumber(num int) {
	lar.mu.Lock()
	defer lar.mu.Unlock()
	lar.clientDeviceNum = num
}

func (lar *LocationAwareLidar) handleData(data []byte, respondMsg func(msg string)) error {
	if bytes.HasPrefix(data, []byte("move: ")) {
		dir := moveDir(bytes.TrimPrefix(data, []byte("move: ")))
		if err := lar.base.(*fakeBase).Move(dir, lar.devices[0].Range()*lar.scaleDown); err != nil {
			return err
		}
		respondMsg(fmt.Sprintf("moved %q", dir))
		respondMsg(lar.base.(*fakeBase).String())
	} else if bytes.Equal(data, []byte("pos")) {
		respondMsg(lar.base.(*fakeBase).String())
	} else if bytes.Equal(data, []byte("lidar_stop")) {
		lar.devices[0].Stop()
		respondMsg("lidar stopped")
	} else if bytes.Equal(data, []byte("lidar_start")) {
		lar.devices[0].Start()
		respondMsg("lidar started")
	} else if bytes.HasPrefix(data, []byte("sv_lidar_seed ")) {
		seedStr := string(bytes.TrimPrefix(data, []byte("sv_lidar_seed ")))
		seed, err := strconv.ParseInt(seedStr, 10, 32)
		if err != nil {
			return err
		}
		if fake, ok := lar.devices[0].(*FakeLidar); ok {
			fake.seed = seed
		}
		respondMsg(seedStr)
	} else if bytes.HasPrefix(data, []byte("cl_lidar_device")) {
		lidarDeviceStr := string(bytes.TrimPrefix(data, []byte("cl_lidar_device")))
		if lidarDeviceStr == "" {
			var devicesStr string
			deviceNum := lar.getClientDeviceNum()
			if deviceNum == -1 {
				devicesStr = "[combined]"
			} else {
				devicesStr = "combined"
			}
			for i := range lar.devices {
				if deviceNum == i {
					devicesStr += fmt.Sprintf("\n[%d]", i)
				} else {
					devicesStr += fmt.Sprintf("\n%d", i)
				}
			}
			respondMsg(devicesStr)
			return nil
		}
		if lidarDeviceStr == "combined" {
			lar.setClientDeviceNumber(-1)
			return nil
		}
		lidarDeviceNum, err := strconv.ParseInt(lidarDeviceStr, 10, 32)
		if err != nil {
			return err
		}
		if lidarDeviceNum < 0 || lidarDeviceNum >= int64(len(lar.devices)) {
			return errors.New("invalid device")
		}
		lar.setClientDeviceNumber(int(lidarDeviceNum))
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
	if err := lar.base.(*fakeBase).Move(dir, lar.devices[0].Range()*lar.scaleDown); err != nil {
		return err
	}
	respondMsg(fmt.Sprintf("moved %q", dir))
	respondMsg(lar.base.(*fakeBase).String())
	return nil
}

func (lar *LocationAwareLidar) Close() {

}
