package slam

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/slam/v1"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/utils"

	"go.uber.org/multierr"

	"github.com/golang/geo/r2"
)

type LocationAwareRobot struct {
	started              bool
	baseDevice           api.Base
	baseDeviceWidthUnits float64
	baseOrientation      float64 // relative to map
	basePosX             float64
	basePosY             float64
	devBounds            []r2.Point
	maxBounds            r2.Point

	devices       []lidar.Device
	deviceOffsets []DeviceOffset

	unitsPerMeter   float64
	rootArea        *SquareArea
	presentViewArea *SquareArea

	compassSensor compass.Device

	clientZoom          float64
	clientClickMode     pb.ClickMode
	clientLidarViewMode pb.LidarViewMode

	activeWorkers   sync.WaitGroup
	serverMu        sync.Mutex
	closeCh         chan struct{}
	signalCloseOnce sync.Once

	updateInterval time.Duration
	cullInterval   int
	updateHook     func(culled bool)
	logger         golog.Logger
}

func NewLocationAwareRobot(
	ctx context.Context,
	baseDevice api.Base,
	area *SquareArea,
	devices []lidar.Device,
	deviceOffsets []DeviceOffset,
	compassSensor compass.Device,
	logger golog.Logger,
) (*LocationAwareRobot, error) {
	baseDeviceWidth, err := baseDevice.WidthMillis(ctx)
	if err != nil {
		return nil, err
	}

	var maxBoundsX, maxBoundsY float64
	devBounds := make([]r2.Point, 0, len(devices))
	for _, dev := range devices {
		bounds, err := dev.Bounds(ctx)
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

	presentViewArea, err := area.BlankCopy(logger)
	if err != nil {
		return nil, err
	}
	_, unitsPerMeter := area.Size()
	robot := &LocationAwareRobot{
		baseDevice: baseDevice,
		maxBounds:  r2.Point{maxBoundsX, maxBoundsY},
		devBounds:  devBounds,

		devices:       devices,
		deviceOffsets: deviceOffsets,

		unitsPerMeter:   unitsPerMeter,
		rootArea:        area,
		presentViewArea: presentViewArea,

		compassSensor: compassSensor,

		clientZoom:          1,
		clientClickMode:     pb.ClickMode_CLICK_MODE_INFO,
		clientLidarViewMode: pb.LidarViewMode_LIDAR_VIEW_MODE_STORED,
		closeCh:             make(chan struct{}),

		updateInterval: defaultUpdateInterval,
		cullInterval:   defaultCullInterval,
		logger:         logger,
	}
	robot.baseDeviceWidthUnits = robot.millimetersToMeasuredUnit(baseDeviceWidth)

	if err := robot.newPresentView(); err != nil {
		return nil, err
	}
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
	lar.activeWorkers.Add(1)
	lar.updateLoop()
	return nil
}

func (lar *LocationAwareRobot) SignalStop() {
	lar.signalCloseOnce.Do(func() {
		close(lar.closeCh)
	})
}

func (lar *LocationAwareRobot) Stop() error {
	lar.SignalStop()
	lar.activeWorkers.Wait()
	return lar.newPresentView()
}

func (lar *LocationAwareRobot) Close() error {
	return lar.Stop()
}

func (lar *LocationAwareRobot) String() string {
	return fmt.Sprintf("pos: (%v, %v)", lar.basePosX, lar.basePosY)
}

func (lar *LocationAwareRobot) Move(ctx context.Context, amountMillis *int, rotateTo *pb.Direction) (err error) {
	lar.serverMu.Lock()
	defer lar.serverMu.Unlock()

	move := api.Move{MillisPerSec: 0, Block: true}

	currentOrientation := lar.orientation()
	if rotateTo != nil {
		lar.logger.Debugw("request to rotate", "dir", *rotateTo)
		from := currentOrientation
		var to float64
		switch *rotateTo {
		case pb.Direction_DIRECTION_UP:
			to = 0
		case pb.Direction_DIRECTION_RIGHT:
			to = 90
		case pb.Direction_DIRECTION_DOWN:
			to = 180
		case pb.Direction_DIRECTION_LEFT:
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

	if amountMillis != nil {
		calcP, err := lar.calculateMove(newOrientation, *amountMillis)
		if err != nil {
			return err
		}
		newX, newY = calcP.X, calcP.Y
		move.DistanceMillis = *amountMillis
	}

	if newX != lar.basePosX || newY != lar.basePosY {
		if err := lar.detectObstacle(newX, newY, newOrientation, amountMillis); err != nil {
			return err
		}
	}

	defer func() {
		err = multierr.Combine(err, lar.newPresentView())
	}()
	spun, moved, err := api.DoMove(ctx, move, lar.baseDevice)
	if err != nil {
		return err
	}
	if spun != move.AngleDeg {
		newOrientation = utils.ModAngDeg(currentOrientation + spun)
	}
	if moved != move.DistanceMillis {
		// this can easily fail if newOrientation is not divisible by 90
		// can only work if you can be at any orientation. this is an accepted
		// failure that breaks the algorithm for now.
		calcP, err := lar.calculateMove(newOrientation, moved)
		if err != nil {
			return err
		}
		newX, newY = calcP.X, calcP.Y
	}
	lar.basePosX = newX
	lar.basePosY = newY
	lar.setOrientation(newOrientation)
	return nil
}

func (lar *LocationAwareRobot) detectObstacle(toX, toY float64, orientation float64, moveAmountMillis *int) error {
	moveOrientation := orientation
	if moveAmountMillis != nil && *moveAmountMillis < 0 {
		moveOrientation = math.Mod(orientation+180, 360)
	}
	moveRect, err := lar.moveRect(toX, toY, moveOrientation)
	if err != nil {
		return err
	}

	var collides bool
	lar.presentViewArea.Mutate(func(mutArea MutableArea) {
		mutArea.Iterate(func(x, y float64, v int) bool {
			if (moveRect.ContainsPoint(r2.Point{x, y})) {
				collides = true
				return false
			}
			return true
		})
	})
	if collides {
		return fmt.Errorf("cannot move to (%v,%v) via %f; would collide", toX, toY, moveOrientation)
	}
	return nil
}

func (lar *LocationAwareRobot) rotateTo(ctx context.Context, dir pb.Direction) error {
	return lar.Move(ctx, nil, &dir)
}

func (lar *LocationAwareRobot) millimetersToMeasuredUnit(amount int) float64 {
	/*
		amount millis
		_____________
		( millis/meter / (units/meter) )
		=>
		amount millis
		_____________
		millis*meter/meter*units
		=>
		amount millis
		_____________
		millis/units
		=>
		amount units
	*/

	amountNeg := amount < 0
	if amountNeg {
		amount *= -1
	}
	units := math.Ceil(float64(amount) / (1000 / lar.unitsPerMeter))
	if amountNeg {
		return units * -1
	}
	return units
}

func (lar *LocationAwareRobot) calculateMove(orientation float64, amountMillis int) (r2.Point, error) {
	newX := lar.basePosX
	newY := lar.basePosY

	amountScaled := lar.millimetersToMeasuredUnit(amountMillis)

	errMsg := fmt.Errorf("cannot move at orientation %f; stuck", orientation)
	quadLen := lar.rootArea.QuadrantLength()
	switch orientation {
	case 0:
		posY := lar.basePosY + amountScaled
		if posY < -quadLen || posY >= quadLen {
			return r2.Point{}, errMsg
		}
		newY = posY
	case 90:
		posX := lar.basePosX + amountScaled
		if posX < -quadLen || posX >= quadLen {
			return r2.Point{}, errMsg
		}
		newX = posX
	case 180:
		posY := lar.basePosY - amountScaled
		if posY < -quadLen || posY >= quadLen {
			return r2.Point{}, errMsg
		}
		newY = posY
	case 270:
		posX := lar.basePosX - amountScaled
		if posX < -quadLen || posX >= quadLen {
			return r2.Point{}, errMsg
		}
		newX = posX
	default:
		return r2.Point{}, fmt.Errorf("cannot move at orientation %f", orientation)
	}
	return r2.Point{newX, newY}, nil
}

const detectionBufferMillis = 150

// the move rect will always be ahead of the base itself even though the
// toX, toY are within the base rect since we move relative to the center.
func (lar *LocationAwareRobot) moveRect(toX, toY float64, orientation float64) (r2.Rect, error) {
	bufferScaled := lar.millimetersToMeasuredUnit(detectionBufferMillis)
	baseRect := lar.baseRect()
	distX, distY := math.Abs(lar.basePosX-toX), math.Abs(lar.basePosY-toY)
	var pathX0, pathY0, pathX1, pathY1 float64
	switch orientation {
	case 0:
		// top-left of base extended up
		pathX0, pathY0 = lar.basePosX-baseRect.Size().X/2, lar.basePosY+baseRect.Size().Y/2
		pathX1 = pathX0 + baseRect.Size().X
		pathY1 = pathY0 + distY + bufferScaled
	case 90:
		// top-right of base extended right
		pathX0, pathY0 = lar.basePosX+baseRect.Size().X/2, lar.basePosY-baseRect.Size().Y/2
		pathX1 = pathX0 + distX + bufferScaled
		pathY1 = pathY0 + baseRect.Size().Y
	case 180:
		// bottom-left of base extended down
		pathX0, pathY0 = lar.basePosX-baseRect.Size().X/2, lar.basePosY-baseRect.Size().Y/2
		pathX1 = pathX0 + baseRect.Size().X
		pathY1 = pathY0 - distY - bufferScaled
	case 270:
		// top-left of base extended left
		pathX0, pathY0 = lar.basePosX-baseRect.Size().X/2, lar.basePosY-baseRect.Size().Y/2
		pathX1 = pathX0 - distX - bufferScaled
		pathY1 = pathY0 + baseRect.Size().Y
	default:
		return r2.EmptyRect(), fmt.Errorf("bad orientation %f", orientation)
	}
	return r2.RectFromPoints(r2.Point{pathX0, pathY0}, r2.Point{pathX1, pathY1}), nil
}

// the rectangle is centered at the position of the base
func (lar *LocationAwareRobot) baseRect() r2.Rect {
	basePosX, basePosY := lar.basePos()

	return r2.RectFromPoints(
		r2.Point{basePosX - lar.baseDeviceWidthUnits/2, basePosY - lar.baseDeviceWidthUnits/2},
		r2.Point{basePosX + lar.baseDeviceWidthUnits/2, basePosY + lar.baseDeviceWidthUnits/2},
	)
}

// assumes appropriate locks are held
func (lar *LocationAwareRobot) newPresentView() error {
	// overlay presentView onto rootArea
	var err error
	lar.rootArea.Mutate(func(mutRoot MutableArea) {
		lar.presentViewArea.Mutate(func(mutPresent MutableArea) {
			mutPresent.Iterate(func(x, y float64, v int) bool {
				err = mutRoot.Set(x, y, v)
				return err == nil
			})
		})
	})
	if err != nil {
		return err
	}

	// allocate new presentView
	newArea, err := lar.presentViewArea.BlankCopy(lar.logger)
	if err != nil {
		return err
	}
	lar.presentViewArea = newArea
	return nil
}

func (lar *LocationAwareRobot) orientation() float64 {
	return lar.baseOrientation
}

func (lar *LocationAwareRobot) setOrientation(orientation float64) {
	lar.baseOrientation = orientation
}

func (lar *LocationAwareRobot) basePos() (float64, float64) {
	return lar.basePosX, lar.basePosY
}

func (lar *LocationAwareRobot) update(ctx context.Context) error {
	lar.serverMu.Lock()
	defer lar.serverMu.Unlock()

	basePosX, basePosY := lar.basePos()
	for _, dev := range lar.devices {
		if fake, ok := dev.(*fake.Lidar); ok {
			fake.SetPosition(r2.Point{basePosX, basePosY})
		}
	}
	return lar.scanAndStore(ctx, lar.devices, lar.presentViewArea)
}

func (lar *LocationAwareRobot) cull() error {
	lar.serverMu.Lock()
	defer lar.serverMu.Unlock()

	maxBoundsX := lar.maxBounds.X * lar.unitsPerMeter
	maxBoundsY := lar.maxBounds.Y * lar.unitsPerMeter

	basePosX, basePosY := lar.basePos()

	// calculate ideal visibility bounds
	areaMinX := basePosX - maxBoundsX/2
	areaMaxX := basePosX + maxBoundsX/2
	areaMinY := basePosY - maxBoundsY/2
	areaMaxY := basePosY + maxBoundsY/2

	// decrement observable area which will be refreshed by scans
	// within the area (assuming the lidar is active)
	var err error
	lar.presentViewArea.Mutate(func(mutArea MutableArea) {
		mutArea.Iterate(func(x, y float64, v int) bool {
			if x < areaMinX || x > areaMaxX || y < areaMinY || y > areaMaxY {
				return true
			}
			if v-1 == 0 {
				mutArea.Unset(x, y)
			} else {
				err = mutArea.Set(x, y, v-1)
				if err != nil {
					return false
				}
			}
			return true
		})
	})
	return err
}

var (
	defaultUpdateInterval = 33 * time.Millisecond
	defaultCullInterval   = 5
)

func (lar *LocationAwareRobot) updateLoop() {
	ticker := time.NewTicker(lar.updateInterval)
	count := 0
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	go func() {
		select {
		case <-cancelCtx.Done():
			return
		case <-lar.closeCh:
			cancelFunc()
		}
	}()
	go func() {
		defer lar.activeWorkers.Done()
		defer ticker.Stop()
		for {
			select {
			case <-lar.closeCh:
				cancelFunc()
				return
			default:
			}
			select {
			case <-lar.closeCh:
				cancelFunc()
				return
			case <-ticker.C:
			}
			func() {
				var culled bool
				if lar.updateHook != nil {
					defer func() {
						lar.updateHook(culled)
					}()
				}
				if err := lar.update(cancelCtx); err != nil {
					lar.logger.Debugw("error updating", "error", err)
				}
				if (count+1)%lar.cullInterval == 0 {
					if err := lar.cull(); err != nil {
						lar.logger.Debugw("error culling", "error", err)
					}
					culled = true
					count = 0
				} else {
					count++
				}
			}()
		}
	}()
}

const cullTTL = 3

func (lar *LocationAwareRobot) scanAndStore(ctx context.Context, devices []lidar.Device, area *SquareArea) error {
	basePosX, basePosY := lar.basePos()
	baseRect := lar.baseRect()

	allMeasurements := make([]lidar.Measurements, len(devices))
	for i, dev := range devices {
		measurements, err := dev.Scan(ctx, lidar.ScanOptions{})
		if err != nil {
			return fmt.Errorf("bad scan on device %d: %w", i, err)
		}
		allMeasurements[i] = measurements
	}

	quadLength := area.QuadrantLength()
	for i, measurements := range allMeasurements {
		var adjust bool
		var offsets DeviceOffset
		if i != 0 && i-1 < len(lar.deviceOffsets) {
			offsets = lar.deviceOffsets[i-1]
			adjust = true
		}
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
			detectedX := basePosX + offsets.DistanceX + x*lar.unitsPerMeter
			detectedY := basePosY + offsets.DistanceY + y*lar.unitsPerMeter
			if detectedX < -quadLength || detectedX >= quadLength {
				continue
			}
			if detectedY < -quadLength || detectedY >= quadLength {
				continue
			}
			if baseRect.ContainsPoint(r2.Point{detectedX, detectedY}) {
				continue
			}
			var err error
			area.Mutate(func(area MutableArea) {
				err = area.Set(detectedX, detectedY, cullTTL)
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (lar *LocationAwareRobot) areasToView() ([]lidar.Device, r2.Point, []*SquareArea) {
	return lar.devices, lar.maxBounds, []*SquareArea{lar.rootArea, lar.presentViewArea}
}

// relative to first device
type DeviceOffset struct {
	Angle                float64
	DistanceX, DistanceY float64
}

func (do *DeviceOffset) String() string {
	return fmt.Sprintf("%#v", do)
}

func (do *DeviceOffset) Set(val string) error {
	parsed, err := parseDevicOffsetFlag(val)
	if err != nil {
		return err
	}
	*do = parsed
	return nil
}

func (do *DeviceOffset) Get() interface{} {
	return do
}

// parseDevicOffsetFlag parses a lidar offset flag from command line arguments.
func parseDevicOffsetFlag(flag string) (DeviceOffset, error) {
	split := strings.Split(flag, ",")
	if len(split) != 3 {
		return DeviceOffset{}, errors.New("wrong offset format; use angle,x,y")
	}
	angle, err := strconv.ParseFloat(split[0], 64)
	if err != nil {
		return DeviceOffset{}, err
	}
	distX, err := strconv.ParseFloat(split[1], 64)
	if err != nil {
		return DeviceOffset{}, err
	}
	distY, err := strconv.ParseFloat(split[2], 64)
	if err != nil {
		return DeviceOffset{}, err
	}

	return DeviceOffset{
		Angle:     angle,
		DistanceX: distX,
		DistanceY: distY,
	}, nil
}
