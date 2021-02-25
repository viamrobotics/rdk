package slam

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/viamrobotics/robotcore/base"
	"github.com/viamrobotics/robotcore/robots/fake"
	"github.com/viamrobotics/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
)

const (
	commandRobotMove         = "robot_move"
	commandRobotMoveForward  = "robot_move_forward"
	commandRobotMoveBackward = "robot_move_backward"
	commandRobotTurnTo       = "robot_turn_to"
	commandRobotStats        = "robot_stats"
	commandRobotDeviceOffset = "robot_device_offset"
	commandRobotLidarStart   = "robot_lidar_start"
	commandRobotLidarStop    = "robot_lidar_stop"
	commandRobotLidarSeed    = "robot_lidar_seed"
	commandClientClickMode   = "cl_click_mode"
	commandClientZoom        = "cl_zoom"
	commandLidarView         = "cl_lidar_view"
	commandLidarViewMode     = "cl_lidar_view_mode"
	commandCalibrate         = "calibrate"
	commandSave              = "save"
)

const (
	clientClickModeMove = "move"
	clientClickModeInfo = "info"
)

const (
	clientLidarViewModeStored = "stored"
	clientLidarViewModeLive   = "live"
)

const defaultClientMoveAmount = 20

func (lar *LocationAwareRobot) RegisterCommands(registry gostream.CommandRegistry) {
	registry.Add(commandSave, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		lar.serverMu.Lock()
		defer lar.serverMu.Unlock()
		if len(cmd.Args) == 0 {
			return nil, errors.New("file to save to required")
		}
		return nil, lar.rootArea.WriteToFile(cmd.Args[0])
	})
	registry.Add(commandCalibrate, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		lar.serverMu.Lock()
		defer lar.serverMu.Unlock()
		if lar.compassSensor != nil {
			golog.Global.Info("calibrating compass")
			if err := lar.compassSensor.StartCalibration(context.TODO()); err != nil {
				return nil, err
			}
		}
		step := 10.0
		for i := 0.0; i < 360; i += step {
			if err := base.Reduce(lar.baseDevice).Spin(step, 0, true); err != nil {
				return nil, err
			}
		}
		if lar.compassSensor != nil {
			if err := lar.compassSensor.StopCalibration(context.TODO()); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	registry.Add(commandLidarViewMode, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		if len(cmd.Args) == 0 {
			return nil, fmt.Errorf("mode required: [%s, %s]", clientLidarViewModeStored, clientLidarViewModeLive)
		}
		switch cmd.Args[0] {
		case clientLidarViewModeStored, clientLidarViewModeLive:
			lar.clientLidarViewMode = cmd.Args[0]
		default:
			return nil, fmt.Errorf("unknown mode %q", cmd.Args[0])
		}
		return nil, nil
	})
	registry.Add(commandClientClickMode, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		if len(cmd.Args) == 0 {
			return nil, fmt.Errorf("mode required: [%s, %s]", clientClickModeMove, clientClickModeInfo)
		}
		switch cmd.Args[0] {
		case clientClickModeMove, clientClickModeInfo:
			lar.clientClickMode = cmd.Args[0]
		default:
			return nil, fmt.Errorf("unknown mode %q", cmd.Args[0])
		}
		return nil, nil
	})
	registry.Add(commandRobotMove, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		if len(cmd.Args) == 0 {
			return nil, fmt.Errorf("move direction required: [%s, %s, %s, %s]",
				DirectionUp, DirectionRight, DirectionDown, DirectionLeft)
		}
		dir := Direction(cmd.Args[0])
		amount := defaultClientMoveAmount
		if err := lar.Move(&amount, &dir); err != nil {
			return nil, err
		}
		return gostream.NewCommandResponseText(fmt.Sprintf("moved %q\n%s", dir, lar)), nil
	})
	registry.Add(commandRobotMoveForward, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		amount := defaultClientMoveAmount
		if err := lar.Move(&amount, nil); err != nil {
			return nil, err
		}
		return gostream.NewCommandResponseText(fmt.Sprintf("moved forwards\n%s", lar)), nil
	})
	registry.Add(commandRobotMoveBackward, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		amount := -defaultClientMoveAmount
		if err := lar.Move(&amount, nil); err != nil {
			return nil, err
		}
		return gostream.NewCommandResponseText(fmt.Sprintf("moved backwards\n%s", lar)), nil
	})
	registry.Add(commandRobotTurnTo, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		if len(cmd.Args) == 0 {
			return nil, fmt.Errorf("rotation direction required: [%s, %s, %s, %s]",
				DirectionUp, DirectionRight, DirectionDown, DirectionLeft)
		}
		dir := Direction(cmd.Args[0])
		if err := lar.rotateTo(dir); err != nil {
			return nil, err
		}
		return gostream.NewCommandResponseText(fmt.Sprintf("rotate to %q", dir)), nil
	})
	registry.Add(commandRobotStats, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		return gostream.NewCommandResponseText(lar.String()), nil
	})
	registry.Add(commandRobotDeviceOffset, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		if len(cmd.Args) < 2 {
			return nil, errors.New("offset number and parameters required (e.g. 0 -90.2,1.2,9.7)")
		}

		offsetNum, err := strconv.ParseInt(cmd.Args[0], 10, 64)
		if err != nil {
			return nil, err
		}
		if offsetNum < 0 || int(offsetNum) > len(lar.deviceOffsets) {
			return nil, errors.New("bad offset nil, number")
		}
		split := strings.Split(cmd.Args[1], ",")
		if len(split) != 3 {
			return nil, errors.New("offset format is angle,x,y")
		}
		angle, err := strconv.ParseFloat(split[0], 64)
		if err != nil {
			return nil, err
		}
		distX, err := strconv.ParseFloat(split[1], 64)
		if err != nil {
			return nil, err
		}
		distY, err := strconv.ParseFloat(split[2], 64)
		if err != nil {
			return nil, err
		}
		lar.deviceOffsets[offsetNum] = DeviceOffset{angle, distX, distY}
		return nil, nil
	})
	registry.Add(commandRobotLidarStart, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		if len(cmd.Args) == 0 {
			return nil, errors.New("device number required")
		}

		lidarDeviceNum, err := lar.parseDeviceNumber(cmd.Args[0])
		if err != nil {
			return nil, err
		}
		if err := lar.devices[lidarDeviceNum].Start(context.TODO()); err != nil {
			return nil, err
		}
		return gostream.NewCommandResponseText(fmt.Sprintf("lidar %d stopped", lidarDeviceNum)), nil
	})
	registry.Add(commandRobotLidarStop, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		if len(cmd.Args) == 0 {
			return nil, errors.New("device number required")
		}

		lidarDeviceNum, err := lar.parseDeviceNumber(cmd.Args[0])
		if err != nil {
			return nil, err
		}
		if err := lar.devices[lidarDeviceNum].Stop(context.TODO()); err != nil {
			return nil, err
		}
		return gostream.NewCommandResponseText(fmt.Sprintf("lidar %d stopped", lidarDeviceNum)), nil
	})
	registry.Add(commandRobotLidarSeed, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		if len(cmd.Args) == 0 {
			seeds := make([]string, len(lar.devices))
			for i, dev := range lar.devices {
				if fake, ok := dev.(*fake.Lidar); ok {
					seeds[i] = fmt.Sprintf("%d", fake.Seed())
				} else {
					seeds[i] = "real-device"
				}
			}
			return gostream.NewCommandResponseText(strings.Join(seeds, ",")), nil
		}

		if len(cmd.Args) < 2 {
			return nil, errors.New("device number and seed required")
		}

		lidarDeviceNum, err := lar.parseDeviceNumber(cmd.Args[0])
		if err != nil {
			return nil, err
		}

		seed, err := strconv.ParseInt(cmd.Args[1], 10, 32)
		if err != nil {
			return nil, err
		}
		if fake, ok := lar.devices[lidarDeviceNum].(*fake.Lidar); ok {
			fake.SetSeed(seed)
			return gostream.NewCommandResponseText(cmd.Args[1]), nil
		}
		return nil, errors.New("cannot set seed on real device")
	})
	registry.Add(commandClientZoom, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		if len(cmd.Args) == 0 {
			return nil, errors.New("zoom level required")
		}
		zoom, err := strconv.ParseFloat(cmd.Args[0], 64)
		if err != nil {
			return nil, err
		}
		if zoom < 1 {
			return nil, errors.New("zoom must be >= 1")
		}
		lar.clientZoom = zoom
		return gostream.NewCommandResponseText(cmd.Args[0]), nil
	})
	registry.Add(commandLidarView, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		if len(cmd.Args) == 0 {
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
			return gostream.NewCommandResponseText(devicesStr), nil
		}

		if cmd.Args[0] == "combined" {
			lar.setClientDeviceNumber(-1)
			return nil, nil
		}
		lidarDeviceNum, err := lar.parseDeviceNumber(cmd.Args[0])
		if err != nil {
			return nil, err
		}
		lar.setClientDeviceNumber(int(lidarDeviceNum))
		return nil, nil
	})
}

func (lar *LocationAwareRobot) parseDeviceNumber(text string) (int64, error) {
	lidarDeviceNum, err := strconv.ParseInt(text, 10, 32)
	if err != nil {
		return 0, err
	}
	if lidarDeviceNum < 0 || lidarDeviceNum >= int64(len(lar.devices)) {
		return 0, errors.New("invalid device number")
	}
	return lidarDeviceNum, nil
}

func (lar *LocationAwareRobot) HandleClick(x, y, viewWidth, viewHeight int) (string, error) {
	switch lar.clientClickMode {
	case clientClickModeMove:
		dir := DirectionFromXY(x, y, viewWidth, viewHeight)
		amount := 20
		if err := lar.Move(&amount, &dir); err != nil {
			return "", err
		}
		return fmt.Sprintf("moved %q\n%s", dir, lar), nil
	case clientClickModeInfo:
		// TODO(erd): refactor to viewCoordToReal
		_, bounds, areas, err := lar.areasToView()
		if err != nil {
			return "", err
		}

		_, scaleDown := areas[0].Size()
		bounds.X = int(math.Ceil(float64(bounds.X) * float64(scaleDown) / lar.clientZoom))
		bounds.Y = int(math.Ceil(float64(bounds.Y) * float64(scaleDown) / lar.clientZoom))

		basePosX, basePosY := lar.basePos()
		minX := basePosX - bounds.X/2
		minY := basePosY - bounds.Y/2

		areaX := minX + int(float64(bounds.X)*(float64(x)/float64(viewWidth)))
		areaY := minY + int(float64(bounds.Y)*(float64(y)/float64(viewHeight)))

		distanceCenterF := math.Sqrt(float64(((areaX - basePosX) * (areaX - basePosX)) + ((areaY - basePosY) * (areaY - basePosY))))
		distanceCenter := int(distanceCenterF)
		baseWidthScaled := baseWidthMeters * float64(scaleDown)
		frontY := basePosY - int(baseWidthScaled/2)
		distanceFront := int(math.Sqrt(float64(((areaX - basePosX) * (areaX - basePosX)) + ((areaY - frontY) * (areaY - frontY)))))

		xForAngle := (areaX - basePosX)
		yForAngle := (areaY - basePosY)
		yForAngle *= -1
		angelCenterRad := math.Atan2(float64(xForAngle), float64(yForAngle))
		angleCenter := utils.RadToDeg(angelCenterRad)
		if angleCenter < 0 {
			angleCenter = 360 + angleCenter
		}

		var present bool
		for _, area := range areas {
			area.Mutate(func(area MutableArea) {
				present = area.At(areaX, areaY) != 0
			})
			if present {
				break
			}
		}

		return fmt.Sprintf("(%d,%d): object=%t, angleCenter=%f,%f, distanceCenter=%dcm distanceFront=%dcm", areaX, areaY, present, angleCenter, angelCenterRad, distanceCenter, distanceFront), nil
	default:
		return "", fmt.Errorf("do not know how to handle click in mode %q", lar.clientClickMode)
	}
}
