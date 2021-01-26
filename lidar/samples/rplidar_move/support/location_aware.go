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
	"gocv.io/x/gocv"
)

type LocationAwareRobot struct {
	mu              sync.Mutex
	base            base.Base
	devices         []lidar.Device
	clientDeviceNum int
	room            *SquareRoom
	distinctRooms   []*SquareRoom
}

func NewLocationAwareRobot(
	base base.Base,
	devices []lidar.Device,
	room *SquareRoom,
) *LocationAwareRobot {
	roomSize, roomSizeScale := room.Size()
	distinctRooms := make([]*SquareRoom, 0, len(devices))
	for range devices {
		distinctRooms = append(distinctRooms, NewSquareRoom(roomSize, roomSizeScale))
	}

	return &LocationAwareRobot{
		base:            base,
		devices:         devices,
		room:            room,
		distinctRooms:   distinctRooms,
		clientDeviceNum: -1,
	}
}

func (lar *LocationAwareRobot) Start() {
	lar.startCulling()
}

func (lar *LocationAwareRobot) Stop() {
	println("todo")
}

func (lar *LocationAwareRobot) startCulling() {
	bounds, err := lar.devices[0].Bounds()
	if err != nil {
		panic(err)
	}

	cull := func() {
		basePosX := lar.base.(*FakeBase).PosX
		basePosY := lar.base.(*FakeBase).PosY

		_, scaleDown := lar.room.Size()
		bounds.X *= scaleDown
		bounds.Y *= scaleDown

		// calculate ideal visibility bounds
		minX := basePosX - bounds.X/2
		maxX := basePosX + bounds.X/2
		minY := basePosY - bounds.Y/2
		maxY := basePosY + bounds.Y/2

		// decrement observable area which will be refreshed by scans
		// within the area (assuming the lidar is active)
		lar.room.Mutate(func(room MutableRoom) {
			room.DoNonZero(func(x, y int, v float64) {
				if x < minX || x > maxX || y < minY || y > maxY {
					return
				}
				room.Set(x, y, v-1)
			})
		})
	}

	// TODO(erd): cancellation
	// TODO(erd): combined
	ticker := time.NewTicker(time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
			}
			cull()
		}
	}()
}

func (lar *LocationAwareRobot) update() {
	basePosX := lar.base.(*FakeBase).PosX
	basePosY := lar.base.(*FakeBase).PosY

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
	roomSize, scaleDown := lar.room.Size()
	roomSize *= scaleDown
	for _, next := range measurements {
		x, y := next.Coords()
		detectedX := basePosX + int(x*float64(scaleDown))
		detectedY := basePosY + int(y*float64(scaleDown))
		if detectedX < 0 || detectedX >= roomSize {
			continue
		}
		if detectedY < 0 || detectedY >= roomSize {
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
		lar.room.Mutate(func(room MutableRoom) {
			room.Set(detectedX, detectedY, 3) // TODO(erd): move to configurable
		})
	}
}

func (lar *LocationAwareRobot) roomToView() (image.Point, *SquareRoom, error) {
	devNum := lar.getClientDeviceNum()
	if devNum == -1 {
		// TODO(erd): combined
		bounds, err := lar.devices[0].Bounds()
		if err != nil {
			return image.Point{}, nil, err
		}
		return bounds, lar.room, nil
	}
	dev := lar.devices[devNum]
	bounds, err := dev.Bounds()
	if err != nil {
		return image.Point{}, nil, err
	}
	return bounds, lar.distinctRooms[devNum], nil
}

func (lar *LocationAwareRobot) NextColorDepthPair() (gocv.Mat, vision.DepthMap, error) {
	lar.update()

	// select device and sparse
	bounds, room, err := lar.roomToView()
	if err != nil {
		return gocv.Mat{}, vision.DepthMap{}, err
	}

	_, scaleDown := room.Size()
	bounds.X *= scaleDown
	bounds.Y *= scaleDown
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	out := gocv.NewMatWithSize(bounds.X, bounds.Y, gocv.MatTypeCV8UC3)

	var drawLine bool
	// drawLine = true

	basePosX := lar.base.(*FakeBase).PosX
	basePosY := lar.base.(*FakeBase).PosY
	minX := basePosX - bounds.X/2
	maxX := basePosX + bounds.X/2
	minY := basePosY - bounds.Y/2
	maxY := basePosY + bounds.Y/2

	// TODO(erd): any way to get a submatrix? may need to segment each one
	// if this starts going slower. fast as long as there are not many points
	room.Mutate(func(room MutableRoom) {
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
	})

	return out, vision.DepthMap{}, nil
}

func (lar *LocationAwareRobot) getClientDeviceNum() int {
	lar.mu.Lock()
	defer lar.mu.Unlock()
	return lar.clientDeviceNum
}

func (lar *LocationAwareRobot) setClientDeviceNumber(num int) {
	lar.mu.Lock()
	defer lar.mu.Unlock()
	lar.clientDeviceNum = num
}

func (lar *LocationAwareRobot) HandleData(data []byte, respondMsg func(msg string)) error {
	_, scaleDown := lar.room.Size()
	if bytes.HasPrefix(data, []byte("move: ")) {
		dir := MoveDir(bytes.TrimPrefix(data, []byte("move: ")))
		if err := lar.base.(*FakeBase).Move(dir, lar.devices[0].Range()*scaleDown); err != nil {
			return err
		}
		respondMsg(fmt.Sprintf("moved %q", dir))
		respondMsg(lar.base.(*FakeBase).String())
	} else if bytes.Equal(data, []byte("pos")) {
		respondMsg(lar.base.(*FakeBase).String())
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

func (lar *LocationAwareRobot) HandleClick(x, y, sX, sY int, respondMsg func(msg string)) error {
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
	_, scaleDown := lar.room.Size()
	if err := lar.base.(*FakeBase).Move(dir, lar.devices[0].Range()*scaleDown); err != nil {
		return err
	}
	respondMsg(fmt.Sprintf("moved %q", dir))
	respondMsg(lar.base.(*FakeBase).String())
	return nil
}

func (lar *LocationAwareRobot) Close() {

}
