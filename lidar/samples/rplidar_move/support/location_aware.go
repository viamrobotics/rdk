package support

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"strconv"
	"sync"
	"time"

	"github.com/echolabsinc/robotcore/base"
	"github.com/echolabsinc/robotcore/lidar"
	"github.com/echolabsinc/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/james-bowman/sparse"
	"gocv.io/x/gocv"
)

type LocationAwareLidar struct {
	mu                 sync.Mutex
	Base               base.Base
	Devices            []lidar.Device
	clientDeviceNum    int
	RoomPointsCombined *sparse.DOK
	RoomPoints         []*sparse.DOK
	RoomPointsMu       *sync.Mutex
	ScaleDown          int
}

func (lar *LocationAwareLidar) Cull() {
	bounds, err := lar.Devices[0].Bounds()
	if err != nil {
		panic(err)
	}
	scaleDown := lar.ScaleDown
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

			basePosX := lar.Base.(*FakeBase).PosX
			basePosY := lar.Base.(*FakeBase).PosY
			minX := basePosX - bounds.X/2
			maxX := basePosX + bounds.X/2
			minY := basePosY - bounds.Y/2
			maxY := basePosY + bounds.Y/2

			// decrement observable area which will be refreshed by scans
			// within the area (assuming the lidar is active)
			func() {
				lar.RoomPointsMu.Lock()
				defer lar.RoomPointsMu.Unlock()
				lar.RoomPointsCombined.DoNonZero(func(x, y int, v float64) {
					if x < minX || x > maxX || y < minY || y > maxY {
						return
					}
					lar.RoomPointsCombined.Set(x, y, v-1)
				})
			}()
		}
	}()
}

func (lar *LocationAwareLidar) update() {
	basePosX := lar.Base.(*FakeBase).PosX
	basePosY := lar.Base.(*FakeBase).PosY

	for _, dev := range lar.Devices {
		if fake, ok := dev.(*FakeLidar); ok {
			fake.posX = basePosX
			fake.posY = basePosY
		}
	}
	// var allMeasurements []lidar.Measurements
	// for _, dev := range lar.Devices {
	measurements, err := lar.Devices[0].Scan()
	if err != nil {
		golog.Global.Debugw("bad scan", "error", err)
		return
	}
	// }

	// TODO(erd): combined
	dimX, dimY := lar.RoomPointsCombined.Dims()
	lar.RoomPointsMu.Lock()
	defer lar.RoomPointsMu.Unlock()
	for _, next := range measurements {
		x, y := next.Coords()
		detectedX := basePosX + int(x*float64(lar.ScaleDown))
		detectedY := basePosY + int(y*float64(lar.ScaleDown))
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
		lar.RoomPointsCombined.Set(detectedX, detectedY, 3) // TODO(erd): move to configurable
	}
}

func (lar *LocationAwareLidar) roomToView() (image.Point, *sparse.DOK, error) {
	devNum := lar.getClientDeviceNum()
	if devNum == -1 {
		// TODO(erd): combined
		bounds, err := lar.Devices[0].Bounds()
		if err != nil {
			return image.Point{}, nil, err
		}
		return bounds, lar.RoomPointsCombined, nil
	}
	dev := lar.Devices[devNum]
	bounds, err := dev.Bounds()
	if err != nil {
		return image.Point{}, nil, err
	}
	return bounds, lar.RoomPoints[devNum], nil
}

func (lar *LocationAwareLidar) NextColorDepthPair() (gocv.Mat, vision.DepthMap, error) {
	lar.update()

	// select device and sparse
	bounds, room, err := lar.roomToView()
	if err != nil {
		return gocv.Mat{}, vision.DepthMap{}, err
	}

	scaleDown := lar.ScaleDown
	bounds.X *= scaleDown
	bounds.Y *= scaleDown
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	out := gocv.NewMatWithSize(bounds.X, bounds.Y, gocv.MatTypeCV8UC3)

	var drawLine bool
	// drawLine = true

	basePosX := lar.Base.(*FakeBase).PosX
	basePosY := lar.Base.(*FakeBase).PosY
	minX := basePosX - bounds.X/2
	maxX := basePosX + bounds.X/2
	minY := basePosY - bounds.Y/2
	maxY := basePosY + bounds.Y/2

	// TODO(erd): any way to get a submatrix? may need to segment each one
	// if this starts going slower. fast as long as there are not many points
	lar.RoomPointsMu.Lock()
	defer lar.RoomPointsMu.Unlock()
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

func (lar *LocationAwareLidar) HandleData(data []byte, respondMsg func(msg string)) error {
	if bytes.HasPrefix(data, []byte("move: ")) {
		dir := MoveDir(bytes.TrimPrefix(data, []byte("move: ")))
		if err := lar.Base.(*FakeBase).Move(dir, lar.Devices[0].Range()*lar.ScaleDown); err != nil {
			return err
		}
		respondMsg(fmt.Sprintf("moved %q", dir))
		respondMsg(lar.Base.(*FakeBase).String())
	} else if bytes.Equal(data, []byte("pos")) {
		respondMsg(lar.Base.(*FakeBase).String())
	} else if bytes.Equal(data, []byte("lidar_stop")) {
		lar.Devices[0].Stop()
		respondMsg("lidar stopped")
	} else if bytes.Equal(data, []byte("lidar_start")) {
		lar.Devices[0].Start()
		respondMsg("lidar started")
	} else if bytes.HasPrefix(data, []byte("sv_lidar_seed ")) {
		seedStr := string(bytes.TrimPrefix(data, []byte("sv_lidar_seed ")))
		seed, err := strconv.ParseInt(seedStr, 10, 32)
		if err != nil {
			return err
		}
		if fake, ok := lar.Devices[0].(*FakeLidar); ok {
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
			for i := range lar.Devices {
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
		if lidarDeviceNum < 0 || lidarDeviceNum >= int64(len(lar.Devices)) {
			return errors.New("invalid device")
		}
		lar.setClientDeviceNumber(int(lidarDeviceNum))
	}
	return nil
}

func (lar *LocationAwareLidar) HandleClick(x, y, sX, sY int, respondMsg func(msg string)) error {
	centerX := sX / 2
	centerY := sX / 2
	var dir MoveDir
	if x < centerX {
		if y < centerY {
			dir = MoveDirUp
		} else {
			dir = MoveDirLeft
		}
	} else {
		if y < centerY {
			dir = MoveDirDown
		} else {
			dir = MoveDirRight
		}
	}
	if err := lar.Base.(*FakeBase).Move(dir, lar.Devices[0].Range()*lar.ScaleDown); err != nil {
		return err
	}
	respondMsg(fmt.Sprintf("moved %q", dir))
	respondMsg(lar.Base.(*FakeBase).String())
	return nil
}

func (lar *LocationAwareLidar) Close() {

}
