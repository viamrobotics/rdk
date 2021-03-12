package lidar

import (
	"context"
	"fmt"
	"math"
	"strings"
)

// BestAngularResolution returns the best angular resolution from the given devices.
func BestAngularResolution(ctx context.Context, lidarDevices []Device) (float64, Device, int, error) {
	best := math.MaxFloat64
	deviceNum := 0
	for i, lidarDev := range lidarDevices {
		angRes, err := lidarDev.AngularResolution(ctx)
		if err != nil {
			return math.NaN(), nil, 0, err
		}
		if angRes < best {
			best = angRes
			deviceNum = i
		}
	}
	return best, lidarDevices[deviceNum], deviceNum, nil
}

// ParseDeviceFlag parses a device flag from command line arguments.
func ParseDeviceFlag(flag string, flagName string) (DeviceDescription, error) {
	deviceFlagParts := strings.Split(flag, ",")
	if len(deviceFlagParts) != 2 {
		return DeviceDescription{}, fmt.Errorf("wrong device format; use --%s=type,path", flagName)
	}
	return DeviceDescription{
		Type: DeviceType(deviceFlagParts[0]), // TODO(erd): validate?
		Path: deviceFlagParts[1],
	}, nil
}

// ParseDeviceFlags parses device flags from command line arguments.
func ParseDeviceFlags(flags []string, flagName string) ([]DeviceDescription, error) {
	deviceDescs := make([]DeviceDescription, 0, len(flags))
	for _, deviceFlag := range flags {
		desc, err := ParseDeviceFlag(deviceFlag, flagName)
		if err != nil {
			return nil, err
		}
		deviceDescs = append(deviceDescs, desc)
	}
	return deviceDescs, nil
}
