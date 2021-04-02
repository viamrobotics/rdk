package slam

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"testing"

	"go.viam.com/robotcore/api"
	pb "go.viam.com/robotcore/proto/slam/v1"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/test"
)

func TestServer(t *testing.T) {
	t.Run("Save", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)
		th.bot.rootArea.Mutate(func(area MutableArea) {
			test.That(t, area.Set(30, 20, 3), test.ShouldBeNil)
		})
		_, err := server.Save(context.Background(), &pb.SaveRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		_, err = server.Save(context.Background(), &pb.SaveRequest{File: "/"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "/")

		temp, err := ioutil.TempFile("", "*.las")
		test.That(t, err, test.ShouldBeNil)
		defer os.Remove(temp.Name())
		_, err = server.Save(context.Background(), &pb.SaveRequest{File: temp.Name()})
		test.That(t, err, test.ShouldBeNil)

		sizeMeters, unitsPerMeter := th.bot.rootArea.Size()
		sq, err := NewSquareAreaFromFile(temp.Name(), sizeMeters, unitsPerMeter, th.logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sq, test.ShouldResemble, th.bot.rootArea)
	})

	t.Run("Calibrate", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)
		_, err := server.Calibrate(context.Background(), &pb.CalibrateRequest{})
		test.That(t, err, test.ShouldBeNil)

		theCompass := &inject.Compass{}
		th.bot.compassSensor = theCompass

		startCount := 0
		theCompass.StartCalibrationFunc = func(ctx context.Context) error {
			startCount++
			return nil
		}
		stopCount := 0
		theCompass.StopCalibrationFunc = func(ctx context.Context) error {
			stopCount++
			return nil
		}

		_, err = server.Calibrate(context.Background(), &pb.CalibrateRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, startCount, test.ShouldEqual, 1)
		test.That(t, stopCount, test.ShouldEqual, 1)

		// 3rd spin fails
		injectBase := &inject.Base{}
		injectBase.WidthMillisFunc = func(ctx context.Context) (int, error) {
			return 600, nil
		}
		th.bot.baseDevice = injectBase
		spinCount := 0
		spinErr := errors.New("nospin")
		injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, speed int, block bool) error {
			if spinCount == 2 {
				return spinErr
			}
			spinCount++
			return nil
		}

		_, err = server.Calibrate(context.Background(), &pb.CalibrateRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "nospin")
		test.That(t, startCount, test.ShouldEqual, 2)
		test.That(t, stopCount, test.ShouldEqual, 2)

		// augment
		headingCount := 0
		theCompass.HeadingFunc = func(ctx context.Context) (float64, error) {
			headingCount++
			return math.NaN(), nil
		}
		baseWithCompass := api.BaseWithCompass(injectBase, theCompass, th.logger)
		th.bot.baseDevice = baseWithCompass
		injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, speed int, block bool) error {
			return nil
		}
		_, err = server.Calibrate(context.Background(), &pb.CalibrateRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, startCount, test.ShouldEqual, 3)
		test.That(t, stopCount, test.ShouldEqual, 3)
		test.That(t, headingCount, test.ShouldEqual, 0)
	})

	t.Run("SetClientLidarViewMode", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)
		_, err := server.SetClientLidarViewMode(context.Background(), &pb.SetClientLidarViewModeRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		_, err = server.SetClientLidarViewMode(context.Background(), &pb.SetClientLidarViewModeRequest{Mode: pb.LidarViewMode_LIDAR_VIEW_MODE_UNSPECIFIED})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		_, err = server.SetClientLidarViewMode(context.Background(), &pb.SetClientLidarViewModeRequest{Mode: pb.LidarViewMode_LIDAR_VIEW_MODE_STORED})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.clientLidarViewMode, test.ShouldEqual, pb.LidarViewMode_LIDAR_VIEW_MODE_STORED)

		_, err = server.SetClientLidarViewMode(context.Background(), &pb.SetClientLidarViewModeRequest{Mode: pb.LidarViewMode_LIDAR_VIEW_MODE_LIVE})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.clientLidarViewMode, test.ShouldEqual, pb.LidarViewMode_LIDAR_VIEW_MODE_LIVE)
	})

	t.Run("SetClientClickMode", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)
		_, err := server.SetClientClickMode(context.Background(), &pb.SetClientClickModeRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		_, err = server.SetClientClickMode(context.Background(), &pb.SetClientClickModeRequest{Mode: pb.ClickMode_CLICK_MODE_UNSPECIFIED})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		_, err = server.SetClientClickMode(context.Background(), &pb.SetClientClickModeRequest{Mode: pb.ClickMode_CLICK_MODE_MOVE})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.clientClickMode, test.ShouldEqual, pb.ClickMode_CLICK_MODE_MOVE)

		_, err = server.SetClientClickMode(context.Background(), &pb.SetClientClickModeRequest{Mode: pb.ClickMode_CLICK_MODE_INFO})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.clientClickMode, test.ShouldEqual, pb.ClickMode_CLICK_MODE_INFO)
	})

	t.Run("MoveRobot", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)
		_, err := server.MoveRobot(context.Background(), &pb.MoveRobotRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		_, err = server.MoveRobot(context.Background(), &pb.MoveRobotRequest{Direction: pb.Direction_DIRECTION_UNSPECIFIED})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		origPosX, origPosY := th.bot.basePosX, th.bot.basePosY
		resp, err := server.MoveRobot(context.Background(), &pb.MoveRobotRequest{Direction: pb.Direction_DIRECTION_RIGHT})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.MoveRobotResponse{
			NewPosition: &pb.BasePosition{X: 20, Y: 0},
		})
		test.That(t, th.bot.orientation(), test.ShouldEqual, 90)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX+th.bot.millimetersToMeasuredUnit(defaultClientMoveAmountMillis))
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY)

		for i := 0; i < 23; i++ {
			resp, err = server.MoveRobot(context.Background(), &pb.MoveRobotRequest{Direction: pb.Direction_DIRECTION_RIGHT})
			test.That(t, err, test.ShouldBeNil)
			expectedX := origPosX + th.bot.millimetersToMeasuredUnit(defaultClientMoveAmountMillis)*(2+i)
			expectedY := origPosY
			test.That(t, resp, test.ShouldResemble, &pb.MoveRobotResponse{
				NewPosition: &pb.BasePosition{
					X: int64(expectedX),
					Y: int64(expectedY),
				},
			})
			test.That(t, th.bot.orientation(), test.ShouldEqual, 90)
			test.That(t, th.bot.basePosX, test.ShouldEqual, expectedX)
			test.That(t, th.bot.basePosY, test.ShouldEqual, expectedY)
		}

		_, err = server.MoveRobot(context.Background(), &pb.MoveRobotRequest{Direction: pb.Direction_DIRECTION_RIGHT})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "stuck")
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX+th.bot.millimetersToMeasuredUnit(defaultClientMoveAmountMillis)*24)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY)

		th.ResetPos()
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			test.That(t, area.Set(th.bot.basePosX+(th.bot.baseDeviceWidthUnits/2)+1, th.bot.basePosY, 3), test.ShouldBeNil)
		})
		_, err = server.MoveRobot(context.Background(), &pb.MoveRobotRequest{Direction: pb.Direction_DIRECTION_RIGHT})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "collide")
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY)
	})

	t.Run("MoveRobotForward", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)
		orgOrientation := th.bot.orientation()
		origPosX, origPosY := th.bot.basePosX, th.bot.basePosY
		resp, err := server.MoveRobotForward(context.Background(), &pb.MoveRobotForwardRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.MoveRobotForwardResponse{
			NewPosition: &pb.BasePosition{
				X: 0,
				Y: 20,
			},
		})
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY+th.bot.millimetersToMeasuredUnit(defaultClientMoveAmountMillis))

		for i := 0; i < 23; i++ {
			resp, err = server.MoveRobotForward(context.Background(), &pb.MoveRobotForwardRequest{})
			test.That(t, err, test.ShouldBeNil)
			expectedX := origPosX
			expectedY := origPosY + th.bot.millimetersToMeasuredUnit(defaultClientMoveAmountMillis)*(2+i)
			test.That(t, resp, test.ShouldResemble, &pb.MoveRobotForwardResponse{
				NewPosition: &pb.BasePosition{
					X: int64(expectedX),
					Y: int64(expectedY),
				},
			})
			test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
			test.That(t, th.bot.basePosX, test.ShouldEqual, expectedX)
			test.That(t, th.bot.basePosY, test.ShouldEqual, expectedY)
		}

		_, err = server.MoveRobotForward(context.Background(), &pb.MoveRobotForwardRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "stuck")
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY+th.bot.millimetersToMeasuredUnit(defaultClientMoveAmountMillis)*24)

		th.ResetPos()
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			test.That(t, area.Set(th.bot.basePosX, th.bot.basePosY+(th.bot.baseDeviceWidthUnits/2)+1, 3), test.ShouldBeNil)
		})
		_, err = server.MoveRobotForward(context.Background(), &pb.MoveRobotForwardRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "collide")
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY)
	})

	t.Run("MoveRobotBackward", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)
		orgOrientation := th.bot.orientation()
		origPosX, origPosY := th.bot.basePosX, th.bot.basePosY
		resp, err := server.MoveRobotBackward(context.Background(), &pb.MoveRobotBackwardRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.MoveRobotBackwardResponse{
			NewPosition: &pb.BasePosition{
				X: 0,
				Y: -20,
			},
		})
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY-th.bot.millimetersToMeasuredUnit(defaultClientMoveAmountMillis))

		for i := 0; i < 24; i++ {
			resp, err = server.MoveRobotBackward(context.Background(), &pb.MoveRobotBackwardRequest{})
			test.That(t, err, test.ShouldBeNil)
			expectedX := origPosX
			expectedY := origPosY - th.bot.millimetersToMeasuredUnit(defaultClientMoveAmountMillis)*(2+i)
			test.That(t, resp, test.ShouldResemble, &pb.MoveRobotBackwardResponse{
				NewPosition: &pb.BasePosition{
					X: int64(expectedX),
					Y: int64(expectedY),
				},
			})
			test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
			test.That(t, th.bot.basePosX, test.ShouldEqual, expectedX)
			test.That(t, th.bot.basePosY, test.ShouldEqual, expectedY)
		}

		_, err = server.MoveRobotBackward(context.Background(), &pb.MoveRobotBackwardRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "stuck")
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY-th.bot.millimetersToMeasuredUnit(defaultClientMoveAmountMillis)*25)

		th.ResetPos()
		th.bot.presentViewArea.Mutate(func(area MutableArea) {
			test.That(t, area.Set(th.bot.basePosX, th.bot.basePosY-((th.bot.baseDeviceWidthUnits/2)+1), 3), test.ShouldBeNil)
		})
		_, err = server.MoveRobotBackward(context.Background(), &pb.MoveRobotBackwardRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "collide")
		test.That(t, th.bot.orientation(), test.ShouldEqual, orgOrientation)
		test.That(t, th.bot.basePosX, test.ShouldEqual, origPosX)
		test.That(t, th.bot.basePosY, test.ShouldEqual, origPosY)
	})

	t.Run("TurnRobotTo", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)
		_, err := server.TurnRobotTo(context.Background(), &pb.TurnRobotToRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		_, err = server.TurnRobotTo(context.Background(), &pb.TurnRobotToRequest{Direction: pb.Direction_DIRECTION_UNSPECIFIED})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "required")

		x, y := th.bot.basePosX, th.bot.basePosY
		_, err = server.TurnRobotTo(context.Background(), &pb.TurnRobotToRequest{Direction: pb.Direction_DIRECTION_UP})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.orientation(), test.ShouldEqual, 0)
		test.That(t, th.bot.basePosX, test.ShouldEqual, x)
		test.That(t, th.bot.basePosY, test.ShouldEqual, y)

		_, err = server.TurnRobotTo(context.Background(), &pb.TurnRobotToRequest{Direction: pb.Direction_DIRECTION_DOWN})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.orientation(), test.ShouldEqual, 180)
		test.That(t, th.bot.basePosX, test.ShouldEqual, x)
		test.That(t, th.bot.basePosY, test.ShouldEqual, y)

		_, err = server.TurnRobotTo(context.Background(), &pb.TurnRobotToRequest{Direction: pb.Direction_DIRECTION_LEFT})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.orientation(), test.ShouldEqual, 270)
		test.That(t, th.bot.basePosX, test.ShouldEqual, x)
		test.That(t, th.bot.basePosY, test.ShouldEqual, y)

		_, err = server.TurnRobotTo(context.Background(), &pb.TurnRobotToRequest{Direction: pb.Direction_DIRECTION_RIGHT})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.orientation(), test.ShouldEqual, 90)
		test.That(t, th.bot.basePosX, test.ShouldEqual, x)
		test.That(t, th.bot.basePosY, test.ShouldEqual, y)
	})

	t.Run("Stats", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)
		resp, err := server.Stats(context.Background(), &pb.StatsRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.StatsResponse{
			BasePosition: &pb.BasePosition{
				X: 0,
				Y: 0,
			},
		})

		th.bot.basePosX = 1
		th.bot.basePosY = 2
		resp, err = server.Stats(context.Background(), &pb.StatsRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.StatsResponse{
			BasePosition: &pb.BasePosition{
				X: 1,
				Y: 2,
			},
		})
	})

	t.Run("UpdateRobotDeviceOffset", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)
		_, err := server.UpdateRobotDeviceOffset(context.Background(), &pb.UpdateRobotDeviceOffsetRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "bad offset")

		_, err = server.UpdateRobotDeviceOffset(context.Background(), &pb.UpdateRobotDeviceOffsetRequest{
			OffsetIndex: 0,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "bad offset")

		_, err = server.UpdateRobotDeviceOffset(context.Background(), &pb.UpdateRobotDeviceOffsetRequest{
			OffsetIndex: 0,
			Offset:      &pb.DeviceOffset{},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "bad offset")

		_, err = server.UpdateRobotDeviceOffset(context.Background(), &pb.UpdateRobotDeviceOffsetRequest{
			OffsetIndex: 1,
			Offset:      &pb.DeviceOffset{},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "bad offset")

		th.bot.deviceOffsets = append(th.bot.deviceOffsets, DeviceOffset{})
		_, err = server.UpdateRobotDeviceOffset(context.Background(), &pb.UpdateRobotDeviceOffsetRequest{
			OffsetIndex: 0,
			Offset: &pb.DeviceOffset{
				Angle:     2,
				DistanceX: 3,
				DistanceY: 4,
			},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.deviceOffsets[0], test.ShouldResemble, DeviceOffset{2, 3, 4})

		th.bot.deviceOffsets = append(th.bot.deviceOffsets, DeviceOffset{})

		_, err = server.UpdateRobotDeviceOffset(context.Background(), &pb.UpdateRobotDeviceOffsetRequest{
			OffsetIndex: 1,
			Offset: &pb.DeviceOffset{
				Angle:     4,
				DistanceX: 5,
				DistanceY: 6,
			},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.deviceOffsets[0], test.ShouldResemble, DeviceOffset{2, 3, 4})
		test.That(t, th.bot.deviceOffsets[1], test.ShouldResemble, DeviceOffset{4, 5, 6})
	})

	t.Run("StartLidar", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)

		th.lidarDev.StartFunc = func(ctx context.Context) error {
			return errors.New("whoops")
		}
		_, err := server.StartLidar(context.Background(), &pb.StartLidarRequest{
			DeviceNumber: 0,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

		_, err = server.StartLidar(context.Background(), &pb.StartLidarRequest{
			DeviceNumber: 1,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid")

		var started bool
		th.lidarDev.StartFunc = func(ctx context.Context) error {
			test.That(t, started, test.ShouldBeFalse)
			started = true
			return nil
		}
		_, err = server.StartLidar(context.Background(), &pb.StartLidarRequest{
			DeviceNumber: 0,
		})
		test.That(t, err, test.ShouldBeNil)

		th.bot.devices = append(th.bot.devices, &inject.LidarDevice{})
		var started2 bool
		th.bot.devices[1].(*inject.LidarDevice).StartFunc = func(ctx context.Context) error {
			test.That(t, started2, test.ShouldBeFalse)
			started2 = true
			return nil
		}
		_, err = server.StartLidar(context.Background(), &pb.StartLidarRequest{
			DeviceNumber: 1,
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("StopLidar", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)

		th.lidarDev.StopFunc = func(ctx context.Context) error {
			return errors.New("whoops")
		}
		_, err := server.StopLidar(context.Background(), &pb.StopLidarRequest{
			DeviceNumber: 0,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

		_, err = server.StopLidar(context.Background(), &pb.StopLidarRequest{
			DeviceNumber: 1,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid")

		var stopped bool
		th.lidarDev.StopFunc = func(ctx context.Context) error {
			test.That(t, stopped, test.ShouldBeFalse)
			stopped = true
			return nil
		}
		_, err = server.StopLidar(context.Background(), &pb.StopLidarRequest{
			DeviceNumber: 0,
		})
		test.That(t, err, test.ShouldBeNil)

		th.bot.devices = append(th.bot.devices, &inject.LidarDevice{})
		var stopped2 bool
		th.bot.devices[1].(*inject.LidarDevice).StopFunc = func(ctx context.Context) error {
			test.That(t, stopped2, test.ShouldBeFalse)
			stopped2 = true
			return nil
		}
		_, err = server.StopLidar(context.Background(), &pb.StopLidarRequest{
			DeviceNumber: 1,
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("GetLidarSeed", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)
		resp, err := server.GetLidarSeed(context.Background(), &pb.GetLidarSeedRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.GetLidarSeedResponse{Seeds: []string{"real-device"}})
	})

	t.Run("SetLidarSeed", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)

		_, err := server.SetLidarSeed(context.Background(), &pb.SetLidarSeedRequest{
			DeviceNumber: 0,
			Seed:         1,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "cannot set seed")

		_, err = server.SetLidarSeed(context.Background(), &pb.SetLidarSeedRequest{
			DeviceNumber: 5,
			Seed:         1,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid")

		th.bot.devices[0] = &fake.Lidar{}
		th.bot.devices = append(th.bot.devices, &fake.Lidar{})
		th.bot.devices = append(th.bot.devices, &inject.LidarDevice{})
		getResp, err := server.GetLidarSeed(context.Background(), &pb.GetLidarSeedRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, getResp, test.ShouldResemble, &pb.GetLidarSeedResponse{Seeds: []string{"0", "0", "real-device"}})

		_, err = server.SetLidarSeed(context.Background(), &pb.SetLidarSeedRequest{
			DeviceNumber: 0,
			Seed:         1,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.devices[0].(*fake.Lidar).Seed(), test.ShouldEqual, 1)

		_, err = server.SetLidarSeed(context.Background(), &pb.SetLidarSeedRequest{
			DeviceNumber: 1,
			Seed:         2,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.devices[1].(*fake.Lidar).Seed(), test.ShouldEqual, 2)
	})

	t.Run("SetClientZoom", func(t *testing.T) {
		th := newTestHarness(t)
		server := NewLocationAwareRobotServer(th.bot)

		_, err := server.SetClientZoom(context.Background(), &pb.SetClientZoomRequest{
			Zoom: 0.9,
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, ">= 1")

		_, err = server.SetClientZoom(context.Background(), &pb.SetClientZoomRequest{
			Zoom: 2,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.bot.clientZoom, test.ShouldEqual, 2.0)
	})
}

func TestHandleClick(t *testing.T) {
	t.Run("unknown click mode", func(t *testing.T) {
		th := newTestHarness(t)
		larBot := th.bot
		larBot.clientClickMode = pb.ClickMode_CLICK_MODE_UNSPECIFIED
		_, err := larBot.HandleClick(context.Background(), 0, 0, 10, 10)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
	})

	t.Run("move mode", func(t *testing.T) {
		th := newTestHarness(t)
		larBot := th.bot
		larBot.clientClickMode = pb.ClickMode_CLICK_MODE_MOVE
		injectBase := &inject.Base{Base: larBot.baseDevice}
		injectBase.WidthMillisFunc = func(ctx context.Context) (int, error) {
			return 600, nil
		}
		larBot.baseDevice = injectBase
		err1 := errors.New("whoops")
		injectBase.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
			return err1
		}
		_, err := larBot.HandleClick(context.Background(), 1, 2, 3, 4)
		test.That(t, err, test.ShouldWrap, err1)

		for i, tc := range []struct {
			x, y                  int
			viewWidth, viewHeight int
			expectedDir           pb.Direction
			expectedX             int
			expectedY             int
		}{
			{0, 0, 0, 0, pb.Direction_DIRECTION_RIGHT, 20, 0}, // bogus for views with area < 0
			{0, 0, 2, 2, pb.Direction_DIRECTION_UP, 0, 20},
			{1, 0, 2, 2, pb.Direction_DIRECTION_DOWN, 0, -20},
			{0, 1, 2, 2, pb.Direction_DIRECTION_LEFT, -20, 0},
		} {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				th := newTestHarness(t)
				larBot := th.bot
				larBot.clientClickMode = pb.ClickMode_CLICK_MODE_MOVE
				ret, err := larBot.HandleClick(context.Background(), tc.x, tc.y, tc.viewWidth, tc.viewHeight)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, ret, test.ShouldContainSubstring, fmt.Sprintf("(%d, %d)", tc.expectedX, tc.expectedY))
				test.That(t, ret, test.ShouldContainSubstring, tc.expectedDir.String())
			})
		}
	})

	t.Run("info mode", func(t *testing.T) {
		th := newTestHarness(t)
		larBot := th.bot
		larBot.clientClickMode = pb.ClickMode_CLICK_MODE_INFO
		larBot.rootArea.Mutate(func(area MutableArea) {
			test.That(t, area.Set(-200, -300, 3), test.ShouldBeNil)
		})

		for i, tc := range []struct {
			x, y                  int
			viewWidth, viewHeight int
			object                bool
			angleCenter           float64
			distanceCenter        int
			distanceFront         int
		}{
			{0, 0, 10, 10, false, 315, 707, 686},
			{5, 0, 10, 10, false, 0, 500, 470},
			{3, 2, 10, 10, true, 326.309932, 360, 336},
		} {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				ret, err := larBot.HandleClick(context.Background(), tc.x, tc.y, tc.viewWidth, tc.viewHeight)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, ret, test.ShouldContainSubstring, fmt.Sprintf("object=%t", tc.object))
				test.That(t, ret, test.ShouldContainSubstring, fmt.Sprintf("angleCenter=%f", tc.angleCenter))
				test.That(t, ret, test.ShouldContainSubstring, fmt.Sprintf("distanceCenter=%dcm", tc.distanceCenter))
				test.That(t, ret, test.ShouldContainSubstring, fmt.Sprintf("distanceFront=%dcm", tc.distanceFront))
			})
		}
	})
}
