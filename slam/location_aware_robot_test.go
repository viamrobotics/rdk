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
	return newTestHarnessWithLidar(t, nil)
}

func newTestHarnessWithLidar(t *testing.T, lidarDev lidar.Device) *testHarness {
	baseDevice := &fake.Base{}
	area, err := NewSquareArea(10, 10)
	test.That(t, err, test.ShouldBeNil)
	injectLidarDev := &inject.LidarDevice{Device: lidarDev}
	if lidarDev == nil {
		injectLidarDev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
			return image.Point{10, 10}, nil
		}
	}

	larBot, err := NewLocationAwareRobot(
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
	area, err := NewSquareArea(10, 10)
	test.That(t, err, test.ShouldBeNil)
	injectLidarDev := &inject.LidarDevice{}
	injectLidarDev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
		return image.Point{10, 10}, nil
	}

	_, err = NewLocationAwareRobot(
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
		baseDevice,
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
	th.bot.Stop()
	test.That(t, th.bot.Start(), test.ShouldEqual, ErrStopped)

	th = newTestHarness(t)
	test.That(t, th.bot.Start(), test.ShouldBeNil)
	th.bot.Close()
	test.That(t, th.bot.Start(), test.ShouldEqual, ErrStopped)
	th.bot.Stop()
	test.That(t, th.bot.Start(), test.ShouldEqual, ErrStopped)

	th = newTestHarness(t)
	test.That(t, th.bot.Start(), test.ShouldBeNil)
	th.bot.SignalStop()
	test.That(t, th.bot.Start(), test.ShouldEqual, ErrStopped)
	th.bot.Stop()
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
		amount           *int
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
		{"move forward", intPtr(10), nil, "", 0, 10, 0, nil},
		{"move backward", intPtr(-10), nil, "", 0, -10, 0, nil},
		{"move forward too far", intPtr(100), nil, "stuck", 0, 0, 0, nil},
		{"move backward too far", intPtr(-100), nil, "stuck", 0, 0, 0, nil},
		{"rotate down and move forward", intPtr(20), dirPtr(DirectionDown), "", 0, -20, 180, nil},
		{"rotate right and move forward", intPtr(20), dirPtr(DirectionRight), "", 20, 0, 90, nil},
		{"rotate left and move forward", intPtr(20), dirPtr(DirectionLeft), "", -20, 0, 270, nil},
		{"rotate down and move backward", intPtr(-20), dirPtr(DirectionDown), "", 0, 20, 180, nil},
		{"rotate right and move backward", intPtr(-20), dirPtr(DirectionRight), "", -20, 0, 90, nil},
		{"rotate left and move backward", intPtr(-20), dirPtr(DirectionLeft), "", 20, 0, 270, nil},
		{"rotate down and move forward too far", intPtr(200), dirPtr(DirectionDown), "stuck", 0, 0, 0, nil},
		{"rotate right and move forward too far", intPtr(200), dirPtr(DirectionRight), "stuck", 0, 0, 0, nil},
		{"rotate left and move forward too far", intPtr(200), dirPtr(DirectionLeft), "stuck", 0, 0, 0, nil},
		{"cannot collide up", intPtr(20), dirPtr(DirectionUp), "collide", 0, 0, 0, func(th *testHarness) {
			th.bot.presentViewArea.Mutate(func(area MutableArea) {
				area.Set(th.bot.basePosX, th.bot.basePosY+15, 3)
			})
		}},
		{"cannot collide down", intPtr(20), dirPtr(DirectionDown), "collide", 0, 0, 0, func(th *testHarness) {
			th.bot.presentViewArea.Mutate(func(area MutableArea) {
				area.Set(th.bot.basePosX, th.bot.basePosY-15, 3)
			})
		}},
		{"cannot collide left", intPtr(20), dirPtr(DirectionLeft), "collide", 0, 0, 0, func(th *testHarness) {
			th.bot.presentViewArea.Mutate(func(area MutableArea) {
				area.Set(th.bot.basePosX-15, th.bot.basePosY, 3)
			})
		}},
		{"cannot collide right", intPtr(20), dirPtr(DirectionRight), "collide", 0, 0, 0, func(th *testHarness) {
			th.bot.presentViewArea.Mutate(func(area MutableArea) {
				area.Set(th.bot.basePosX+15, th.bot.basePosY, 3)
			})
		}},
		{"unknown direction", intPtr(20), dirPtr("ouch"), "do not know how", 0, 0, 0, nil},
		{"moving fails", intPtr(20), dirPtr(DirectionRight), "whoops", 0, 0, 0, func(th *testHarness) {
			injectBase := &inject.Base{}
			th.bot.baseDevice = injectBase
			injectBase.SpinFunc = func(angleDeg float64, speed int, block bool) error {
				return errors.New("whoops")
			}
			injectBase.MoveStraightFunc = func(distanceMM int, speed float64, block bool) error {
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
			err := th.bot.Move(tc.amount, tc.rotateTo)
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

func TestRobotCalculateMove(t *testing.T) {
	for i, tc := range []struct {
		basePosX    int
		basePosY    int
		orientation float64
		amount      int
		err         string
		newCoords   image.Point
	}{
		{-50, -50, 0, 0, "", image.Point{-50, -50}},
		{-50, -50, 0, 1, "", image.Point{-50, -49}},
		{-50, -50, 45, 1, "orientation", image.Point{}},
		{-50, -50, 90, 1, "", image.Point{-49, -50}},
		{-50, -50, 180, 1, "stuck", image.Point{}},
		{-50, -50, 270, 1, "stuck", image.Point{}},
		{0, 0, 0, 1, "", image.Point{0, 1}},
		{0, 0, 90, 1, "", image.Point{1, 0}},
		{0, 0, 180, 1, "", image.Point{0, -1}},
		{0, 0, 270, 1, "", image.Point{-1, 0}},
		{49, 49, 0, 1, "stuck", image.Point{}},
		{49, 49, 45, 1, "orientation", image.Point{}},
		{49, 49, 90, 1, "stuck", image.Point{}},
		{49, 49, 180, 1, "", image.Point{49, 48}},
		{49, 49, 270, 1, "", image.Point{48, 49}},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			th := newTestHarness(t)
			th.bot.basePosX = tc.basePosX
			th.bot.basePosY = tc.basePosY
			newCoords, err := th.bot.calculateMove(tc.orientation, tc.amount)
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
		{0, 0, image.Rect(-3, -3, 3, 3)},
		{-50, -50, image.Rect(-53, -53, -47, -47)},
		{40, 13, image.Rect(37, 10, 43, 16)},
		{49, 49, image.Rect(46, 46, 52, 52)},
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
	th := newTestHarness(t)
	test.That(t, func() {
		th.bot.moveRect(0, 0, -23)
	}, test.ShouldPanic)

	for i, tc := range []struct {
		basePosX    int
		basePosY    int
		toX         int
		toY         int
		orientation float64
		rect        image.Rectangle
	}{
		{0, 0, 0, 0, 0, image.Rect(-3, 3, 3, 18)},
		{0, 0, 0, 0, 90, image.Rect(3, -3, 18, 3)},
		{0, 0, 0, 0, 180, image.Rect(-3, -18, 3, -3)},
		{0, 0, 0, 0, 270, image.Rect(-18, -3, -3, 3)},
		{23, 54, 23, 54, 0, image.Rect(20, 57, 26, 72)},
		{23, 54, 23, 54, 90, image.Rect(26, 51, 41, 57)},
		{23, 54, 23, 54, 180, image.Rect(20, 36, 26, 51)},
		{23, 54, 23, 54, 270, image.Rect(5, 51, 20, 57)},
		{49, 48, 50, 32, 0, image.Rect(46, 51, 52, 82)},
		{49, 48, 64, 50, 90, image.Rect(52, 45, 82, 51)},
		{49, 48, 50, 64, 180, image.Rect(46, 14, 52, 45)},
		{49, 48, 32, 50, 270, image.Rect(14, 45, 46, 51)},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			th := newTestHarness(t)
			th.bot.basePosX = tc.basePosX
			th.bot.basePosY = tc.basePosY
			test.That(t, th.bot.moveRect(tc.toX, tc.toY, tc.orientation), test.ShouldResemble, tc.rect)
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
	th.bot.newPresentView()

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
		area.Set(1, 2, 5)
		area.Set(0, 4, 1)
		area.Set(7, 6, 1)
		area.Set(1, 1, 0)
		area.Set(0, 0, 1)
		area.Set(32, 49, 2)
	})
	th.bot.newPresentView()

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
	area := th.area.BlankCopy()
	device := &inject.LidarDevice{}
	err1 := errors.New("whoops")
	device.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
		return nil, err1
	}
	err := th.bot.scanAndStore([]lidar.Device{device}, area)
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
				"0,10,3": {},
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
				"0,10,3": {},
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
				"0,10,3": {},
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
				"0,10,3": {},
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
				"0,30,3":   {},
				"14,37,3":  {},
				"8,4,3":    {},
				"14,-13,3": {},
				"-37,13,3": {},
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
		{"one device with some measurements near bounds", -50, -50, 0, nil, []lidar.Measurements{
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
				"-50,-20,3": {},
				"-35,-12,3": {},
				"-41,-45,3": {},
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
				"0,30,3":   {},
				"14,37,3":  {},
				"8,4,3":    {},
				"14,-13,3": {},
				"-37,13,3": {},
				"1,29,3":   {},
				"15,36,3":  {},
				"9,4,3":    {},
				"14,-14,3": {},
				"-37,14,3": {},
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
				"0,30,3":   {},
				"14,37,3":  {},
				"8,4,3":    {},
				"14,-13,3": {},
				"-37,13,3": {},
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
					"0,30,3":   {},
					"14,37,3":  {},
					"8,4,3":    {},
					"14,-13,3": {},
					"-37,13,3": {},
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
				{45, 1, 2},
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
					"0,30,3":   {},
					"14,37,3":  {},
					"8,4,3":    {},
					"14,-13,3": {},
					"-37,13,3": {},
					"0,32,3":   {},
					"15,39,3":  {},
					"9,6,3":    {},
					"3,2,3":    {},
					"15,-11,3": {},
					"-36,15,3": {},
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
			area := th.area.BlankCopy()
			var err error
			if internal {
				err = th.bot.scanAndStore(devices, area)
			} else {
				th.bot.devices = devices
				err = th.bot.update()
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
			area.Set(1, 2, 3)
			area.Set(0, 4, 3)
			area.Set(7, 6, 4)
		})
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			area.Set(1, 2, 3)
			area.Set(0, 4, 3)
			area.Set(7, 6, 4)
			area.Set(1, 1, 3)
			area.Set(0, 0, 3)
			area.Set(2, 49, 3)
			area.Set(-35, -4, 3)
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
			area.Set(1, 2, 3)
			area.Set(0, 4, 3)
			area.Set(7, 6, 4)
			area.Set(1, 1, 3)
			area.Set(0, 0, 3)
			area.Set(2, 49, 3)
			area.Set(-35, -4, 3)
		})
		th.bot.cull()
		th.bot.cull()
		th.bot.cull()
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 3)
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			test.That(t, area.At(7, 6), test.ShouldEqual, 1)
			test.That(t, area.At(2, 49), test.ShouldEqual, 3)
			test.That(t, area.At(-35, -4), test.ShouldEqual, 3)
		})
		th.bot.cull()
		th.bot.cull()
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 2)
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			test.That(t, area.At(2, 49), test.ShouldEqual, 3)
			test.That(t, area.At(-35, -4), test.ShouldEqual, 3)
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
		th.bot.Stop()

		test.That(t, th.bot.rootArea.PointCloud().Size(), test.ShouldEqual, 6)
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 0)
		expected := map[string]struct{}{
			"1,3,1": {},
			"3,3,1": {},
			"4,3,1": {},
			"6,3,2": {},
			"7,2,2": {},
			"8,1,2": {},
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
			"1,3,1": {},
			"3,3,1": {},
			"4,3,1": {},
			"6,3,2": {},
			"7,2,2": {},
			"8,1,2": {},
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
		test.That(t, th.bot.Move(&moveAmount, &moveDir), test.ShouldBeNil)
		test.That(t, th.bot.rootArea.PointCloud().Size(), test.ShouldEqual, (cullTTL-1)*th.scansPerCull)
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 0)

		waitCh := waitForCulls(th, waitFor, func() {
			th.bot.SignalStop()
		})
		swap <- struct{}{}

		<-waitCh
		th.bot.Stop()

		test.That(t, th.bot.rootArea.PointCloud().Size(), test.ShouldEqual, 2*(cullTTL-1)*th.scansPerCull)
		test.That(t, th.bot.presentViewArea.PointCloud().Size(), test.ShouldEqual, 0)
		expected = map[string]struct{}{
			"1,3,1": {},
			"3,3,1": {},
			"4,3,1": {},
			"6,3,2": {},
			"7,2,2": {},
			"8,1,2": {},

			"-21,9,2": {},
			"-23,8,2": {},
			"-24,2,1": {},
			"-24,3,1": {},
			"-24,5,1": {},
			"-24,6,2": {},
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
