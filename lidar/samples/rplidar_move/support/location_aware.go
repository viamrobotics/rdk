package support

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/echolabsinc/robotcore/base"
	"github.com/echolabsinc/robotcore/lidar"
	"github.com/echolabsinc/robotcore/vision"

	"github.com/edaniels/golog"
	"gocv.io/x/gocv"
)

// relative to first device
type DeviceOffset struct {
	Angle                float64
	DistanceX, DistanceY float64
}

type LocationAwareRobot struct {
	mu              sync.Mutex
	base            base.Base
	basePosX        int
	basePosY        int
	devices         []lidar.Device
	deviceOffsets   []DeviceOffset
	maxBounds       image.Point
	clientDeviceNum int
	clientZoom      float64
	room            *SquareRoom
	roomBounds      image.Point
	distinctRooms   []*SquareRoom
	orientations    []float64
}

func NewLocationAwareRobot(
	base base.Base,
	baseStart image.Point, // TODO(erd): should/could base itself be aware of location?
	devices []lidar.Device,
	deviceOffsets []DeviceOffset,
	room *SquareRoom,
	roomBounds image.Point,
) (*LocationAwareRobot, error) {
	roomSize, roomSizeScale := room.Size()
	distinctRooms := make([]*SquareRoom, 0, len(devices))
	for range devices {
		distinctRooms = append(distinctRooms, NewSquareRoom(roomSize, roomSizeScale))
	}

	var maxBoundsX, maxBoundsY int
	for _, dev := range devices {
		bounds, err := dev.Bounds()
		if err != nil {
			return nil, err
		}
		if bounds.X > maxBoundsX {
			maxBoundsX = bounds.X
		}
		if bounds.Y > maxBoundsY {
			maxBoundsY = bounds.Y
		}
	}

	return &LocationAwareRobot{
		base:            base,
		basePosX:        baseStart.X,
		basePosY:        baseStart.Y,
		devices:         devices,
		deviceOffsets:   deviceOffsets,
		maxBounds:       image.Point{maxBoundsX, maxBoundsY},
		room:            room,
		roomBounds:      roomBounds,
		distinctRooms:   distinctRooms,
		clientDeviceNum: -1,
		clientZoom:      1,
		orientations:    make([]float64, len(devices)),
	}, nil
}

func (lar *LocationAwareRobot) Start() {
	lar.startCulling()
}

func (lar *LocationAwareRobot) Stop() {

}

func (lar *LocationAwareRobot) basePos() (int, int) {
	return lar.basePosX, lar.basePosY
}

func (lar *LocationAwareRobot) basePosString() string {
	return fmt.Sprintf("pos: (%d, %d)", lar.basePosX, lar.basePosY)
}

func (lar *LocationAwareRobot) startCulling() {
	_, scaleDown := lar.room.Size()
	maxBoundsX := lar.maxBounds.X * scaleDown
	maxBoundsY := lar.maxBounds.Y * scaleDown

	cull := func() {
		// TODO(erd): not thread safe
		basePosX, basePosY := lar.basePos()

		// calculate ideal visibility bounds
		roomMinX := basePosX - maxBoundsX/2
		roomMaxX := basePosX + maxBoundsX/2
		roomMinY := basePosY - maxBoundsY/2
		roomMaxY := basePosY + maxBoundsY/2

		// decrement observable area which will be refreshed by scans
		// within the area (assuming the lidar is active)
		cullRoom := func(room *SquareRoom, minX, maxX, minY, maxY int) {
			room.Mutate(func(mutRoom MutableRoom) {
				mutRoom.DoNonZero(func(x, y int, v float64) {
					if x < minX || x > maxX || y < minY || y > maxY {
						return
					}
					mutRoom.Set(x, y, v-1)
				})
			})
		}

		cullRoom(lar.room, roomMinX, roomMaxX, roomMinY, roomMaxY)

		for i, room := range lar.distinctRooms {
			bounds, err := lar.devices[i].Bounds()
			if err != nil {
				panic(err)
			}
			bounds.X *= scaleDown
			bounds.Y *= scaleDown

			roomMinX := basePosX - bounds.X/2
			roomMaxX := basePosX + bounds.X/2
			roomMinY := basePosY - bounds.Y/2
			roomMaxY := basePosY + bounds.Y/2

			cullRoom(room, roomMinX, roomMaxX, roomMinY, roomMaxY)
		}
	}

	// TODO(erd): cancellation
	// TODO(erd): combined
	ticker := time.NewTicker(time.Second)
	go func() {
		defer ticker.Stop()
		for {
			<-ticker.C
			cull()
		}
	}()
}

func (lar *LocationAwareRobot) update() {
	basePosX, basePosY := lar.basePos()

	for _, dev := range lar.devices {
		if fake, ok := dev.(*FakeLidar); ok {
			fake.posX = basePosX
			fake.posY = basePosY
		}
	}
	allMeasurements := make([]lidar.Measurements, len(lar.devices))
	for i, dev := range lar.devices {
		measurements, err := dev.Scan()
		if err != nil {
			golog.Global.Debugw("bad scan", "device", i, "error", err)
			continue
		}
		allMeasurements[i] = measurements
	}

	roomSize, scaleDown := lar.room.Size()
	roomSize *= scaleDown
	for i, measurements := range allMeasurements {
		minAngle := math.Inf(1)
		var adjust bool
		var offsets DeviceOffset
		if i != 0 && i-1 < len(lar.deviceOffsets) {
			offsets = lar.deviceOffsets[i-1]
			adjust = true
		}
		// TODO(erd): better to just adjust in advance?
		for _, next := range measurements {
			angle := next.Angle()
			x, y := next.Coords()
			if adjust {
				angle += offsets.Angle
				angleRad := offsets.Angle * math.Pi / 180
				// rotate vector around base ccw
				newX := math.Cos(angleRad)*x - math.Sin(angleRad)*y
				newY := math.Sin(angleRad)*x + math.Cos(angleRad)*y
				x = newX
				y = newY
			}
			if angle < minAngle {
				minAngle = angle
			}
			detectedX := int(float64(basePosX) + offsets.DistanceX + x*float64(scaleDown))
			detectedY := int(float64(basePosY) + offsets.DistanceY + y*float64(scaleDown))
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
			lar.distinctRooms[i].Mutate(func(room MutableRoom) {
				room.Set(detectedX, detectedY, 3) // TODO(erd): move to configurable
			})
		}
		lar.orientations[i] = minAngle
	}
}

func (lar *LocationAwareRobot) roomToView() (image.Point, *SquareRoom, error) {
	devNum := lar.getClientDeviceNum()
	if devNum == -1 {
		return lar.maxBounds, lar.room, nil
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
	bounds.X = int(math.Ceil(float64(bounds.X) * float64(scaleDown) / lar.clientZoom))
	bounds.Y = int(math.Ceil(float64(bounds.Y) * float64(scaleDown) / lar.clientZoom))
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	out := gocv.NewMatWithSize(bounds.X, bounds.Y, gocv.MatTypeCV8UC3)

	var drawLine bool
	// drawLine = true

	basePosX, basePosY := lar.basePos()
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

	for i, orientation := range lar.orientations {
		if math.IsInf(orientation, 1) {
			continue
		}
		distance := 20.0
		x := distance * math.Cos(orientation*math.Pi/180)
		y := distance * math.Sin(orientation*math.Pi/180)
		relX := centerX + int(x)
		relY := centerY + int(y)
		p := image.Point{relX, relY}

		if i == 0 {
			gocv.ArrowedLine(&out, image.Point{centerX, centerY}, p, color.RGBA{G: 255}, 5)
		} else {
			gocv.ArrowedLine(&out, image.Point{centerX, centerY}, p, color.RGBA{B: 255}, 5)
		}
	}

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
	if bytes.HasPrefix(data, []byte("move: ")) {
		dir := direction(bytes.TrimPrefix(data, []byte("move: ")))
		if err := lar.rotateTo(dir); err != nil {
			return err
		}
		if err := lar.move(100); err != nil {
			return err
		}
		respondMsg(fmt.Sprintf("moved %q", dir))
		respondMsg(lar.basePosString())
	} else if bytes.Equal(data, []byte("pos")) {
		respondMsg(lar.basePosString())
	} else if bytes.HasPrefix(data, []byte("sv_device_offset ")) {
		offsetStr := string(bytes.TrimPrefix(data, []byte("sv_device_offset ")))
		offsetSplit := strings.SplitN(offsetStr, " ", 2)
		if len(offsetSplit) != 2 {
			return errors.New("malformed offset")
		}
		offsetNum, err := strconv.ParseInt(offsetSplit[0], 10, 64)
		if err != nil {
			return err
		}
		if offsetNum < 0 || int(offsetNum) > len(lar.deviceOffsets) {
			return errors.New("bad offset number")
		}
		split := strings.Split(offsetSplit[1], ",")
		if len(split) != 3 {
			return errors.New("offset format is angle,x,y")
		}
		angle, err := strconv.ParseFloat(split[0], 64)
		if err != nil {
			return err
		}
		distX, err := strconv.ParseFloat(split[1], 64)
		if err != nil {
			return err
		}
		distY, err := strconv.ParseFloat(split[2], 64)
		if err != nil {
			return err
		}
		lar.deviceOffsets[offsetNum] = DeviceOffset{angle, distX, distY}
		return nil
	} else if bytes.HasPrefix(data, []byte("sv_lidar_stop ")) {
		lidarDeviceStr := string(bytes.TrimPrefix(data, []byte("sv_lidar_stop ")))
		lidarDeviceNum, err := strconv.ParseInt(lidarDeviceStr, 10, 32)
		if err != nil {
			return err
		}
		if lidarDeviceNum < 0 || lidarDeviceNum >= int64(len(lar.devices)) {
			return errors.New("invalid device")
		}
		lar.devices[lidarDeviceNum].Stop()
		respondMsg(fmt.Sprintf("lidar %d stopped", lidarDeviceNum))
	} else if bytes.HasPrefix(data, []byte("sv_lidar_start ")) {
		lidarDeviceStr := string(bytes.TrimPrefix(data, []byte("sv_lidar_start ")))
		lidarDeviceNum, err := strconv.ParseInt(lidarDeviceStr, 10, 32)
		if err != nil {
			return err
		}
		if lidarDeviceNum < 0 || lidarDeviceNum >= int64(len(lar.devices)) {
			return errors.New("invalid device")
		}
		lar.devices[lidarDeviceNum].Start()
		respondMsg(fmt.Sprintf("lidar %d started", lidarDeviceNum))
	} else if bytes.HasPrefix(data, []byte("sv_lidar_seed ")) {
		seedStr := string(bytes.TrimPrefix(data, []byte("sv_lidar_seed ")))
		seed, err := strconv.ParseInt(seedStr, 10, 32)
		if err != nil {
			return err
		}
		if fake, ok := lar.devices[0].(*FakeLidar); ok {
			fake.Seed = seed
		}
		respondMsg(seedStr)
	} else if bytes.HasPrefix(data, []byte("cl_zoom ")) {
		zoomStr := string(bytes.TrimPrefix(data, []byte("cl_zoom ")))
		zoom, err := strconv.ParseFloat(zoomStr, 64)
		if err != nil {
			return err
		}
		if zoom < 1 {
			return errors.New("zoom must be >= 1")
		}
		lar.clientZoom = zoom
		respondMsg(zoomStr)
	} else if bytes.HasPrefix(data, []byte("cl_lidar_view")) {
		lidarDeviceStr := string(bytes.TrimSpace(bytes.TrimPrefix(data, []byte("cl_lidar_view"))))
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

type direction string

const (
	directionUp    = "up"
	directionRight = "right"
	directionDown  = "down"
	directionLeft  = "left"
)

func (lar *LocationAwareRobot) rotateTo(dir direction) error {
	currOrientation := lar.base.Orientation()
	var rotateBy int
	switch dir {
	case directionUp:
		rotateBy = currOrientation
	case directionRight:
		rotateBy = 90 - currOrientation
	case directionDown:
		rotateBy = 180 - currOrientation
	case directionLeft:
		rotateBy = 270 - currOrientation
	default:
		return fmt.Errorf("do not know how to rotate to absolute %q", dir)
	}
	return lar.base.Spin(rotateBy, 0, true)
}

func (lar *LocationAwareRobot) move(amount int) error {
	orientation := lar.base.Orientation()
	errMsg := fmt.Errorf("cannot move at orientation %d; stuck", orientation)
	newX := lar.basePosX
	newY := lar.basePosY
	switch orientation {
	case 0:
		if lar.basePosY-amount < 0 {
			return errMsg
		}
		golog.Global.Debugw("up", "amount", amount)
		newY = lar.basePosY - amount
	case 90:
		if lar.basePosX+amount >= lar.roomBounds.X {
			return errMsg
		}
		golog.Global.Debugw("right", "amount", amount)
		newX = lar.basePosX + amount
	case 180:
		if lar.basePosY+amount >= lar.roomBounds.Y {
			return errMsg
		}
		golog.Global.Debugw("down", "amount", amount)
		newY = lar.basePosY + amount
	case 270:
		if lar.basePosX-amount < 0 {
			return errMsg
		}
		golog.Global.Debugw("left", "amount", amount)
		newX = lar.basePosX - amount
	default:
		return fmt.Errorf("cannot move at orientation %d", orientation)
	}
	if err := lar.base.MoveStraight(amount*10, 0, true); err != nil {
		return err
	}
	lar.basePosX = newX
	lar.basePosY = newY
	return nil
}

func (lar *LocationAwareRobot) HandleClick(x, y, sX, sY int, respondMsg func(msg string)) error {
	centerX := sX / 2
	centerY := sX / 2

	var rotateTo direction
	if x < centerX {
		if y < centerY {
			rotateTo = directionUp
		} else {
			rotateTo = directionLeft
		}
	} else {
		if y < centerY {
			rotateTo = directionDown
		} else {
			rotateTo = directionRight
		}
	}

	if err := lar.rotateTo(rotateTo); err != nil {
		return err
	}
	if err := lar.move(100); err != nil {
		return err
	}
	respondMsg(fmt.Sprintf("moved %q", rotateTo))
	respondMsg(lar.basePosString())
	return nil
}

func (lar *LocationAwareRobot) Close() {

}
