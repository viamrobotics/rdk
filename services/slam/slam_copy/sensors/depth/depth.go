// Package depth implements the Depth sensor
package depth

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/services/slam/slam_copy/sensors/utils"
)

// Depth represents a depth sensor.
type Depth struct {
	Name  string
	depth camera.Camera
}

// New creates a new Depth sensor based on the sensor definition and the service config.
func New(deps registry.Dependencies, sensors []string, sensorIndex int) (Depth, error) {
	name, err := utils.GetName(sensors, sensorIndex)
	if err != nil {
		return Depth{}, err
	}

	newDepth, err := camera.FromDependencies(deps, name)
	if err != nil {
		return Depth{}, errors.Wrapf(err, "error getting camera %v for slam service", name)
	}

	return Depth{
		Name:  name,
		depth: newDepth,
	}, nil
}

// GetData returns data from the depth sensor. The returned function is a release function
// that must be called once the caller of GetData is done using the image.
func (depth Depth) GetData(ctx context.Context) ([]byte, func(), error) {
	return utils.GetPNGImage(ctx, depth.depth)
}
