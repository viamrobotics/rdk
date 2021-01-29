package slam

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/echolabsinc/robotcore/robots/fake"
)

// TODO(erd): I don't think I want this file to exist in favor of some other way of controlling

func (lar *LocationAwareRobot) HandleData(data []byte, respondMsg func(msg string)) error {
	if bytes.HasPrefix(data, []byte("move: ")) {
		dir := direction(bytes.TrimPrefix(data, []byte("move: ")))
		amount := 100
		if err := lar.move(&amount, &dir); err != nil {
			return err
		}
		respondMsg(fmt.Sprintf("moved %q", dir))
		respondMsg(lar.basePosString())
	} else if bytes.HasPrefix(data, []byte("rotate_to ")) {
		dir := direction(bytes.TrimPrefix(data, []byte("rotate_to ")))
		if err := lar.rotateTo(dir); err != nil {
			return err
		}
		respondMsg(fmt.Sprintf("rotate to %q", dir))
	} else if bytes.Equal(data, []byte("pos")) {
		respondMsg(lar.basePosString())
	} else if bytes.HasPrefix(data, []byte("sv_device_offset ")) {
		offsetStr := string(bytes.TrimPrefix(data, []byte("sv_device_offset ")))
		offsetSplit := strings.SplitN(offsetStr, " ", 2)
		if len(offsetSplit) != 2 {
			return errors.New("malformed offset")
		}
		offsetNum, err := strconv.ParseInt(offsetSplit[0], 10, 64)
		if err != nil {
			return err
		}
		if offsetNum < 0 || int(offsetNum) > len(lar.deviceOffsets) {
			return errors.New("bad offset number")
		}
		split := strings.Split(offsetSplit[1], ",")
		if len(split) != 3 {
			return errors.New("offset format is angle,x,y")
		}
		angle, err := strconv.ParseFloat(split[0], 64)
		if err != nil {
			return err
		}
		distX, err := strconv.ParseFloat(split[1], 64)
		if err != nil {
			return err
		}
		distY, err := strconv.ParseFloat(split[2], 64)
		if err != nil {
			return err
		}
		lar.deviceOffsets[offsetNum] = DeviceOffset{angle, distX, distY}
		return nil
	} else if bytes.HasPrefix(data, []byte("sv_lidar_stop ")) {
		lidarDeviceStr := string(bytes.TrimPrefix(data, []byte("sv_lidar_stop ")))
		lidarDeviceNum, err := strconv.ParseInt(lidarDeviceStr, 10, 32)
		if err != nil {
			return err
		}
		if lidarDeviceNum < 0 || lidarDeviceNum >= int64(len(lar.devices)) {
			return errors.New("invalid device")
		}
		lar.devices[lidarDeviceNum].Stop()
		respondMsg(fmt.Sprintf("lidar %d stopped", lidarDeviceNum))
	} else if bytes.HasPrefix(data, []byte("sv_lidar_start ")) {
		lidarDeviceStr := string(bytes.TrimPrefix(data, []byte("sv_lidar_start ")))
		lidarDeviceNum, err := strconv.ParseInt(lidarDeviceStr, 10, 32)
		if err != nil {
			return err
		}
		if lidarDeviceNum < 0 || lidarDeviceNum >= int64(len(lar.devices)) {
			return errors.New("invalid device")
		}
		lar.devices[lidarDeviceNum].Start()
		respondMsg(fmt.Sprintf("lidar %d started", lidarDeviceNum))
	} else if bytes.HasPrefix(data, []byte("sv_lidar_seed ")) {
		seedStr := string(bytes.TrimPrefix(data, []byte("sv_lidar_seed ")))
		seed, err := strconv.ParseInt(seedStr, 10, 32)
		if err != nil {
			return err
		}
		if fake, ok := lar.devices[0].(*fake.Lidar); ok {
			fake.SetSeed(seed)
		}
		respondMsg(seedStr)
	} else if bytes.HasPrefix(data, []byte("cl_zoom ")) {
		zoomStr := string(bytes.TrimPrefix(data, []byte("cl_zoom ")))
		zoom, err := strconv.ParseFloat(zoomStr, 64)
		if err != nil {
			return err
		}
		if zoom < 1 {
			return errors.New("zoom must be >= 1")
		}
		lar.clientZoom = zoom
		respondMsg(zoomStr)
	} else if bytes.HasPrefix(data, []byte("cl_lidar_view")) {
		lidarDeviceStr := string(bytes.TrimSpace(bytes.TrimPrefix(data, []byte("cl_lidar_view"))))
		if lidarDeviceStr == "" {
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
			respondMsg(devicesStr)
			return nil
		}
		if lidarDeviceStr == "combined" {
			lar.setClientDeviceNumber(-1)
			return nil
		}
		lidarDeviceNum, err := strconv.ParseInt(lidarDeviceStr, 10, 32)
		if err != nil {
			return err
		}
		if lidarDeviceNum < 0 || lidarDeviceNum >= int64(len(lar.devices)) {
			return errors.New("invalid device")
		}
		lar.setClientDeviceNumber(int(lidarDeviceNum))
	}
	return nil
}

func (lar *LocationAwareRobot) HandleClick(x, y, sX, sY int, respondMsg func(msg string)) error {
	centerX := sX / 2
	centerY := sX / 2

	var rotateTo direction
	if x < centerX {
		if y < centerY {
			rotateTo = directionUp
		} else {
			rotateTo = directionLeft
		}
	} else {
		if y < centerY {
			rotateTo = directionDown
		} else {
			rotateTo = directionRight
		}
	}

	amount := 100
	if err := lar.move(&amount, &rotateTo); err != nil {
		return err
	}
	respondMsg(fmt.Sprintf("moved %q", rotateTo))
	respondMsg(lar.basePosString())
	return nil
}

func (lar *LocationAwareRobot) Close() {

}
