package slam

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"sync"
	"time"

	"go.viam.com/robotcore/base"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
)

// relative to first device
type DeviceOffset struct {
	Angle                float64
	DistanceX, DistanceY float64
}

type LocationAwareRobot struct {
	started         bool
	baseDevice      base.Device
	baseOrientation float64 // relative to map
	basePosX        int
	basePosY        int
	devBounds       []image.Point
	maxBounds       image.Point

	devices       []lidar.Device
	deviceOffsets []DeviceOffset

	rootArea        *SquareArea
	presentViewArea *SquareArea

	compassSensor compass.Device

	clientZoom          float64
	clientClickMode     string
	clientLidarViewMode string

	serverMu  sync.Mutex
	closeCh   chan struct{}
	closeOnce sync.Once
}

func NewLocationAwareRobot(
	baseDevice base.Device,
	area *SquareArea,
	devices []lidar.Device,
	deviceOffsets []DeviceOffset,
	compassSensor compass.Device,
) (*LocationAwareRobot, error) {
	var maxBoundsX, maxBoundsY int
	devBounds := make([]image.Point, 0, len(devices))
	for _, dev := range devices {
		bounds, err := dev.Bounds(context.TODO())
		if err != nil {
			return nil, err
		}
		if bounds.X > maxBoundsX {
			maxBoundsX = bounds.X
		}
		if bounds.Y > maxBoundsY {
			maxBoundsY = bounds.Y
		}
		devBounds = append(devBounds, bounds)
	}

	robot := &LocationAwareRobot{
		baseDevice: baseDevice,
		maxBounds:  image.Point{maxBoundsX, maxBoundsY},
		devBounds:  devBounds,

		devices:       devices,
		deviceOffsets: deviceOffsets,

		rootArea:        area,
		presentViewArea: area.BlankCopy(),

		compassSensor: compassSensor,

		clientZoom:          1,
		clientClickMode:     clientClickModeInfo,
		clientLidarViewMode: clientLidarViewModeStored,
		closeCh:             make(chan struct{}),
	}
	robot.newPresentView()
	return robot, nil
}

var (
	ErrAlreadyStarted = errors.New("already started")
	ErrStopped        = errors.New("robot is stopped")
)

func (lar *LocationAwareRobot) Start() error {
	select {
	case <-lar.closeCh:
		return ErrStopped
	default:
	}
	lar.serverMu.Lock()
	if lar.started {
		lar.serverMu.Unlock()
		return ErrAlreadyStarted
	}
	lar.started = true
	lar.serverMu.Unlock()
	lar.cullLoop()
	lar.updateLoop()
	return nil
}

func (lar *LocationAwareRobot) Stop() {
	lar.closeOnce.Do(func() {
		close(lar.closeCh)
		lar.newPresentView()
	})
}

func (lar *LocationAwareRobot) Close() error {
	lar.Stop()
	return nil
}

func (lar *LocationAwareRobot) String() string {
	return fmt.Sprintf("pos: (%d, %d)", lar.basePosX, lar.basePosY)
}

func (lar *LocationAwareRobot) Move(amount *int, rotateTo *Direction) error {
	lar.serverMu.Lock()
	defer lar.serverMu.Unlock()

	move := base.Move{Speed: 0, Block: true}

	currentOrientation := lar.orientation()
	if rotateTo != nil {
		golog.Global.Debugw("request to rotate", "dir", *rotateTo)
		from := currentOrientation
		var to float64
		switch *rotateTo {
		case DirectionUp:
			to = 0
		case DirectionRight:
			to = 90
		case DirectionDown:
			to = 180
		case DirectionLeft:
			to = 270
		default:
			return fmt.Errorf("do not know how to rotate to absolute %q", *rotateTo)
		}
		var rotateBy float64
		if from > to {
			rotateBy = to - from
		} else {
			rotateBy = from - to
		}
		if rotateBy != 180 && rotateBy != -180 {
			rotateBy = math.Mod((rotateBy + 180), 180)
			if from > to {
				rotateBy *= -1
			}
		}
		move.AngleDeg = rotateBy
	}
	newOrientation := utils.ModAngDeg(currentOrientation + move.AngleDeg)

	newX := lar.basePosX
	newY := lar.basePosY

	if amount != nil {
		calcP, err := lar.calculateMove(newOrientation, *amount)
		if err != nil {
			return err
		}
		newX, newY = calcP.X, calcP.Y
		move.DistanceMM = *amount * 10 // TODO(erd): remove 10 in favor of scale
	}

	if newX != lar.basePosX || newY != lar.basePosY {
		// TODO(erd): refactor out to func
		// detect obstacle START

		// TODO(erd): use area of entity to determine collision
		// the lidar will give out around this distance so
		// we must make sure to not approach an area like this so as
		// to avoid the collision disappearing.

		moveOrientation := newOrientation
		if amount != nil && *amount < 0 {
			moveOrientation = math.Mod(newOrientation+180, 360)
		}
		moveRect := lar.moveRect(newX, newY, moveOrientation)

		var collides bool
		lar.presentViewArea.Mutate(func(mutArea MutableArea) {
			mutArea.Iterate(func(x, y, v int) bool {
				if (image.Point{x, y}.In(moveRect)) {
					collides = true
					return false
				}
				return true
			})
		})
		if collides {
			return fmt.Errorf("cannot move to (%d,%d) via %f; would collide", newX, newY, moveOrientation)
		}

		// detect obstacle END
	}

	defer lar.newPresentView() // TODO(erd): what about errors?
	if _, _, err := base.DoMove(move, lar.baseDevice); err != nil {
		return err
	}
	lar.basePosX = newX
	lar.basePosY = newY
	lar.setOrientation(newOrientation)
	return nil
}

func (lar *LocationAwareRobot) rotateTo(dir Direction) error {
	return lar.Move(nil, &dir)
}

func (lar *LocationAwareRobot) calculateMove(orientation float64, amount int) (image.Point, error) {
	newX := lar.basePosX
	newY := lar.basePosY

	errMsg := fmt.Errorf("cannot move at orientation %f; stuck", orientation)
	quadLen := lar.rootArea.QuadrantLength()
	switch orientation {
	case 0:
		posY := lar.basePosY + amount
		if posY < -quadLen || posY >= quadLen {
			return image.Point{}, errMsg
		}
		newY = posY
	case 90:
		posX := lar.basePosX + amount
		if posX < -quadLen || posX >= quadLen {
			return image.Point{}, errMsg
		}
		newX = posX
	case 180:
		posY := lar.basePosY - amount
		if posY < -quadLen || posY >= quadLen {
			return image.Point{}, errMsg
		}
		newY = posY
	case 270:
		posX := lar.basePosX - amount
		if posX < -quadLen || posX >= quadLen {
			return image.Point{}, errMsg
		}
		newX = posX
	default:
		return image.Point{}, fmt.Errorf("cannot move at orientation %f", orientation)
	}
	return image.Point{newX, newY}, nil
}

const detectionBuffer = 15

// the move rect will always be ahead of the base itself even though the
// toX, toY are within the base rect since we move relative to the center.
func (lar *LocationAwareRobot) moveRect(toX, toY int, orientation float64) image.Rectangle {
	baseRect := lar.baseRect()
	distX, distY := int(math.Abs(float64(lar.basePosX-toX))), int(math.Abs(float64(lar.basePosY-toY)))
	var pathX0, pathY0, pathX1, pathY1 int
	switch orientation {
	case 0:
		// top-left of base extended up
		pathX0, pathY0 = lar.basePosX-baseRect.Dx()/2, lar.basePosY-baseRect.Dy()/2
		pathX1 = pathX0 + baseRect.Dx()
		pathY1 = int(float64(pathY0 - distY - detectionBuffer))
	case 90:
		// top-right of base extended right
		pathX0, pathY0 = lar.basePosX+baseRect.Dx()/2, lar.basePosY-baseRect.Dy()/2
		pathX1 = int(float64(pathX0 + distX + detectionBuffer))
		pathY1 = pathY0 + baseRect.Dy()
	case 180:
		// bottom-left of base extended down
		pathX0, pathY0 = lar.basePosX-baseRect.Dx()/2, lar.basePosY+baseRect.Dy()/2
		pathX1 = pathX0 + baseRect.Dx()
		pathY1 = int(float64(pathY0 + distY + detectionBuffer))
	case 270:
		// top-left of base extended left
		pathX0, pathY0 = lar.basePosX-baseRect.Dx()/2, lar.basePosY-baseRect.Dy()/2
		pathX1 = int(float64(pathX0 - distX - detectionBuffer))
		pathY1 = pathY0 + baseRect.Dy()
	default:
		panic(fmt.Errorf("bad orientation %f", orientation))
	}
	return image.Rect(pathX0, pathY0, pathX1, pathY1)
}

// TODO(erd): config param
const baseWidthMeters = 0.60

// the rectangle is centered at the position of the base
func (lar *LocationAwareRobot) baseRect() image.Rectangle {
	_, scaleDown := lar.rootArea.Size()
	basePosX, basePosY := lar.basePos()

	baseWidthScaled := int(math.Ceil(baseWidthMeters * float64(scaleDown)))
	return image.Rect(
		basePosX-baseWidthScaled/2,
		basePosY-baseWidthScaled/2,
		basePosX+baseWidthScaled/2,
		basePosY+baseWidthScaled/2,
	)
}

// assumes appropriate locks are held
func (lar *LocationAwareRobot) newPresentView() {
	// overlay presentView onto rootArea
	lar.rootArea.Mutate(func(mutRoot MutableArea) {
		lar.presentViewArea.Mutate(func(mutPresent MutableArea) {
			mutPresent.Iterate(func(x, y, v int) bool {
				mutRoot.Set(x, y, v)
				return true
			})
		})
	})

	// allocate new presentView
	areaSize, scaleTo := lar.presentViewArea.Size()
	newArea, err := NewSquareArea(areaSize, scaleTo)
	if err != nil {
		panic(err) // should not happen
	}
	lar.presentViewArea = newArea
}

func (lar *LocationAwareRobot) orientation() float64 {
	return lar.baseOrientation
}

func (lar *LocationAwareRobot) setOrientation(orientation float64) {
	lar.baseOrientation = orientation
}

func (lar *LocationAwareRobot) basePos() (int, int) {
	return lar.basePosX, lar.basePosY
}

func (lar *LocationAwareRobot) cullLoop() {
	_, scaleDown := lar.rootArea.Size()
	maxBoundsX := lar.maxBounds.X * scaleDown
	maxBoundsY := lar.maxBounds.Y * scaleDown

	cull := func() {
		lar.serverMu.Lock()
		defer lar.serverMu.Unlock()

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
				mutArea.Iterate(func(x, y, v int) bool {
					if x < minX || x > maxX || y < minY || y > maxY {
						return true
					}
					if v-1 == 0 {
						mutArea.Unset(x, y)
					} else {
						mutArea.Set(x, y, v-1)
					}
					return true
				})
			})
		}

		cullArea(lar.presentViewArea, areaMinX, areaMaxX, areaMinY, areaMaxY)
	}

	ticker := time.NewTicker(time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-lar.closeCh:
				return
			default:
			}
			select {
			case <-lar.closeCh:
				return
			case <-ticker.C:
			}
			cull()
		}
	}()
}

func (lar *LocationAwareRobot) updateLoop() {
	update := func() {
		lar.serverMu.Lock()
		defer lar.serverMu.Unlock()

		basePosX, basePosY := lar.basePos()
		for _, dev := range lar.devices {
			if fake, ok := dev.(*fake.Lidar); ok {
				fake.SetPosition(image.Point{basePosX, basePosY})
			}
		}
		if err := lar.scanAndStore(lar.devices, lar.presentViewArea); err != nil {
			golog.Global.Debugw("error scanning and storing", "error", err)
			return
		}
	}
	ticker := time.NewTicker(33 * time.Millisecond)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-lar.closeCh:
				return
			default:
			}
			select {
			case <-lar.closeCh:
				return
			case <-ticker.C:
			}
			update()
		}
	}()
}

const cullTTL = 3

func (lar *LocationAwareRobot) scanAndStore(devices []lidar.Device, area *SquareArea) error {
	basePosX, basePosY := lar.basePos()
	baseRect := lar.baseRect()

	allMeasurements := make([]lidar.Measurements, len(devices))
	for i, dev := range devices {
		measurements, err := dev.Scan(context.TODO(), lidar.ScanOptions{})
		if err != nil {
			return fmt.Errorf("bad scan on device %d: %w", i, err)
		}
		allMeasurements[i] = measurements
	}

	_, scaleDown := area.Size()
	quadLength := area.QuadrantLength()
	for i, measurements := range allMeasurements {
		var adjust bool
		var offsets DeviceOffset
		if i != 0 && i-1 < len(lar.deviceOffsets) {
			offsets = lar.deviceOffsets[i-1]
			adjust = true
		}
		// TODO(erd): better to just adjust in advance?
		for _, next := range measurements {
			x, y := next.Coords()
			currentOrientation := lar.orientation()
			if adjust || currentOrientation != 0 {
				// rotate vector around base ccw (negative orientation + offset)
				offset := (-currentOrientation - offsets.Angle)
				rotateBy := utils.DegToRad(offset)
				newX := math.Cos(rotateBy)*x - math.Sin(rotateBy)*y
				newY := math.Sin(rotateBy)*x + math.Cos(rotateBy)*y
				x = newX
				y = newY
			}
			detectedX := int(float64(basePosX) + offsets.DistanceX + x*float64(scaleDown))
			detectedY := int(float64(basePosY) + offsets.DistanceY + y*float64(scaleDown))
			if detectedX < -quadLength || detectedX >= quadLength {
				continue
			}
			if detectedY < -quadLength || detectedY >= quadLength {
				continue
			}
			if (image.Point{detectedX, detectedY}).In(baseRect) {
				continue
			}
			// TODO(erd): should we also add here as a sense of permanency
			// Want to also combine this with occlusion, right. So if there's
			// a wall detected, and we're pretty confident it's staying there,
			// it being occluded should give it a low chance of it being removed.
			// Realistically once the bounds of a location are determined, most
			// environments would only have it deform over very long periods of time.
			// Probably longer than the lifetime of the application itself.
			area.Mutate(func(area MutableArea) {
				area.Set(detectedX, detectedY, cullTTL)
			})
		}
	}
	return nil
}

func (lar *LocationAwareRobot) areasToView() ([]lidar.Device, image.Point, []*SquareArea) {
	return lar.devices, lar.maxBounds, []*SquareArea{lar.rootArea, lar.presentViewArea}
}
