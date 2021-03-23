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

func (desc *DeviceDescription) String() string {
	return fmt.Sprintf("%#v", desc)
}

func (desc *DeviceDescription) Set(val string) error {
	parsed, err := ParseDeviceFlag("", val)
	if err != nil {
		return err
	}
	*desc = parsed
	return nil
}

func (desc *DeviceDescription) Get() interface{} {
	return desc
}

// ParseDeviceFlag parses a device flag from command line arguments.
func ParseDeviceFlag(flagName, flag string) (DeviceDescription, error) {
	deviceFlagParts := strings.Split(flag, ",")
	if len(deviceFlagParts) != 2 {
		return DeviceDescription{}, fmt.Errorf("wrong device format; use --%s=type,path", flagName)
	}
	return DeviceDescription{
		Type: DeviceType(deviceFlagParts[0]),
		Path: deviceFlagParts[1],
	}, nil
}

// ParseDeviceFlags parses device flags from command line arguments.
func ParseDeviceFlags(flagName string, flags []string) ([]DeviceDescription, error) {
	deviceDescs := make([]DeviceDescription, 0, len(flags))
	for _, deviceFlag := range flags {
		desc, err := ParseDeviceFlag(flagName, deviceFlag)
		if err != nil {
			return nil, err
		}
		deviceDescs = append(deviceDescs, desc)
	}
	return deviceDescs, nil
}
