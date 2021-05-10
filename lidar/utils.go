package lidar

import (
	"context"
	"math"
)

// BestAngularResolution returns the best angular resolution from the given devices.
func BestAngularResolution(ctx context.Context, lidars []Device) (float64, Device, int, error) {
	best := math.MaxFloat64
	deviceNum := 0
	for i, lidarDev := range lidars {
		angRes, err := lidarDev.AngularResolution(ctx)
		if err != nil {
			return math.NaN(), nil, 0, err
		}
		if angRes < best {
			best = angRes
			deviceNum = i
		}
	}
	return best, lidars[deviceNum], deviceNum, nil
}
