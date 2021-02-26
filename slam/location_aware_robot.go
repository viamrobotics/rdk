package slam

import (
	"context"
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
	baseDevice      base.Device
	baseOrientation float64 // relative to map
	basePosX        int
	basePosY        int
	devBounds       []image.Point
	maxBounds       image.Point

	devices       []lidar.Device
	deviceOffsets []DeviceOffset
	orientations  []float64

	rootArea        *SquareArea
	presentViewArea *SquareArea
	distinctAreas   []*SquareArea // TODO(erd): this probably isn't that useful

	compassSensor compass.Device

	clientDeviceNum     int
	clientZoom          float64
	clientClickMode     string
	clientLidarViewMode string

	// TODO(erd): possibly complexify. this used to be more locks but it got hairy
	serverMu  sync.Mutex
	clientMu  sync.Mutex
	closeCh   chan struct{}
	closeOnce sync.Once
}

func NewLocationAwareRobot(
	baseDevice base.Device,
	baseStart image.Point,
	area *SquareArea,
	devices []lidar.Device,
	deviceOffsets []DeviceOffset,
	compassSensor compass.Device,
) (*LocationAwareRobot, error) {
	distinctAreas := make([]*SquareArea, 0, len(devices))
	for range devices {
		distinctAreas = append(distinctAreas, area.BlankCopy())
	}

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
		basePosX:   baseStart.X,
		basePosY:   baseStart.Y,
		maxBounds:  image.Point{maxBoundsX, maxBoundsY},
		devBounds:  devBounds,

		devices:       devices,
		deviceOffsets: deviceOffsets,
		orientations:  make([]float64, len(devices)),

		rootArea:        area,
		presentViewArea: area.BlankCopy(),
		distinctAreas:   distinctAreas,

		compassSensor: compassSensor,

		clientDeviceNum:     -1,
		clientZoom:          1,
		clientClickMode:     clientClickModeInfo,
		clientLidarViewMode: clientLidarViewModeStored,
		closeCh:             make(chan struct{}),
	}
	robot.newPresentView()
	return robot, nil
}

func (lar *LocationAwareRobot) Start() error {
	select {
	case <-lar.closeCh:
		return nil
	default:
	}
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

// assumes appropriate locks are held
func (lar *LocationAwareRobot) newPresentView() {
	// overlay presentView onto rootArea
	lar.rootArea.Mutate(func(mutRoot MutableArea) {
		lar.presentViewArea.Mutate(func(mutPresent MutableArea) {
			mutPresent.Iterate(func(x, y int, v float64) bool {
				mutRoot.Set(x, y, v)
				return true
			})
		})
	})

	// allocate new presentView
	areaSize, scaleTo := lar.presentViewArea.Size()
	lar.presentViewArea = NewSquareArea(areaSize, scaleTo)
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
		// top-left
		pathX0, pathY0 = lar.basePosX-baseRect.Dx()/2, lar.basePosY-baseRect.Dy()/2
		pathX1 = pathX0 + baseRect.Dx()
		pathY1 = int(math.Max(0, float64(pathY0-distY-detectionBuffer)))
	case 90:
		// top-right
		pathX0, pathY0 = lar.basePosX+baseRect.Dx()/2, lar.basePosY-baseRect.Dy()/2
		pathX1 = int(math.Min(math.MaxFloat64, float64(pathX0+distX+detectionBuffer)))
		pathY1 = pathY0 + baseRect.Dy()
	case 180:
		// bottom-left
		pathX0, pathY0 = lar.basePosX-baseRect.Dx()/2, lar.basePosY+baseRect.Dy()/2
		pathX1 = pathX0 + baseRect.Dx()
		pathY1 = int(math.Min(math.MaxFloat64, float64(pathY0+distY+detectionBuffer)))
	case 270:
		// top-left
		pathX0, pathY0 = lar.basePosX-baseRect.Dx()/2, lar.basePosY-baseRect.Dy()/2
		pathX1 = int(math.Max(0, float64(pathX0-distX-detectionBuffer)))
		pathY1 = pathY0 + baseRect.Dy()
	default:
		panic(fmt.Errorf("bad orientation %f", orientation))
	}
	return image.Rect(pathX0, pathY0, pathX1, pathY1)
}

func (lar *LocationAwareRobot) calculateMove(orientation float64, amount int) (image.Point, int, error) {
	newX := lar.basePosX
	newY := lar.basePosY

	errMsg := fmt.Errorf("cannot move at orientation %f; stuck", orientation)
	boundsX, boundsY := lar.rootArea.Dims()
	switch orientation {
	case 0:
		if lar.basePosY-amount < 0 {
			return image.Point{}, 0, errMsg
		}
		newY = lar.basePosY - amount
	case 90:
		if lar.basePosX+amount >= boundsX {
			return image.Point{}, 0, errMsg
		}
		newX = lar.basePosX + amount
	case 180:
		if lar.basePosY+amount >= boundsY {
			return image.Point{}, 0, errMsg
		}
		newY = lar.basePosY + amount
	case 270:
		if lar.basePosX-amount < 0 {
			return image.Point{}, 0, errMsg
		}
		newX = lar.basePosX - amount
	default:
		return image.Point{}, 0, fmt.Errorf("cannot move at orientation %f", orientation)
	}
	return image.Point{newX, newY}, amount, nil
}

func (lar *LocationAwareRobot) orientation() float64 {
	return lar.baseOrientation
}

func (lar *LocationAwareRobot) setOrientation(orientation float64) {
	lar.baseOrientation = orientation
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
	newOrientation := math.Mod(math.Mod((currentOrientation+move.AngleDeg), 360)+360, 360)

	newX := lar.basePosX
	newY := lar.basePosY

	if amount != nil {
		calcP, dist, err := lar.calculateMove(newOrientation, *amount)
		if err != nil {
			return err
		}
		newX, newY = calcP.X, calcP.Y
		move.DistanceMM = dist * 10 // TODO(erd): remove 10 in favor of scale
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
			mutArea.Iterate(func(x, y int, v float64) bool {
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

func (lar *LocationAwareRobot) basePos() (int, int) {
	return lar.basePosX, lar.basePosY
}

func (lar *LocationAwareRobot) String() string {
	return fmt.Sprintf("pos: (%d, %d)", lar.basePosX, lar.basePosY)
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
				mutArea.Iterate(func(x, y int, v float64) bool {
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

		for i, area := range lar.distinctAreas {
			bounds, err := lar.devices[i].Bounds(context.TODO())
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

const cullTTL = 3

func (lar *LocationAwareRobot) scanAndStore(devices []lidar.Device, area *SquareArea, distinctAreas ...*SquareArea) ([]float64, error) {
	basePosX, basePosY := lar.basePos()
	baseRect := lar.baseRect()

	allMeasurements := make([]lidar.Measurements, len(devices))
	for i, dev := range devices {
		measurements, err := dev.Scan(context.TODO(), lidar.ScanOptions{})
		if err != nil {
			return nil, fmt.Errorf("bad scan on device %d: %w", i, err)
		}
		allMeasurements[i] = measurements
	}

	areaSize, scaleDown := area.Size()
	areaSize *= scaleDown
	orientations := make([]float64, 0, len(allMeasurements))
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
			currentOrientation := lar.orientation()
			if adjust || currentOrientation != 0 {
				offset := (currentOrientation + offsets.Angle)
				angle += math.Mod(offset, 360)
				angleRad := utils.DegToRad(angle)
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
			if len(distinctAreas) > i {
				lar.distinctAreas[i].Mutate(func(area MutableArea) {
					area.Set(detectedX, detectedY, cullTTL)
				})
			}
		}
		orientations = append(orientations, minAngle)
	}
	return orientations, nil
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
		orientations, err := lar.scanAndStore(lar.devices, lar.presentViewArea, lar.distinctAreas...)
		if err != nil {
			golog.Global.Debugw("error scanning and storing", "error", err)
			return
		}
		lar.orientations = orientations
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

func (lar *LocationAwareRobot) areasToView() ([]lidar.Device, image.Point, []*SquareArea) {
	lar.serverMu.Lock()
	defer lar.serverMu.Unlock()
	devNum := lar.getClientDeviceNum()
	if devNum == -1 {
		return lar.devices, lar.maxBounds, []*SquareArea{lar.rootArea, lar.presentViewArea}
	}
	dev := lar.devices[devNum : devNum+1]
	bounds := lar.devBounds[devNum]
	return dev, bounds, []*SquareArea{lar.distinctAreas[devNum]}
}

func (lar *LocationAwareRobot) getClientDeviceNum() int {
	lar.clientMu.Lock()
	defer lar.clientMu.Unlock()
	return lar.clientDeviceNum
}

func (lar *LocationAwareRobot) setClientDeviceNumber(num int) {
	lar.clientMu.Lock()
	defer lar.clientMu.Unlock()
	lar.clientDeviceNum = num
}

func (lar *LocationAwareRobot) rotateTo(dir Direction) error {
	return lar.Move(nil, &dir)
}
