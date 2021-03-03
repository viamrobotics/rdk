package slam

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"testing"

	"go.viam.com/robotcore/base"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/gostream"
	"github.com/edaniels/test"
)

func TestCommands(t *testing.T) {
	t.Run(commandSave, func(t *testing.T) {
		th := newTestHarness(t)
		th.bot.rootArea.Mutate(func(area MutableArea) {
			area.Set(30, 20, 3)
		})
		_, err := th.cmdReg.Process(&gostream.Command{
			Name: commandSave,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandSave,
			Args: []string{"/"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "/")

		temp, err := ioutil.TempFile("", "*.las")
		test.That(t, err, test.ShouldBeNil)
		defer os.Remove(temp.Name())
		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandSave,
			Args: []string{temp.Name()},
		})
		test.That(t, err, test.ShouldBeNil)

		sizeMeters, scaleTo := th.bot.rootArea.Size()
		sq, err := NewSquareAreaFromFile(temp.Name(), sizeMeters, scaleTo)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sq, test.ShouldResemble, th.bot.rootArea)
	})

	t.Run(commandCalibrate, func(t *testing.T) {
		th := newTestHarness(t)
		_, err := th.cmdReg.Process(&gostream.Command{
			Name: commandCalibrate,
		})
		test.That(t, err, test.ShouldBeNil)

		compass := &inject.Compass{}
		th.bot.compassSensor = compass

		startCount := 0
		compass.StartCalibrationFunc = func(ctx context.Context) error {
			startCount++
			return nil
		}
		stopCount := 0
		compass.StopCalibrationFunc = func(ctx context.Context) error {
			stopCount++
			return nil
		}

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandCalibrate,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, startCount, test.ShouldEqual, 1)
		test.That(t, stopCount, test.ShouldEqual, 1)

		// 3rd spin fails
		injectBase := &inject.Base{}
		th.bot.baseDevice = injectBase
		spinCount := 0
		spinErr := errors.New("nospin")
		injectBase.SpinFunc = func(angleDeg float64, speed int, block bool) error {
			if spinCount == 2 {
				return spinErr
			}
			spinCount++
			return nil
		}

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandCalibrate,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "nospin")
		test.That(t, startCount, test.ShouldEqual, 2)
		test.That(t, stopCount, test.ShouldEqual, 2)

		// augment
		headingCount := 0
		compass.HeadingFunc = func(ctx context.Context) (float64, error) {
			headingCount++
			return math.NaN(), nil
		}
		baseWithCompass := base.Augment(injectBase, compass)
		th.bot.baseDevice = baseWithCompass
		injectBase.SpinFunc = func(angleDeg float64, speed int, block bool) error {
			return nil
		}
		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandCalibrate,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, startCount, test.ShouldEqual, 3)
		test.That(t, stopCount, test.ShouldEqual, 3)
		test.That(t, headingCount, test.ShouldEqual, 0)
	})

	t.Run(commandLidarViewMode, func(t *testing.T) {
		th := newTestHarness(t)
		_, err := th.cmdReg.Process(&gostream.Command{
			Name: commandLidarViewMode,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandLidarViewMode,
			Args: []string{"/"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandLidarViewMode,
			Args: []string{clientLidarViewModeStored},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.clientLidarViewMode, test.ShouldEqual, clientLidarViewModeStored)

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandLidarViewMode,
			Args: []string{clientLidarViewModeLive},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.clientLidarViewMode, test.ShouldEqual, clientLidarViewModeLive)
	})

	t.Run(commandClientClickMode, func(t *testing.T) {
		th := newTestHarness(t)
		_, err := th.cmdReg.Process(&gostream.Command{
			Name: commandClientClickMode,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandClientClickMode,
			Args: []string{"/"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandClientClickMode,
			Args: []string{clientClickModeMove},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.clientClickMode, test.ShouldEqual, clientClickModeMove)

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandClientClickMode,
			Args: []string{clientClickModeInfo},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.clientClickMode, test.ShouldEqual, clientClickModeInfo)
	})

	t.Run(commandRobotMove, func(t *testing.T) {
		th := newTestHarness(t)
		_, err := th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMove,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMove,
			Args: []string{"westward"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")

		origPosX, origPosY := th.bot.basePosX, th.bot.basePosY
		resp, err := th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMove,
			Args: []string{DirectionRight},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "right")
		test.That(t, th.bot.orientation(), test.ShouldEqual, 90)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX+defaultClientMoveAmount)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY)

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMove,
			Args: []string{DirectionRight},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "right")
		test.That(t, th.bot.orientation(), test.ShouldEqual, 90)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX+defaultClientMoveAmount*2)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY)

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMove,
			Args: []string{DirectionRight},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "stuck")
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX+defaultClientMoveAmount*2)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY)

		th.ResetPos()
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			area.Set(th.bot.basePosX+5, th.bot.basePosY, 3)
		})
		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMove,
			Args: []string{DirectionRight},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "collide")
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY)
	})

	t.Run(commandRobotMoveForward, func(t *testing.T) {
		th := newTestHarness(t)
		orgOrientation := th.bot.orientation()
		origPosX, origPosY := th.bot.basePosX, th.bot.basePosY
		resp, err := th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMoveForward,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "forward")
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY-defaultClientMoveAmount)

		resp, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMoveForward,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "forward")
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY-defaultClientMoveAmount*2)

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMoveForward,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "stuck")
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY-defaultClientMoveAmount*2)

		th.ResetPos()
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			area.Set(th.bot.basePosX, th.bot.basePosY-5, 3)
		})
		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMoveForward,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "collide")
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY)
	})

	t.Run(commandRobotMoveBackward, func(t *testing.T) {
		th := newTestHarness(t)
		orgOrientation := th.bot.orientation()
		origPosX, origPosY := th.bot.basePosX, th.bot.basePosY
		resp, err := th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMoveBackward,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "backward")
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY+defaultClientMoveAmount)

		resp, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMoveBackward,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "backward")
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY+defaultClientMoveAmount*2)

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMoveBackward,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "stuck")
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY+defaultClientMoveAmount*2)

		th.ResetPos()
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			area.Set(th.bot.basePosX, th.bot.basePosY+5, 3)
		})
		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotMoveBackward,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "collide")
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY)
	})

	t.Run(commandRobotTurnTo, func(t *testing.T) {
		th := newTestHarness(t)
		_, err := th.cmdReg.Process(&gostream.Command{
			Name: commandRobotTurnTo,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotTurnTo,
			Args: []string{"upwards"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")

		x, y := th.bot.basePosX, th.bot.basePosY
		resp, err := th.cmdReg.Process(&gostream.Command{
			Name: commandRobotTurnTo,
			Args: []string{DirectionUp},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, DirectionUp)
		test.That(t, th.bot.orientation(), test.ShouldEqual, 0)
		test.That(t, th.bot.basePosX, test.ShouldEqual, x)
		test.That(t, th.bot.basePosY, test.ShouldEqual, y)

		resp, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotTurnTo,
			Args: []string{DirectionDown},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, DirectionDown)
		test.That(t, th.bot.orientation(), test.ShouldEqual, 180)
		test.That(t, th.bot.basePosX, test.ShouldEqual, x)
		test.That(t, th.bot.basePosY, test.ShouldEqual, y)

		resp, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotTurnTo,
			Args: []string{DirectionLeft},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, DirectionLeft)
		test.That(t, th.bot.orientation(), test.ShouldEqual, 270)
		test.That(t, th.bot.basePosX, test.ShouldEqual, x)
		test.That(t, th.bot.basePosY, test.ShouldEqual, y)

		resp, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotTurnTo,
			Args: []string{DirectionRight},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, DirectionRight)
		test.That(t, th.bot.orientation(), test.ShouldEqual, 90)
		test.That(t, th.bot.basePosX, test.ShouldEqual, x)
		test.That(t, th.bot.basePosY, test.ShouldEqual, y)
	})

	t.Run(commandRobotStats, func(t *testing.T) {
		th := newTestHarness(t)
		resp, err := th.cmdReg.Process(&gostream.Command{
			Name: commandRobotStats,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "pos")

		th.bot.basePosX = 1
		th.bot.basePosY = 2
		resp, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotStats,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "pos")
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "(1, 2)")
	})

	t.Run(commandRobotDeviceOffset, func(t *testing.T) {
		th := newTestHarness(t)
		_, err := th.cmdReg.Process(&gostream.Command{
			Name: commandRobotDeviceOffset,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "parameters required")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotDeviceOffset,
			Args: []string{"0"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "parameters required")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotDeviceOffset,
			Args: []string{"0", "0,0,0"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "bad offset")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotDeviceOffset,
			Args: []string{"1", "0,0,0"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "bad offset")

		th.bot.deviceOffsets = append(th.bot.deviceOffsets, DeviceOffset{})
		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotDeviceOffset,
			Args: []string{"0", "000"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "format")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotDeviceOffset,
			Args: []string{"0", "a,0,0"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotDeviceOffset,
			Args: []string{"0", "0,a,0"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotDeviceOffset,
			Args: []string{"0", "0,0,a"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotDeviceOffset,
			Args: []string{"0", "2,3,4"},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.deviceOffsets[0], test.ShouldResemble, DeviceOffset{2, 3, 4})

		th.bot.deviceOffsets = append(th.bot.deviceOffsets, DeviceOffset{})
		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotDeviceOffset,
			Args: []string{"1", "4,5,6"},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.deviceOffsets[0], test.ShouldResemble, DeviceOffset{2, 3, 4})
		test.That(t, th.bot.deviceOffsets[1], test.ShouldResemble, DeviceOffset{4, 5, 6})
	})

	t.Run(commandRobotLidarStart, func(t *testing.T) {
		th := newTestHarness(t)
		_, err := th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarStart,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "device number required")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarStart,
			Args: []string{"one"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

		th.lidarDev.StartFunc = func(ctx context.Context) error {
			return errors.New("whoops")
		}
		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarStart,
			Args: []string{"0"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarStart,
			Args: []string{"1"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid")

		var started bool
		th.lidarDev.StartFunc = func(ctx context.Context) error {
			test.That(t, started, test.ShouldBeFalse)
			started = true
			return nil
		}
		resp, err := th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarStart,
			Args: []string{"0"},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "0")
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "started")

		th.bot.devices = append(th.bot.devices, &inject.LidarDevice{})
		var started2 bool
		th.bot.devices[1].(*inject.LidarDevice).StartFunc = func(ctx context.Context) error {
			test.That(t, started2, test.ShouldBeFalse)
			started2 = true
			return nil
		}
		resp, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarStart,
			Args: []string{"1"},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "1")
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "started")
	})

	t.Run(commandRobotLidarStop, func(t *testing.T) {
		th := newTestHarness(t)
		_, err := th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarStop,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "device number required")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarStop,
			Args: []string{"one"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

		th.lidarDev.StopFunc = func(ctx context.Context) error {
			return errors.New("whoops")
		}
		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarStop,
			Args: []string{"0"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarStop,
			Args: []string{"1"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid")

		var stopped bool
		th.lidarDev.StopFunc = func(ctx context.Context) error {
			test.That(t, stopped, test.ShouldBeFalse)
			stopped = true
			return nil
		}
		resp, err := th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarStop,
			Args: []string{"0"},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "0")
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "stopped")

		th.bot.devices = append(th.bot.devices, &inject.LidarDevice{})
		var stopped2 bool
		th.bot.devices[1].(*inject.LidarDevice).StopFunc = func(ctx context.Context) error {
			test.That(t, stopped2, test.ShouldBeFalse)
			stopped2 = true
			return nil
		}
		resp, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarStop,
			Args: []string{"1"},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "1")
		test.That(t, string(resp.Data()), test.ShouldContainSubstring, "stopped")
	})

	t.Run(commandRobotLidarSeed, func(t *testing.T) {
		th := newTestHarness(t)
		resp, err := th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarSeed,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldEqual, "real-device")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarSeed,
			Args: []string{"0"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "device number and seed required")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarSeed,
			Args: []string{"0", "1"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "cannot set seed")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarSeed,
			Args: []string{"5", "1"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarSeed,
			Args: []string{"0", "foo"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

		th.bot.devices[0] = &fake.Lidar{}
		th.bot.devices = append(th.bot.devices, &fake.Lidar{})
		th.bot.devices = append(th.bot.devices, &inject.LidarDevice{})
		resp, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarSeed,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldEqual, "0,0,real-device")

		resp, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarSeed,
			Args: []string{"0", "1"},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldEqual, "1")
		test.That(t, th.bot.devices[0].(*fake.Lidar).Seed(), test.ShouldEqual, 1)

		resp, err = th.cmdReg.Process(&gostream.Command{
			Name: commandRobotLidarSeed,
			Args: []string{"1", "2"},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldEqual, "2")
		test.That(t, th.bot.devices[1].(*fake.Lidar).Seed(), test.ShouldEqual, 2)
	})

	t.Run(commandClientZoom, func(t *testing.T) {
		th := newTestHarness(t)
		_, err := th.cmdReg.Process(&gostream.Command{
			Name: commandClientZoom,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "zoom level required")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandClientZoom,
			Args: []string{"pointnine"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

		_, err = th.cmdReg.Process(&gostream.Command{
			Name: commandClientZoom,
			Args: []string{".9"},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, ">= 1")

		resp, err := th.cmdReg.Process(&gostream.Command{
			Name: commandClientZoom,
			Args: []string{"2"},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(resp.Data()), test.ShouldEqual, "2")
		test.That(t, th.bot.clientZoom, test.ShouldEqual, 2.0)
	})
}

func TestHandleClick(t *testing.T) {
	t.Run("unknown click mode", func(t *testing.T) {
		th := newTestHarness(t)
		larBot := th.bot
		larBot.clientClickMode = "who"
		_, err := larBot.HandleClick(0, 0, 10, 10)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
	})

	t.Run("move mode", func(t *testing.T) {
		th := newTestHarness(t)
		larBot := th.bot
		larBot.clientClickMode = clientClickModeMove
		injectBase := &inject.Base{Device: larBot.baseDevice}
		larBot.baseDevice = injectBase
		err1 := errors.New("whoops")
		injectBase.MoveStraightFunc = func(distanceMM int, speed float64, block bool) error {
			return err1
		}
		_, err := larBot.HandleClick(1, 2, 3, 4)
		test.That(t, err, test.ShouldWrap, err1)

		for i, tc := range []struct {
			x, y                  int
			viewWidth, viewHeight int
			expectedDir           Direction
			expectedX             int
			expectedY             int
		}{
			{0, 0, 0, 0, DirectionRight, 70, 50}, // bogus for views with area < 0
			{0, 0, 2, 2, DirectionUp, 50, 30},
			{1, 0, 2, 2, DirectionDown, 50, 70},
			{0, 1, 2, 2, DirectionLeft, 30, 50},
		} {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				th := newTestHarness(t)
				larBot := th.bot
				larBot.clientClickMode = clientClickModeMove
				ret, err := larBot.HandleClick(tc.x, tc.y, tc.viewWidth, tc.viewHeight)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, ret, test.ShouldContainSubstring, fmt.Sprintf("(%d, %d)", tc.expectedX, tc.expectedY))
				test.That(t, ret, test.ShouldContainSubstring, string(tc.expectedDir))
			})
		}
	})

	t.Run("info mode", func(t *testing.T) {
		th := newTestHarness(t)
		larBot := th.bot
		larBot.clientClickMode = clientClickModeInfo
		larBot.rootArea.Mutate(func(area MutableArea) {
			area.Set(30, 20, 3)
		})

		for i, tc := range []struct {
			x, y                  int
			viewWidth, viewHeight int
			object                bool
			angleCenter           float64
			distanceCenter        int
			distanceFront         int
		}{
			{0, 0, 10, 10, false, 315, 70, 68},
			{5, 0, 10, 10, false, 0, 50, 47},
			{3, 2, 10, 10, true, 326.309932, 36, 33},
		} {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				ret, err := larBot.HandleClick(tc.x, tc.y, tc.viewWidth, tc.viewHeight)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, ret, test.ShouldContainSubstring, fmt.Sprintf("object=%t", tc.object))
				test.That(t, ret, test.ShouldContainSubstring, fmt.Sprintf("angleCenter=%f", tc.angleCenter))
				test.That(t, ret, test.ShouldContainSubstring, fmt.Sprintf("distanceCenter=%dcm", tc.distanceCenter))
				test.That(t, ret, test.ShouldContainSubstring, fmt.Sprintf("distanceFront=%dcm", tc.distanceFront))
			})
		}
	})
}
