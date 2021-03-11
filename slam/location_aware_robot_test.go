package slam

import (
	"context"
	"errors"
	"fmt"
	"image"
	"testing"
	"time"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/gostream"
	"github.com/edaniels/test"
)

type testHarness struct {
	bot          *LocationAwareRobot
	baseDevice   *fake.Base
	area         *SquareArea
	lidarDev     *inject.LidarDevice
	cmdReg       gostream.CommandRegistry
	scansPerCull int
}

func (th *testHarness) ResetPos() {
	th.bot.basePosX = 0
	th.bot.basePosY = 0
}

func newTestHarness(t *testing.T) *testHarness {
	return newTestHarnessWithLidarAndSize(t, nil, 10, 100)
}

func newTestHarnessWithSize(t *testing.T, meters, scale int) *testHarness {
	return newTestHarnessWithLidarAndSize(t, nil, meters, scale)
}

func newTestHarnessWithLidarAndSize(t *testing.T, lidarDev lidar.Device, meters, scale int) *testHarness {
	baseDevice := &fake.Base{}
	area, err := NewSquareArea(meters, scale)
	test.That(t, err, test.ShouldBeNil)
	injectLidarDev := &inject.LidarDevice{Device: lidarDev}
	if lidarDev == nil {
		injectLidarDev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
			return image.Point{meters, meters}, nil
		}
	}

	larBot, err := NewLocationAwareRobot(
		context.Background(),
		baseDevice,
		area,
		[]lidar.Device{injectLidarDev},
		nil,
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	// changing this will modify test output
	larBot.updateInterval = time.Millisecond
	larBot.cullInterval = 3
	scansPerCull := larBot.cullInterval / int(larBot.updateInterval/time.Millisecond)

	cmdReg := gostream.NewCommandRegistry()
	larBot.RegisterCommands(cmdReg)

	return &testHarness{
		larBot,
		baseDevice,
		area,
		injectLidarDev,
		cmdReg,
		scansPerCull,
	}
}

func TestNewLocationAwareRobot(t *testing.T) {
	baseDevice := &fake.Base{}
	area, err := NewSquareArea(10, 100)
	test.That(t, err, test.ShouldBeNil)
	injectLidarDev := &inject.LidarDevice{}
	injectLidarDev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
		return image.Point{10, 10}, nil
	}

	_, err = NewLocationAwareRobot(
		context.Background(),
		baseDevice,
		area,
		[]lidar.Device{injectLidarDev},
		nil,
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	err1 := errors.New("whoops")
	injectLidarDev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
		return image.Point{}, err1
	}

	_, err = NewLocationAwareRobot(
		context.Background(),
		baseDevice,
		area,
		[]lidar.Device{injectLidarDev},
		nil,
		nil,
	)
	test.That(t, err, test.ShouldWrap, err1)

	injectLidarDev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
		return image.Point{5, 5}, nil
	}
	injectBase := &inject.Base{Base: baseDevice}
	injectBase.WidthMillisFunc = func(ctx context.Context) (int, error) {
		return 0, err1
	}

	_, err = NewLocationAwareRobot(
		context.Background(),
		injectBase,
		area,
		[]lidar.Device{injectLidarDev},
		nil,
		nil,
	)
	test.That(t, err, test.ShouldWrap, err1)
}

func TestRobotString(t *testing.T) {
	th := newTestHarness(t)
	test.That(t, th.bot.String(), test.ShouldContainSubstring, fmt.Sprintf("(%d, %d)", 0, 0))
	th.bot.basePosX = 20
	th.bot.basePosY = 40
	test.That(t, th.bot.String(), test.ShouldContainSubstring, fmt.Sprintf("(%d, %d)", 20, 40))
}

func TestRobotStartStopClose(t *testing.T) {
	th := newTestHarness(t)
	test.That(t, th.bot.Start(), test.ShouldBeNil)
	test.That(t, th.bot.Start(), test.ShouldEqual, ErrAlreadyStarted)
	test.That(t, th.bot.Stop(), test.ShouldBeNil)
	test.That(t, th.bot.Start(), test.ShouldEqual, ErrStopped)

	th = newTestHarness(t)
	test.That(t, th.bot.Start(), test.ShouldBeNil)
	test.That(t, th.bot.Close(), test.ShouldBeNil)
	test.That(t, th.bot.Start(), test.ShouldEqual, ErrStopped)
	test.That(t, th.bot.Stop(), test.ShouldBeNil)
	test.That(t, th.bot.Start(), test.ShouldEqual, ErrStopped)

	th = newTestHarness(t)
	test.That(t, th.bot.Start(), test.ShouldBeNil)
	th.bot.SignalStop()
	test.That(t, th.bot.Start(), test.ShouldEqual, ErrStopped)
	test.That(t, th.bot.Stop(), test.ShouldBeNil)
	test.That(t, th.bot.Start(), test.ShouldEqual, ErrStopped)
}

func TestMove(t *testing.T) {
	intPtr := func(v int) *int {
		return &v
	}
	dirPtr := func(v Direction) *Direction {
		return &v
	}

	for _, tc := range []struct {
		desc             string
		amountMillis     *int
		rotateTo         *Direction
		err              string
		deltaX           int
		deltaY           int
		deltaOrientation float64
		pre              func(th *testHarness)
	}{
		{"do nothing", nil, nil, "", 0, 0, 0, nil},
		{"rotate up", nil, dirPtr(DirectionUp), "", 0, 0, 0, nil},
		{"rotate down", nil, dirPtr(DirectionDown), "", 0, 0, 180, nil},
		{"rotate left", nil, dirPtr(DirectionLeft), "", 0, 0, 270, nil},
		{"rotate right", nil, dirPtr(DirectionRight), "", 0, 0, 90, nil},
		{"move forward", intPtr(100), nil, "", 0, 10, 0, nil},
		{"move backward", intPtr(-100), nil, "", 0, -10, 0, nil},
		{"move forward too far", intPtr(10000), nil, "stuck", 0, 0, 0, nil},
		{"move backward too far", intPtr(-10000), nil, "stuck", 0, 0, 0, nil},
		{"rotate down and move forward", intPtr(200), dirPtr(DirectionDown), "", 0, -20, 180, nil},
		{"rotate right and move forward", intPtr(200), dirPtr(DirectionRight), "", 20, 0, 90, nil},
		{"rotate left and move forward", intPtr(200), dirPtr(DirectionLeft), "", -20, 0, 270, nil},
		{"rotate down and move backward", intPtr(-200), dirPtr(DirectionDown), "", 0, 20, 180, nil},
		{"rotate right and move backward", intPtr(-200), dirPtr(DirectionRight), "", -20, 0, 90, nil},
		{"rotate left and move backward", intPtr(-200), dirPtr(DirectionLeft), "", 20, 0, 270, nil},
		{"rotate down and move forward too far", intPtr(20000), dirPtr(DirectionDown), "stuck", 0, 0, 0, nil},
		{"rotate right and move forward too far", intPtr(20000), dirPtr(DirectionRight), "stuck", 0, 0, 0, nil},
		{"rotate left and move forward too far", intPtr(20000), dirPtr(DirectionLeft), "stuck", 0, 0, 0, nil},
		{"cannot collide up", intPtr(200), dirPtr(DirectionUp), "collide", 0, 0, 0, func(th *testHarness) {
			th.bot.presentViewArea.Mutate(func(area MutableArea) {
				test.That(t, area.Set(th.bot.basePosX, th.bot.basePosY+((th.bot.baseDeviceWidthScaled/2)+1), 3), test.ShouldBeNil)
			})
		}},
		{"cannot collide down", intPtr(200), dirPtr(DirectionDown), "collide", 0, 0, 0, func(th *testHarness) {
			th.bot.presentViewArea.Mutate(func(area MutableArea) {
				test.That(t, area.Set(th.bot.basePosX, th.bot.basePosY-((th.bot.baseDeviceWidthScaled/2)+1), 3), test.ShouldBeNil)
			})
		}},
		{"cannot collide left", intPtr(200), dirPtr(DirectionLeft), "collide", 0, 0, 0, func(th *testHarness) {
			th.bot.presentViewArea.Mutate(func(area MutableArea) {
				test.That(t, area.Set(th.bot.basePosX-((th.bot.baseDeviceWidthScaled/2)+1), th.bot.basePosY, 3), test.ShouldBeNil)
			})
		}},
		{"cannot collide right", intPtr(200), dirPtr(DirectionRight), "collide", 0, 0, 0, func(th *testHarness) {
			th.bot.presentViewArea.Mutate(func(area MutableArea) {
				test.That(t, area.Set(th.bot.basePosX+((th.bot.baseDeviceWidthScaled/2)+1), th.bot.basePosY, 3), test.ShouldBeNil)
			})
		}},
		{"unknown direction", intPtr(200), dirPtr("ouch"), "do not know how", 0, 0, 0, nil},
		{"moving fails", intPtr(200), dirPtr(DirectionRight), "whoops", 0, 0, 0, func(th *testHarness) {
			injectBase := &inject.Base{}
			injectBase.WidthMillisFunc = func(ctx context.Context) (int, error) {
				return 600, nil
			}
			th.bot.baseDevice = injectBase
			injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, speed int, block bool) error {
				return errors.New("whoops")
			}
			injectBase.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
				return errors.New("whoops")
			}
		}},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			th := newTestHarness(t)
			if tc.pre != nil {
				tc.pre(th)
			}
			origX := th.bot.basePosX
			origY := th.bot.basePosX
			origOrientation := th.bot.orientation()
			err := th.bot.Move(context.Background(), tc.amountMillis, tc.rotateTo)
			if tc.err != "" {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.err)
			} else {
				test.That(t, err, test.ShouldBeNil)
			}
			test.That(t, th.bot.basePosX-origX, test.ShouldEqual, tc.deltaX)
			test.That(t, th.bot.basePosY-origY, test.ShouldEqual, tc.deltaY)
			test.That(t, th.bot.orientation()-origOrientation, test.ShouldEqual, tc.deltaOrientation)
		})
	}
}

func TestRobotOrientation(t *testing.T) {
	th := newTestHarness(t)
	test.That(t, th.bot.orientation(), test.ShouldEqual, 0)
	th.bot.setOrientation(5)
	test.That(t, th.bot.orientation(), test.ShouldEqual, 5)
}

func TestRobotBasePos(t *testing.T) {
	th := newTestHarness(t)
	x, y := th.bot.basePos()
	test.That(t, x, test.ShouldEqual, 0)
	test.That(t, y, test.ShouldEqual, 0)

	th.bot.basePosX = 20
	th.bot.basePosY = -1555
	x, y = th.bot.basePos()
	test.That(t, x, test.ShouldEqual, 20)
	test.That(t, y, test.ShouldEqual, -1555)
}

func TestRobotAreasToView(t *testing.T) {
	th := newTestHarness(t)
	devices, bounds, areas := th.bot.areasToView()
	test.That(t, devices, test.ShouldResemble, th.bot.devices)
	test.That(t, bounds, test.ShouldResemble, th.bot.maxBounds)
	expected := map[*SquareArea]struct{}{
		th.bot.rootArea:        {},
		th.bot.presentViewArea: {},
	}
	for _, a := range areas {
		delete(expected, a)
	}
	test.That(t, expected, test.ShouldBeEmpty)
}

func TestRobotMillimetersToScaledUnit(t *testing.T) {
	for i, tc := range []struct {
		millis   int
		expected int
	}{
		{0, 0},
		{1, 1},
		{5, 1},
		{10, 1},
		{11, 2},
		{20, 2},
		{-1, -1},
		{-5, -1},
		{-10, -1},
		{-11, -2},
		{-20, -2},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			th := newTestHarness(t)
			test.That(t, th.bot.millimetersToScaledUnit(tc.millis), test.ShouldEqual, tc.expected)
		})
	}
}

func TestRobotCalculateMove(t *testing.T) {
	for i, tc := range []struct {
		basePosX     int
		basePosY     int
		orientation  float64
		amountMillis int
		err          string
		newCoords    image.Point
	}{
		{-500, -500, 0, 0, "", image.Point{-500, -500}},
		{-500, -500, 0, 1, "", image.Point{-500, -499}},
		{-500, -500, 45, 1, "orientation", image.Point{}},
		{-500, -500, 90, 1, "", image.Point{-499, -500}},
		{-500, -500, 180, 1, "stuck", image.Point{}},
		{-500, -500, 270, 1, "stuck", image.Point{}},
		{0, 0, 0, 1, "", image.Point{0, 1}},
		{0, 0, 90, 1, "", image.Point{1, 0}},
		{0, 0, 180, 1, "", image.Point{0, -1}},
		{0, 0, 270, 1, "", image.Point{-1, 0}},
		{0, 0, 0, 100, "", image.Point{0, 10}},
		{0, 0, 90, 100, "", image.Point{10, 0}},
		{0, 0, 180, 100, "", image.Point{0, -10}},
		{0, 0, 270, 100, "", image.Point{-10, 0}},
		{499, 499, 0, 1, "stuck", image.Point{}},
		{499, 499, 45, 1, "orientation", image.Point{}},
		{499, 499, 90, 1, "stuck", image.Point{}},
		{499, 499, 180, 1, "", image.Point{499, 498}},
		{499, 499, 270, 1, "", image.Point{498, 499}},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			th := newTestHarness(t)
			th.bot.basePosX = tc.basePosX
			th.bot.basePosY = tc.basePosY
			newCoords, err := th.bot.calculateMove(tc.orientation, tc.amountMillis)
			if tc.err != "" {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.err)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, newCoords, test.ShouldResemble, tc.newCoords)
		})
	}
}

func TestBaseRect(t *testing.T) {
	for i, tc := range []struct {
		basePosX int
		basePosY int
		rect     image.Rectangle
	}{
		{0, 0, image.Rect(-30, -30, 30, 30)},
		{-50, -50, image.Rect(-80, -80, -20, -20)},
		{40, 13, image.Rect(10, -17, 70, 43)},
		{49, 49, image.Rect(19, 19, 79, 79)},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			th := newTestHarness(t)
			th.bot.basePosX = tc.basePosX
			th.bot.basePosY = tc.basePosY
			test.That(t, th.bot.baseRect(), test.ShouldResemble, tc.rect)
		})
	}
}

func TestMoveRect(t *testing.T) {
	for i, tc := range []struct {
		basePosX    int
		basePosY    int
		toX         int
		toY         int
		orientation float64
		rect        image.Rectangle
		err         string
	}{
		{0, 0, 0, 0, 0, image.Rect(-30, 30, 30, 45), ""},
		{0, 0, 0, 0, 90, image.Rect(30, -30, 45, 30), ""},
		{0, 0, 0, 0, 180, image.Rect(-30, -45, 30, -30), ""},
		{0, 0, 0, 0, 270, image.Rect(-45, -30, -30, 30), ""},
		{23, 54, 23, 54, 0, image.Rect(-7, 84, 53, 99), ""},
		{23, 54, 23, 54, 90, image.Rect(53, 24, 68, 84), ""},
		{23, 54, 23, 54, 180, image.Rect(-7, 9, 53, 24), ""},
		{23, 54, 23, 54, 270, image.Rect(-22, 24, -7, 84), ""},
		{49, 48, 50, 32, 0, image.Rect(19, 78, 79, 109), ""},
		{49, 48, 64, 50, 90, image.Rect(79, 18, 109, 78), ""},
		{49, 48, 50, 64, 180, image.Rect(19, -13, 79, 18), ""},
		{49, 48, 32, 50, 270, image.Rect(-13, 18, 19, 78), ""},
		{49, 48, 32, 50, 271, image.Rect(-13, 18, 19, 78), "bad orientation"},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			th := newTestHarness(t)
			th.bot.basePosX = tc.basePosX
			th.bot.basePosY = tc.basePosY
			rect, err := th.bot.moveRect(tc.toX, tc.toY, tc.orientation)
			if tc.err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, rect, test.ShouldResemble, tc.rect)
				return
			}
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.err)
		})
	}
}

func TestNewPresentView(t *testing.T) {
	th := newTestHarness(t)

	// verify no data
	rootCount := 0
	presentCount := 0
	th.bot.rootArea.Mutate(func(area MutableArea) {
		area.Iterate(func(x, y, v int) bool {
			rootCount++
			return true
		})
	})
	th.bot.presentViewArea.Mutate(func(area MutableArea) {
		area.Iterate(func(x, y, v int) bool {
			presentCount++
			return true
		})
	})
	test.That(t, rootCount, test.ShouldEqual, 0)
	test.That(t, presentCount, test.ShouldEqual, 0)
	test.That(t, th.bot.newPresentView(), test.ShouldBeNil)

	th.bot.rootArea.Mutate(func(area MutableArea) {
		area.Iterate(func(x, y, v int) bool {
			rootCount++
			return true
		})
	})
	th.bot.presentViewArea.Mutate(func(area MutableArea) {
		area.Iterate(func(x, y, v int) bool {
			presentCount++
			return true
		})
	})
	test.That(t, rootCount, test.ShouldEqual, 0)
	test.That(t, presentCount, test.ShouldEqual, 0)

	// add some points
	th.bot.presentViewArea.Mutate(func(area MutableArea) {
		test.That(t, area.Set(1, 2, 5), test.ShouldBeNil)
		test.That(t, area.Set(0, 4, 1), test.ShouldBeNil)
		test.That(t, area.Set(7, 6, 1), test.ShouldBeNil)
		test.That(t, area.Set(1, 1, 0), test.ShouldBeNil)
		test.That(t, area.Set(0, 0, 1), test.ShouldBeNil)
		test.That(t, area.Set(32, 49, 2), test.ShouldBeNil)
	})
	test.That(t, th.bot.newPresentView(), test.ShouldBeNil)

	th.bot.rootArea.Mutate(func(area MutableArea) {
		area.Iterate(func(x, y, v int) bool {
			rootCount++
			return true
		})
	})
	th.bot.presentViewArea.Mutate(func(area MutableArea) {
		area.Iterate(func(x, y, v int) bool {
			presentCount++
			return true
		})
	})
	test.That(t, rootCount, test.ShouldEqual, 6)
	test.That(t, presentCount, test.ShouldEqual, 0)

	expected := map[string]struct{}{
		"1,2,5":   {},
		"0,4,1":   {},
		"7,6,1":   {},
		"1,1,0":   {},
		"0,0,1":   {},
		"32,49,2": {},
	}
	th.bot.rootArea.Mutate(func(area MutableArea) {
		area.Iterate(func(x, y, v int) bool {
			delete(expected, fmt.Sprintf("%d,%d,%d", x, y, v))
			return true
		})
	})
	test.That(t, expected, test.ShouldBeEmpty)
}

func TestScanAndStore(t *testing.T) {
	testUpdate(t, true)
}

func testUpdate(t *testing.T, internal bool) {
	th := newTestHarness(t)
	area, err := th.area.BlankCopy()
	test.That(t, err, test.ShouldBeNil)
	device := &inject.LidarDevice{}
	err1 := errors.New("whoops")
	device.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
		return nil, err1
	}
	err = th.bot.scanAndStore(context.Background(), []lidar.Device{device}, area)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "bad scan")
	test.That(t, err, test.ShouldWrap, err1)

	for _, tc := range []struct {
		desc            string
		basePosX        int
		basePosY        int
		orientation     float64
		deviceOffsets   []DeviceOffset
		allMeasurements []lidar.Measurements
		err             string
		validateArea    func(t *testing.T, area *SquareArea)
	}{
		{"base case", 0, 0, 0, nil, nil, "", func(t *testing.T, area *SquareArea) {
			count := 0
			area.Mutate(func(area MutableArea) {
				area.Iterate(func(x, y, v int) bool {
					count++
					return true
				})
			})
			test.That(t, count, test.ShouldEqual, 0)
		}},
		{"same measurement at orientation 0", 0, 0, 0, nil, []lidar.Measurements{
			{
				lidar.NewMeasurement(0, 1),
			},
		}, "", func(t *testing.T, area *SquareArea) {
			count := 0
			expected := map[string]struct{}{
				"0,100,3": {},
			}
			area.Mutate(func(area MutableArea) {
				area.Iterate(func(x, y, v int) bool {
					count++
					delete(expected, fmt.Sprintf("%d,%d,%d", x, y, v))
					return true
				})
			})
			test.That(t, count, test.ShouldEqual, 1)
			test.That(t, expected, test.ShouldBeEmpty)
		}},
		{"same measurement at orientation 90", 0, 0, 90, nil, []lidar.Measurements{
			{
				lidar.NewMeasurement(270, 1),
			},
		}, "", func(t *testing.T, area *SquareArea) {
			count := 0
			expected := map[string]struct{}{
				"0,100,3": {},
			}
			area.Mutate(func(area MutableArea) {
				area.Iterate(func(x, y, v int) bool {
					count++
					delete(expected, fmt.Sprintf("%d,%d,%d", x, y, v))
					return true
				})
			})
			test.That(t, count, test.ShouldEqual, 1)
			test.That(t, expected, test.ShouldBeEmpty)
		}},
		{"same measurement at orientation 180", 0, 0, 180, nil, []lidar.Measurements{
			{
				lidar.NewMeasurement(180, 1),
			},
		}, "", func(t *testing.T, area *SquareArea) {
			count := 0
			expected := map[string]struct{}{
				"0,100,3": {},
			}
			area.Mutate(func(area MutableArea) {
				area.Iterate(func(x, y, v int) bool {
					count++
					delete(expected, fmt.Sprintf("%d,%d,%d", x, y, v))
					return true
				})
			})
			test.That(t, count, test.ShouldEqual, 1)
			test.That(t, expected, test.ShouldBeEmpty)
		}},
		{"same measurement at orientation 270", 0, 0, 270, nil, []lidar.Measurements{
			{
				lidar.NewMeasurement(90, 1),
			},
		}, "", func(t *testing.T, area *SquareArea) {
			count := 0
			expected := map[string]struct{}{
				"0,100,3": {},
			}
			area.Mutate(func(area MutableArea) {
				area.Iterate(func(x, y, v int) bool {
					count++
					delete(expected, fmt.Sprintf("%d,%d,%d", x, y, v))
					return true
				})
			})
			test.That(t, count, test.ShouldEqual, 1)
			test.That(t, expected, test.ShouldBeEmpty)
		}},
		{"one device with some measurements at center", 0, 0, 0, nil, []lidar.Measurements{
			{
				lidar.NewMeasurement(0, 3),
				lidar.NewMeasurement(21, 4),
				lidar.NewMeasurement(64, 1),
				lidar.NewMeasurement(90, .2), // within base
				lidar.NewMeasurement(132, 2),
				lidar.NewMeasurement(290, 4),
				lidar.NewMeasurement(180, 10), // out of range
				lidar.NewMeasurement(90, 10),  // out of range
			},
		}, "", func(t *testing.T, area *SquareArea) {
			count := 0
			expected := map[string]struct{}{
				"0,300,3":    {},
				"143,373,3":  {},
				"89,43,3":    {},
				"148,-133,3": {},
				"-375,136,3": {},
			}
			area.Mutate(func(area MutableArea) {
				area.Iterate(func(x, y, v int) bool {
					count++
					delete(expected, fmt.Sprintf("%d,%d,%d", x, y, v))
					return true
				})
			})
			test.That(t, count, test.ShouldEqual, 5)
			test.That(t, expected, test.ShouldBeEmpty)
		}},
		{"one device with some measurements near bounds", -500, -500, 0, nil, []lidar.Measurements{
			{
				lidar.NewMeasurement(0, 3),
				lidar.NewMeasurement(21, 4),
				lidar.NewMeasurement(64, 1),
				lidar.NewMeasurement(90, .2),  // within base
				lidar.NewMeasurement(132, 2),  // out of range
				lidar.NewMeasurement(290, 4),  // out of range
				lidar.NewMeasurement(180, 10), // out of range
				lidar.NewMeasurement(90, 10),  // out of range
			},
		}, "", func(t *testing.T, area *SquareArea) {
			count := 0
			expected := map[string]struct{}{
				"-500,-200,3": {},
				"-356,-126,3": {},
				"-410,-456,3": {},
			}
			area.Mutate(func(area MutableArea) {
				area.Iterate(func(x, y, v int) bool {
					count++
					delete(expected, fmt.Sprintf("%d,%d,%d", x, y, v))
					return true
				})
			})
			test.That(t, count, test.ShouldEqual, 3)
			test.That(t, expected, test.ShouldBeEmpty)
		}},
		{"multiple devices with some measurements at center", 0, 0, 0, nil, []lidar.Measurements{
			{
				lidar.NewMeasurement(0, 3),
				lidar.NewMeasurement(21, 4),
				lidar.NewMeasurement(64, 1),
				lidar.NewMeasurement(90, .2), // within base
				lidar.NewMeasurement(132, 2),
				lidar.NewMeasurement(290, 4),
				lidar.NewMeasurement(180, 10), // out of range
				lidar.NewMeasurement(90, 10),  // out of range
			},
			{
				lidar.NewMeasurement(2, 3),
				lidar.NewMeasurement(23, 4),
				lidar.NewMeasurement(66, 1),
				lidar.NewMeasurement(92, .2), // within base
				lidar.NewMeasurement(135, 2),
				lidar.NewMeasurement(292, 4),
				lidar.NewMeasurement(182, 10), // out of range
				lidar.NewMeasurement(92, 10),  // out of range
			},
		}, "", func(t *testing.T, area *SquareArea) {
			count := 0
			expected := map[string]struct{}{
				"0,300,3":    {},
				"143,373,3":  {},
				"89,43,3":    {},
				"148,-133,3": {},
				"-370,149,3": {},
				"10,299,3":   {},
				"156,368,3":  {},
				"91,40,3":    {},
				"141,-141,3": {},
				"-375,136,3": {},
			}
			area.Mutate(func(area MutableArea) {
				area.Iterate(func(x, y, v int) bool {
					count++
					delete(expected, fmt.Sprintf("%d,%d,%d", x, y, v))
					return true
				})
			})
			test.That(t, count, test.ShouldEqual, 10)
			test.That(t, expected, test.ShouldBeEmpty)
		}},
		{"multiple devices with missing measurements at center", 0, 0, 0, nil, []lidar.Measurements{
			{
				lidar.NewMeasurement(0, 3),
				lidar.NewMeasurement(21, 4),
				lidar.NewMeasurement(64, 1),
				lidar.NewMeasurement(90, .2), // within base
				lidar.NewMeasurement(132, 2),
				lidar.NewMeasurement(290, 4),
				lidar.NewMeasurement(180, 10), // out of range
				lidar.NewMeasurement(90, 10),  // out of range
			},
			{},
		}, "", func(t *testing.T, area *SquareArea) {
			count := 0
			expected := map[string]struct{}{
				"0,300,3":    {},
				"143,373,3":  {},
				"89,43,3":    {},
				"148,-133,3": {},
				"-375,136,3": {},
			}
			area.Mutate(func(area MutableArea) {
				area.Iterate(func(x, y, v int) bool {
					count++
					delete(expected, fmt.Sprintf("%d,%d,%d", x, y, v))
					return true
				})
			})
			test.That(t, count, test.ShouldEqual, 5)
			test.That(t, expected, test.ShouldBeEmpty)
		}},
		{"multiple devices with some measurements at center and offsets", 0, 0, 0,
			[]DeviceOffset{
				{45, 0, 0},
			},
			[]lidar.Measurements{
				{
					lidar.NewMeasurement(0, 3),
					lidar.NewMeasurement(21, 4),
					lidar.NewMeasurement(64, 1),
					lidar.NewMeasurement(90, .2), // within base
					lidar.NewMeasurement(132, 2),
					lidar.NewMeasurement(290, 4),
					lidar.NewMeasurement(180, 10), // out of range
					lidar.NewMeasurement(90, 10),  // out of range
				},
				{
					lidar.NewMeasurement(315, 3),
					lidar.NewMeasurement(336, 4),
					lidar.NewMeasurement(379, 1),
					lidar.NewMeasurement(45, .2), // within base
					lidar.NewMeasurement(87, 2),
					lidar.NewMeasurement(245, 4),
					lidar.NewMeasurement(135, 10), // out of range
					lidar.NewMeasurement(45, 10),  // out of range
				},
			}, "", func(t *testing.T, area *SquareArea) {
				count := 0
				expected := map[string]struct{}{
					"0,300,3":    {},
					"143,373,3":  {},
					"89,43,3":    {},
					"148,-133,3": {},
					"-375,136,3": {},
				}
				area.Mutate(func(area MutableArea) {
					area.Iterate(func(x, y, v int) bool {
						count++
						delete(expected, fmt.Sprintf("%d,%d,%d", x, y, v))
						return true
					})
				})
				test.That(t, count, test.ShouldEqual, 5)
				test.That(t, expected, test.ShouldBeEmpty)
			}},
		{"multiple devices with some measurements at center and shifted offsets", 0, 0, 0,
			[]DeviceOffset{
				{45, 10, 20},
			},
			[]lidar.Measurements{
				{
					lidar.NewMeasurement(0, 3),
					lidar.NewMeasurement(21, 4),
					lidar.NewMeasurement(64, 1),
					lidar.NewMeasurement(90, .2), // within base
					lidar.NewMeasurement(132, 2),
					lidar.NewMeasurement(290, 4),
					lidar.NewMeasurement(180, 10), // out of range
					lidar.NewMeasurement(90, 10),  // out of range
				},
				{
					lidar.NewMeasurement(315, 3),
					lidar.NewMeasurement(336, 4),
					lidar.NewMeasurement(379, 1),
					lidar.NewMeasurement(45, .2), // not within base now
					lidar.NewMeasurement(87, 2),
					lidar.NewMeasurement(245, 4),
					lidar.NewMeasurement(135, 10), // out of range
					lidar.NewMeasurement(45, 10),  // out of range
				},
			}, "", func(t *testing.T, area *SquareArea) {
				count := 0
				expected := map[string]struct{}{
					"0,300,3":    {},
					"143,373,3":  {},
					"89,43,3":    {},
					"148,-133,3": {},
					"-375,136,3": {},

					"9,320,3":    {},
					"153,393,3":  {},
					"99,63,3":    {},
					"30,20,3":    {},
					"158,-113,3": {},
					"-365,156,3": {},
				}
				area.Mutate(func(area MutableArea) {
					area.Iterate(func(x, y, v int) bool {
						count++
						delete(expected, fmt.Sprintf("%d,%d,%d", x, y, v))
						return true
					})
				})
				test.That(t, count, test.ShouldEqual, 11)
				test.That(t, expected, test.ShouldBeEmpty)
			}},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			th := newTestHarness(t)
			th.bot.basePosX = tc.basePosX
			th.bot.basePosY = tc.basePosY
			th.bot.setOrientation(tc.orientation)
			th.bot.deviceOffsets = tc.deviceOffsets
			devices := make([]lidar.Device, 0, len(tc.allMeasurements))
			for _, measurements := range tc.allMeasurements {
				mCopy := measurements
				device := &inject.LidarDevice{}
				device.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
					return mCopy, nil
				}
				devices = append(devices, device)
			}
			area, err := th.area.BlankCopy()
			test.That(t, err, test.ShouldBeNil)
			if internal {
				err = th.bot.scanAndStore(context.Background(), devices, area)
			} else {
				th.bot.devices = devices
				err = th.bot.update(context.Background())
			}
			if tc.err != "" {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.err)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			if tc.validateArea == nil {
				return
			}
			if internal {
				tc.validateArea(t, area)
			} else {
				tc.validateArea(t, th.bot.presentViewArea)
			}
		})
	}
}

func TestRobotUpdate(t *testing.T) {
	testUpdate(t, false)
}

func TestRobotCull(t *testing.T) {
	t.Run("should only cull anything within range in the present view", func(t *testing.T) {
		th := newTestHarness(t)
		th.bot.rootArea.Mutate(func(area MutableArea) {
			test.That(t, area.Set(1, 2, 3), test.ShouldBeNil)
			test.That(t, area.Set(0, 4, 3), test.ShouldBeNil)
			test.That(t, area.Set(7, 6, 4), test.ShouldBeNil)
		})
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			test.That(t, area.Set(1, 2, 3), test.ShouldBeNil)
			test.That(t, area.Set(0, 4, 3), test.ShouldBeNil)
			test.That(t, area.Set(7, 6, 4), test.ShouldBeNil)
			test.That(t, area.Set(1, 1, 3), test.ShouldBeNil)
			test.That(t, area.Set(0, 0, 3), test.ShouldBeNil)
			test.That(t, area.Set(2, 49, 3), test.ShouldBeNil)
			test.That(t, area.Set(-35, -4, 3), test.ShouldBeNil)
		})

		th.bot.cull()
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 7)
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			test.That(t, area.At(1, 2), test.ShouldEqual, 2)
			test.That(t, area.At(0, 4), test.ShouldEqual, 2)
			test.That(t, area.At(7, 6), test.ShouldEqual, 3)
			test.That(t, area.At(1, 1), test.ShouldEqual, 2)
			test.That(t, area.At(0, 0), test.ShouldEqual, 2)
			test.That(t, area.At(2, 49), test.ShouldEqual, 2)
			test.That(t, area.At(-35, -4), test.ShouldEqual, 2)
		})

		th.bot.cull()
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 7)
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			test.That(t, area.At(1, 2), test.ShouldEqual, 1)
			test.That(t, area.At(0, 4), test.ShouldEqual, 1)
			test.That(t, area.At(7, 6), test.ShouldEqual, 2)
			test.That(t, area.At(1, 1), test.ShouldEqual, 1)
			test.That(t, area.At(0, 0), test.ShouldEqual, 1)
			test.That(t, area.At(2, 49), test.ShouldEqual, 1)
			test.That(t, area.At(-35, -4), test.ShouldEqual, 1)
		})

		th.bot.cull()
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 1)
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			test.That(t, area.At(7, 6), test.ShouldEqual, 1)
		})
		th.bot.cull()
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 0)

		th.bot.maxBounds = image.Point{5, 5}
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			test.That(t, area.Set(1, 2, 3), test.ShouldBeNil)
			test.That(t, area.Set(0, 4, 3), test.ShouldBeNil)
			test.That(t, area.Set(7, 6, 4), test.ShouldBeNil)
			test.That(t, area.Set(1, 1, 3), test.ShouldBeNil)
			test.That(t, area.Set(0, 0, 3), test.ShouldBeNil)
			test.That(t, area.Set(20, 499, 3), test.ShouldBeNil)
			test.That(t, area.Set(-350, -40, 3), test.ShouldBeNil)
		})
		th.bot.cull()
		th.bot.cull()
		th.bot.cull()
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 3)
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			test.That(t, area.At(7, 6), test.ShouldEqual, 1)
			test.That(t, area.At(20, 499), test.ShouldEqual, 3)
			test.That(t, area.At(-350, -40), test.ShouldEqual, 3)
		})
		th.bot.cull()
		th.bot.cull()
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 2)
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			test.That(t, area.At(20, 499), test.ShouldEqual, 3)
			test.That(t, area.At(-350, -40), test.ShouldEqual, 3)
		})

		th.bot.rootArea.Mutate(func(area MutableArea) {
			test.That(t, area.At(1, 2), test.ShouldEqual, 3)
			test.That(t, area.At(0, 4), test.ShouldEqual, 3)
			test.That(t, area.At(7, 6), test.ShouldEqual, 4)
		})
	})
}

func TestRobotActive(t *testing.T) {
	waitForCulls := func(th *testHarness, num int, onDone func()) <-chan struct{} {
		ch := make(chan struct{})
		count := 0
		th.bot.updateHook = func(culled bool) {
			if !culled {
				return
			}
			if count+1 == num {
				onDone()
				close(ch)
				return
			}
			count++
		}
		return ch
	}

	t.Run("still base should continue to update and cull the present view", func(t *testing.T) {
		th := newTestHarness(t)
		waitFor := 3
		waitCh := waitForCulls(th, waitFor, func() {
			th.bot.SignalStop()
		})

		test.That(t, th.bot.Start(), test.ShouldBeNil)

		expectedNumMeasurements := waitFor * th.scansPerCull
		measurments := []*lidar.Measurement{
			lidar.NewMeasurement(0, .1),
			lidar.NewMeasurement(10, .2),
			lidar.NewMeasurement(20, .3),
			lidar.NewMeasurement(30, .4),
			lidar.NewMeasurement(40, .5),
			lidar.NewMeasurement(50, .6),
			lidar.NewMeasurement(60, .7),
			lidar.NewMeasurement(70, .8),
			lidar.NewMeasurement(80, .9),
		}
		test.That(t, measurments, test.ShouldHaveLength, expectedNumMeasurements)
		count := 0
		th.lidarDev.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
			m := measurments[count]
			count++
			return lidar.Measurements{m}, nil
		}
		<-waitCh
		test.That(t, th.bot.Stop(), test.ShouldBeNil)

		test.That(t, th.bot.rootArea.PointCloud().Size(), test.ShouldEqual, 6)
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 0)
		expected := map[string]struct{}{
			"20,34,1": {},
			"32,38,1": {},
			"45,38,1": {},
			"60,35,2": {},
			"75,27,2": {},
			"88,15,2": {},
		}
		actual := map[string]struct{}{}
		th.bot.rootArea.Mutate(func(area MutableArea) {
			area.Iterate(func(x, y, v int) bool {
				actual[fmt.Sprintf("%d,%d,%d", x, y, v)] = struct{}{}
				return true
			})
		})
		test.That(t, actual, test.ShouldHaveLength, (cullTTL-1)*th.scansPerCull)
		test.That(t, actual, test.ShouldResemble, expected)
	})

	t.Run("moving base should update root view over time", func(t *testing.T) {
		th := newTestHarness(t)
		waitFor := 3
		count := 0
		swap := make(chan struct{})
		waitForCulls(th, waitFor, func() {
			count = 0
			swap <- struct{}{}
			<-swap
		})

		test.That(t, th.bot.Start(), test.ShouldBeNil)

		expectedNumMeasurements := waitFor * th.scansPerCull
		measurments := []*lidar.Measurement{
			lidar.NewMeasurement(0, .1),
			lidar.NewMeasurement(10, .2),
			lidar.NewMeasurement(20, .3),
			lidar.NewMeasurement(30, .4),
			lidar.NewMeasurement(40, .5),
			lidar.NewMeasurement(50, .6),
			lidar.NewMeasurement(60, .7),
			lidar.NewMeasurement(70, .8),
			lidar.NewMeasurement(80, .9),
		}
		test.That(t, measurments, test.ShouldHaveLength, expectedNumMeasurements)
		th.lidarDev.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
			m := measurments[count]
			count++
			return lidar.Measurements{m}, nil
		}

		<-swap
		test.That(t, th.bot.rootArea.PointCloud().Size(), test.ShouldEqual, 0)
		expected := map[string]struct{}{
			"20,34,1": {},
			"32,38,1": {},
			"45,38,1": {},
			"60,35,2": {},
			"75,27,2": {},
			"88,15,2": {},
		}
		actual := map[string]struct{}{}
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			area.Iterate(func(x, y, v int) bool {
				actual[fmt.Sprintf("%d,%d,%d", x, y, v)] = struct{}{}
				return true
			})
		})
		test.That(t, actual, test.ShouldHaveLength, (cullTTL-1)*th.scansPerCull)
		test.That(t, actual, test.ShouldResemble, expected)

		count = 0
		// Next set
		measurments = []*lidar.Measurement{
			lidar.NewMeasurement(0, .2),
			lidar.NewMeasurement(10, .3),
			lidar.NewMeasurement(20, .4),
			lidar.NewMeasurement(30, .5),
			lidar.NewMeasurement(40, .6),
			lidar.NewMeasurement(50, .7),
			lidar.NewMeasurement(60, .8),
			lidar.NewMeasurement(70, .9),
			lidar.NewMeasurement(80, 1),
		}

		moveAmount := 20
		moveDir := DirectionLeft
		test.That(t, th.bot.Move(context.Background(), &moveAmount, &moveDir), test.ShouldBeNil)
		test.That(t, th.bot.rootArea.PointCloud().Size(), test.ShouldEqual, (cullTTL-1)*th.scansPerCull)
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 0)

		waitCh := waitForCulls(th, waitFor, func() {
			th.bot.SignalStop()
		})
		swap <- struct{}{}

		<-waitCh
		test.That(t, th.bot.Stop(), test.ShouldBeNil)

		test.That(t, th.bot.rootArea.PointCloud().Size(), test.ShouldEqual, 2*(cullTTL-1)*th.scansPerCull)
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 0)
		expected = map[string]struct{}{
			"20,34,1": {},
			"32,38,1": {},
			"45,38,1": {},
			"60,35,2": {},
			"75,27,2": {},
			"88,15,2": {},

			"-19,98,2": {},
			"-45,24,1": {},
			"-47,38,1": {},
			"-46,53,1": {},
			"-42,69,2": {},
			"-32,84,2": {},
		}
		actual = map[string]struct{}{}
		th.bot.rootArea.Mutate(func(area MutableArea) {
			area.Iterate(func(x, y, v int) bool {
				actual[fmt.Sprintf("%d,%d,%d", x, y, v)] = struct{}{}
				return true
			})
		})
		test.That(t, actual, test.ShouldHaveLength, 2*(cullTTL-1)*th.scansPerCull)
		test.That(t, actual, test.ShouldResemble, expected)
	})
}
