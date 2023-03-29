// Package rgb implements the RGB sensor
package rgb

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/services/slam/slam_copy/sensors/utils"
)

// RGB represents an RGB sensor.
type RGB struct {
	Name string
	rgb  camera.Camera
}

// New creates a new RGB sensor based on the sensor definition and the service config.
func New(ctx context.Context, deps registry.Dependencies, sensors []string, sensorIndex int) (RGB, error) {
	name, err := utils.GetName(sensors, sensorIndex)
	if err != nil {
		return RGB{}, err
	}

	newRGB, err := camera.FromDependencies(deps, name)
	if err != nil {
		return RGB{}, errors.Wrapf(err, "error getting camera %v for slam service", name)
	}

	if err = validate(ctx, newRGB); err != nil {
		return RGB{}, err
	}

	return RGB{
		Name: name,
		rgb:  newRGB,
	}, nil
}

// GetData returns data from the RGB sensor. The returned function is a release function
// that must be called once the caller of GetData is done using the image.
func (rgb RGB) GetData(ctx context.Context) ([]byte, func(), error) {
	return utils.GetPNGImage(ctx, rgb.rgb)
}

func validate(ctx context.Context, rgb camera.Camera) error {
	proj, err := rgb.Projector(ctx)
	if err != nil {
		return errors.Wrap(err,
			"Unable to get camera features for first camera, make sure the color camera is listed first")
	}
	intrinsics, ok := proj.(*transform.PinholeCameraIntrinsics)
	if !ok {
		return transform.NewNoIntrinsicsError("Intrinsics do not exist")
	}

	err = intrinsics.CheckValid()
	if err != nil {
		return err
	}

	props, err := rgb.Properties(ctx)
	if err != nil {
		return errors.Wrap(err, "error getting camera properties for slam service")
	}

	brownConrady, ok := props.DistortionParams.(*transform.BrownConrady)
	if !ok {
		return errors.New("error getting distortion_parameters for slam service, only BrownConrady distortion parameters are supported")
	}
	if err := brownConrady.CheckValid(); err != nil {
		return errors.Wrapf(err, "error validating distortion_parameters for slam service")
	}
	return nil
}
