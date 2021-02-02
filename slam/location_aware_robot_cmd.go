package slam

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/viamrobotics/robotcore/robots/fake"

	"github.com/edaniels/gostream"
)

const (
	commandRobotMove         = "robot_move"
	commandRobotRotateTo     = "robot_rotate_to"
	commandRobotStats        = "robot_stats"
	commandRobotDeviceOffset = "robot_device_offset"
	commandRobotLidarStart   = "robot_lidar_start"
	commandRobotLidarStop    = "robot_lidar_stop"
	commandRobotLidarSeed    = "robot_lidar_seed"
	commandClientZoom        = "cl_zoom"
	commandLidarView         = "cl_lidar_view"
)

func (lar *LocationAwareRobot) RegisterCommands(registry gostream.CommandRegistry) {
	registry.Add(commandRobotMove, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
		if len(cmd.Args) == 0 {
			return nil, fmt.Errorf("move direction required: [%s, %s, %s, %s]",
				DirectionUp, DirectionRight, DirectionDown, DirectionLeft)
		}
		dir := Direction(cmd.Args[0])
		amount := 100
		if err := lar.Move(&amount, &dir); err != nil {
			return nil, err
		}
		return gostream.NewCommandResponseText(fmt.Sprintf("moved %q\n%s", dir, lar)), nil
	})
	registry.Add(commandRobotRotateTo, func(cmd *gostream.Command) (*gostream.CommandResponse, error) {
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
		lar.devices[lidarDeviceNum].Start()
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
		lar.devices[lidarDeviceNum].Stop()
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
