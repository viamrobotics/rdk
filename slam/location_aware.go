package slam

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
	"sync/atomic"
	"time"

	"github.com/echolabsinc/robotcore/base"
	"github.com/echolabsinc/robotcore/lidar"
	"github.com/echolabsinc/robotcore/robots/fake"
	"github.com/echolabsinc/robotcore/utils"

	"github.com/edaniels/golog"
	"gocv.io/x/gocv"
)

// relative to first device
type DeviceOffset struct {
	Angle                float64
	DistanceX, DistanceY float64
}

type LocationAwareRobot struct {
	isMoving         int32
	moveMu           sync.Mutex
	scanMu           sync.Mutex
	clientSettingsMu sync.Mutex
	base             base.Base
	baseOrientation  int // relative to map
	basePosX         int
	basePosY         int
	devices          []lidar.Device
	deviceOffsets    []DeviceOffset
	maxBounds        image.Point
	clientDeviceNum  int
	clientZoom       float64
	area             *SquareArea
	areaBounds       image.Point
	distinctAreas    []*SquareArea
	orientations     []float64
}

func NewLocationAwareRobot(
	base base.Base,
	baseStart image.Point, // TODO(erd): should/could base itself be aware of location?
	devices []lidar.Device,
	deviceOffsets []DeviceOffset,
	area *SquareArea,
	areaBounds image.Point,
) (*LocationAwareRobot, error) {
	areaSize, areaSizeScale := area.Size()
	distinctAreas := make([]*SquareArea, 0, len(devices))
	for range devices {
		distinctAreas = append(distinctAreas, NewSquareArea(areaSize, areaSizeScale))
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
		area:            area,
		areaBounds:      areaBounds,
		distinctAreas:   distinctAreas,
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
	_, scaleDown := lar.area.Size()
	maxBoundsX := lar.maxBounds.X * scaleDown
	maxBoundsY := lar.maxBounds.Y * scaleDown

	cull := func() {
		if atomic.LoadInt32(&lar.isMoving) == 1 {
			return
		}
		// TODO(erd): not thread safe
		basePosX, basePosY := lar.basePos()

		// calculate ideal visibility bounds
		areaMinX := basePosX - maxBoundsX/2
		areaMaxX := basePosX + maxBoundsX/2
		areaMinY := basePosY - maxBoundsY/2
		areaMaxY := basePosY + maxBoundsY/2

		// decrement observable area which will be refreshed by scans
		// within the area (assuming the lidar is active)
		cullArea := func(area *SquareArea, minX, maxX, minY, maxY int) {
			area.Mutate(func(mutArea MutableArea) {
				mutArea.DoNonZero(func(x, y int, v float64) {
					if x < minX || x > maxX || y < minY || y > maxY {
						return
					}
					mutArea.Set(x, y, v-1)
				})
			})
		}

		cullArea(lar.area, areaMinX, areaMaxX, areaMinY, areaMaxY)

		for i, area := range lar.distinctAreas {
			bounds, err := lar.devices[i].Bounds()
			if err != nil {
				panic(err)
			}
			bounds.X *= scaleDown
			bounds.Y *= scaleDown

			areaMinX := basePosX - bounds.X/2
			areaMaxX := basePosX + bounds.X/2
			areaMinY := basePosY - bounds.Y/2
			areaMaxY := basePosY + bounds.Y/2

			cullArea(area, areaMinX, areaMaxX, areaMinY, areaMaxY)
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
	if atomic.LoadInt32(&lar.isMoving) == 1 {
		return
	}
	basePosX, basePosY := lar.basePos()

	for _, dev := range lar.devices {
		if fake, ok := dev.(*fake.Lidar); ok {
			fake.SetPosition(image.Point{basePosX, basePosY})
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

	areaSize, scaleDown := lar.area.Size()
	areaSize *= scaleDown
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
				angleRad := utils.DegToRad(offsets.Angle)
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
			if detectedX < 0 || detectedX >= areaSize {
				continue
			}
			if detectedY < 0 || detectedY >= areaSize {
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
			lar.area.Mutate(func(area MutableArea) {
				area.Set(detectedX, detectedY, 3) // TODO(erd): move to configurable
			})
			lar.distinctAreas[i].Mutate(func(area MutableArea) {
				area.Set(detectedX, detectedY, 3) // TODO(erd): move to configurable
			})
		}
		lar.orientations[i] = minAngle
	}
}

func (lar *LocationAwareRobot) areaToView() (image.Point, *SquareArea, error) {
	devNum := lar.getClientDeviceNum()
	if devNum == -1 {
		return lar.maxBounds, lar.area, nil
	}
	dev := lar.devices[devNum]
	bounds, err := dev.Bounds()
	if err != nil {
		return image.Point{}, nil, err
	}
	return bounds, lar.distinctAreas[devNum], nil
}

func (lar *LocationAwareRobot) NextMat() (gocv.Mat, error) {
	lar.update()

	// select device and sparse
	bounds, area, err := lar.areaToView()
	if err != nil {
		return gocv.Mat{}, err
	}

	_, scaleDown := area.Size()
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
	area.Mutate(func(area MutableArea) {
		area.DoNonZero(func(x, y int, _ float64) {
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
		// Remember, our view is from x,y=0,0 at top left of matrix
		// 0°   -  (0,-1) // Up
		// 90°  -  (1, 0) // Right
		// 180° -  (0, 1) // Down
		// 270° -  (-1,0) // Left
		orientationRads := utils.DegToRad(orientation)
		x := distance * math.Sin(orientationRads)
		y := distance * -math.Cos(orientationRads)
		relX := centerX + int(x)
		relY := centerY + int(y)
		p := image.Point{relX, relY}

		if i == 0 {
			gocv.ArrowedLine(&out, image.Point{centerX, centerY}, p, color.RGBA{G: 255}, 5)
		} else {
			gocv.ArrowedLine(&out, image.Point{centerX, centerY}, p, color.RGBA{B: 255}, 5)
		}
	}

	return out, nil
}

func (lar *LocationAwareRobot) getClientDeviceNum() int {
	lar.clientSettingsMu.Lock()
	defer lar.clientSettingsMu.Unlock()
	return lar.clientDeviceNum
}

func (lar *LocationAwareRobot) setClientDeviceNumber(num int) {
	lar.clientSettingsMu.Lock()
	defer lar.clientSettingsMu.Unlock()
	lar.clientDeviceNum = num
}

func (lar *LocationAwareRobot) HandleData(data []byte, respondMsg func(msg string)) error {
	if bytes.HasPrefix(data, []byte("move: ")) {
		dir := direction(bytes.TrimPrefix(data, []byte("move: ")))
		amount := 100
		if err := lar.move(&amount, &dir); err != nil {
			return err
		}
		respondMsg(fmt.Sprintf("moved %q", dir))
		respondMsg(lar.basePosString())
	} else if bytes.HasPrefix(data, []byte("rotate_to ")) {
		dir := direction(bytes.TrimPrefix(data, []byte("rotate_to ")))
		if err := lar.rotateTo(dir); err != nil {
			return err
		}
		respondMsg(fmt.Sprintf("rotate to %q", dir))
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
		if fake, ok := lar.devices[0].(*fake.Lidar); ok {
			fake.SetSeed(seed)
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
	return lar.move(nil, &dir)
}

func (lar *LocationAwareRobot) move(amount *int, rotateTo *direction) error {
	lar.scanMu.Lock()
	atomic.StoreInt32(&lar.isMoving, 1)
	lar.scanMu.Unlock()
	defer atomic.StoreInt32(&lar.isMoving, 0)
	lar.moveMu.Lock()
	defer lar.moveMu.Unlock()

	move := base.Move{Speed: 0, Block: true}

	newX := lar.basePosX
	newY := lar.basePosY
	if amount != nil {
		actualAmount := *amount
		orientation := lar.baseOrientation
		errMsg := fmt.Errorf("cannot move at orientation %d; stuck", orientation)
		switch orientation {
		case 0:
			if lar.basePosY-actualAmount < 0 {
				return errMsg
			}
			golog.Global.Debugw("up", "amount", actualAmount)
			newY = lar.basePosY - actualAmount
		case 90:
			if lar.basePosX+actualAmount >= lar.areaBounds.X {
				return errMsg
			}
			golog.Global.Debugw("right", "amount", actualAmount)
			newX = lar.basePosX + actualAmount
		case 180:
			if lar.basePosY+actualAmount >= lar.areaBounds.Y {
				return errMsg
			}
			golog.Global.Debugw("down", "amount", actualAmount)
			newY = lar.basePosY + actualAmount
		case 270:
			if lar.basePosX-actualAmount < 0 {
				return errMsg
			}
			golog.Global.Debugw("left", "amount", actualAmount)
			newX = lar.basePosX - actualAmount
		default:
			return fmt.Errorf("cannot move at orientation %d", orientation)
		}
		move.DistanceMM = actualAmount * 10
	}

	if rotateTo != nil {
		from := lar.baseOrientation
		var to int
		switch *rotateTo {
		case directionUp:
			to = 0
		case directionRight:
			to = 90
		case directionDown:
			to = 180
		case directionLeft:
			to = 270
		default:
			return fmt.Errorf("do not know how to rotate to absolute %q", *rotateTo)
		}
		rotateBy := from - to
		if rotateBy != 180 && rotateBy != -180 {
			rotateBy = (rotateBy + 180) % 180
			if from > to {
				rotateBy *= -1
			}
		}
		move.AngleDeg = rotateBy
	}

	if _, _, err := base.DoMove(move, lar.base); err != nil {
		return err
	}
	lar.basePosX = newX
	lar.basePosY = newY
	lar.baseOrientation = (((lar.baseOrientation + move.AngleDeg) % 360) + 360) % 360
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

	amount := 100
	if err := lar.move(&amount, &rotateTo); err != nil {
		return err
	}
	respondMsg(fmt.Sprintf("moved %q", rotateTo))
	respondMsg(lar.basePosString())
	return nil
}

func (lar *LocationAwareRobot) Close() {

}
