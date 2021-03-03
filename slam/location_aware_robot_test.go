package slam

import (
	"context"
	"errors"
	"fmt"
	"image"
	"testing"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/gostream"
	"github.com/edaniels/test"
)

type testHarness struct {
	bot        *LocationAwareRobot
	baseDevice *fake.Base
	area       *SquareArea
	lidarDev   *inject.LidarDevice
	cmdReg     gostream.CommandRegistry
}

func (th *testHarness) ResetPos() {
	center := th.bot.rootArea.Center()
	th.bot.basePosX = center.X
	th.bot.basePosY = center.Y
}

func newTestHarness(t *testing.T) *testHarness {
	return newTestHarnessWithLidar(t, nil)
}

func newTestHarnessWithLidar(t *testing.T, lidarDev lidar.Device) *testHarness {
	baseDevice := &fake.Base{}
	area := NewSquareArea(10, 10)
	baseStart := area.Center()
	injectLidarDev := &inject.LidarDevice{Device: lidarDev}
	if lidarDev == nil {
		injectLidarDev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
			return image.Point{10, 10}, nil
		}
	}

	larBot, err := NewLocationAwareRobot(
		baseDevice,
		baseStart,
		area,
		[]lidar.Device{injectLidarDev},
		nil,
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	cmdReg := gostream.NewCommandRegistry()
	larBot.RegisterCommands(cmdReg)

	return &testHarness{
		larBot,
		baseDevice,
		area,
		injectLidarDev,
		cmdReg,
	}
}

func TestNewLocationAwareRobot(t *testing.T) {
	baseDevice := &fake.Base{}
	area := NewSquareArea(10, 10)
	baseStart := area.Center()
	injectLidarDev := &inject.LidarDevice{}
	injectLidarDev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
		return image.Point{10, 10}, nil
	}

	_, err := NewLocationAwareRobot(
		baseDevice,
		baseStart,
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
		baseStart,
		area,
		[]lidar.Device{injectLidarDev},
		nil,
		nil,
	)
	test.That(t, err, test.ShouldWrap, err1)
}

func TestRobotString(t *testing.T) {
	th := newTestHarness(t)
	center := th.area.Center()
	test.That(t, th.bot.String(), test.ShouldContainSubstring, fmt.Sprintf("(%d, %d)", center.X, center.Y))
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
		{"move forward", intPtr(10), nil, "", 0, -10, 0, nil},
		{"move backward", intPtr(-10), nil, "", 0, 10, 0, nil},
		{"move forward too far", intPtr(100), nil, "stuck", 0, 0, 0, nil},
		{"move backward too far", intPtr(-100), nil, "stuck", 0, 0, 0, nil},
		{"rotate down and move forward", intPtr(20), dirPtr(DirectionDown), "", 0, 20, 180, nil},
		{"rotate right and move forward", intPtr(20), dirPtr(DirectionRight), "", 20, 0, 90, nil},
		{"rotate left and move forward", intPtr(20), dirPtr(DirectionLeft), "", -20, 0, 270, nil},
		{"rotate down and move backward", intPtr(-20), dirPtr(DirectionDown), "", 0, -20, 180, nil},
		{"rotate right and move backward", intPtr(-20), dirPtr(DirectionRight), "", -20, 0, 90, nil},
		{"rotate left and move backward", intPtr(-20), dirPtr(DirectionLeft), "", 20, 0, 270, nil},
		{"rotate down and move forward too far", intPtr(200), dirPtr(DirectionDown), "stuck", 0, 0, 0, nil},
		{"rotate right and move forward too far", intPtr(200), dirPtr(DirectionRight), "stuck", 0, 0, 0, nil},
		{"rotate left and move forward too far", intPtr(200), dirPtr(DirectionLeft), "stuck", 0, 0, 0, nil},
		{"cannot collide up", intPtr(20), dirPtr(DirectionUp), "collide", 0, 0, 0, func(th *testHarness) {
			th.bot.presentViewArea.Mutate(func(area MutableArea) {
				area.Set(th.bot.basePosX, th.bot.basePosY-15, 3)
			})
		}},
		{"cannot collide down", intPtr(20), dirPtr(DirectionDown), "collide", 0, 0, 0, func(th *testHarness) {
			th.bot.presentViewArea.Mutate(func(area MutableArea) {
				area.Set(th.bot.basePosX, th.bot.basePosY+15, 3)
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
	center := th.area.Center()
	x, y := th.bot.basePos()
	test.That(t, x, test.ShouldEqual, center.X)
	test.That(t, y, test.ShouldEqual, center.Y)

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
		{0, 0, 0, 0, "", image.Point{}},
		{0, 0, 0, 1, "stuck", image.Point{}},
		{0, 0, 45, 1, "orientation", image.Point{}},
		{0, 0, 90, 1, "", image.Point{1, 0}},
		{0, 0, 180, 1, "", image.Point{0, 1}},
		{0, 0, 270, 1, "stuck", image.Point{}},
		{50, 50, 0, 1, "", image.Point{50, 49}},
		{50, 50, 90, 1, "", image.Point{51, 50}},
		{50, 50, 180, 1, "", image.Point{50, 51}},
		{50, 50, 270, 1, "", image.Point{49, 50}},
		{100, 100, 0, 1, "", image.Point{100, 99}},
		{100, 100, 45, 1, "orientation", image.Point{}},
		{100, 100, 90, 1, "stuck", image.Point{}},
		{100, 100, 180, 1, "stuck", image.Point{}},
		{100, 100, 270, 1, "", image.Point{99, 100}},
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
		{50, 50, image.Rect(47, 47, 53, 53)},
		{40, 13, image.Rect(37, 10, 43, 16)},
		{100, 100, image.Rect(97, 97, 103, 103)},
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
		{0, 0, 0, 0, 0, image.Rect(-3, -3, 3, 0)},
		{0, 0, 0, 0, 90, image.Rect(3, -3, 18, 3)},
		{0, 0, 0, 0, 180, image.Rect(-3, 3, 3, 18)},
		{0, 0, 0, 0, 270, image.Rect(-3, -3, 0, 3)},
		{23, 54, 23, 54, 0, image.Rect(20, 36, 26, 51)},
		{23, 54, 23, 54, 90, image.Rect(26, 51, 41, 57)},
		{23, 54, 23, 54, 180, image.Rect(20, 57, 26, 72)},
		{23, 54, 23, 54, 270, image.Rect(5, 51, 20, 57)},
		{49, 48, 50, 32, 0, image.Rect(46, 14, 52, 45)},
		{49, 48, 64, 50, 90, image.Rect(52, 45, 82, 51)},
		{49, 48, 50, 64, 180, image.Rect(46, 51, 52, 82)},
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
		area.Set(32, 50, 2)
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
		"32,50,2": {},
	}
	th.bot.rootArea.Mutate(func(area MutableArea) {
		area.Iterate(func(x, y, v int) bool {
			delete(expected, fmt.Sprintf("%d,%d,%d", x, y, v))
			return true
		})
	})
	test.That(t, expected, test.ShouldBeEmpty)
}
