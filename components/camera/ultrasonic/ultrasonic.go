package ultrasonic

import (
	"context"
	"errors"
	"fmt"
	"image"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/sensor"
	ultrasense "go.viam.com/rdk/components/sensor/ultrasonic"
	pointCloud "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"

	"github.com/edaniels/golog"
	"github.com/viamrobotics/gostream"
)

var model = resource.DefaultModelFamily.WithModel("ultrasonic")

type ultrasonicWrapper struct {
	usSensor sensor.Sensor
}

func init() {
	resource.RegisterComponent(
		camera.API,
		model,
		resource.Registration[camera.Camera, *ultrasense.Config]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (camera.Camera, error) {
				newConf, err := resource.NativeConfig[*ultrasense.Config](conf)
				if err != nil {
					return nil, err
				}
				return newCamera(ctx, deps, conf.ResourceName(), newConf, logger)
			},
		})
}

func newCamera(ctx context.Context, deps resource.Dependencies, name resource.Name, newConf *ultrasense.Config, logger golog.Logger) (camera.Camera, error) {

	// sns, err := sensor.FromDependencies(deps, conf.Name)
	// if err != nil {
	// 	return nil, err
	// }
	// usWrapper := ultrasonicWrapper{usSensor: sns}
	fmt.Print("resource.Name:", name.API)
	fmt.Print("resource.Name:", name.Name)
	fmt.Print("resource.Name:", name.Remote)

	usSensor, err := ultrasense.NewSensor(ctx, deps, name, newConf)
	if err != nil {
		return nil, err
	}
	usWrapper := ultrasonicWrapper{usSensor: usSensor}

	usVideoSource, err := camera.NewVideoSourceFromReader(ctx, &usWrapper, nil, camera.UnspecifiedStream)
	if err != nil {
		return nil, err
	}

	return camera.FromVideoSource(name, usVideoSource), nil
}

func (usvs *ultrasonicWrapper) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	return nil, errors.New("Not yet implemented")
}

func (usvs *ultrasonicWrapper) NextPointCloud(ctx context.Context) (pointCloud.PointCloud, error) {

	readings, err := usvs.usSensor.Readings(ctx, make(map[string]interface{}))
	if err != nil {
		return nil, err
	}
	pcToReturn := pointCloud.New()
	distFloat, ok := readings["distance"].(float64)
	if !ok {
		return nil, errors.New("unable to convert distance to float64")
	}
	basicData := pointCloud.NewBasicData()
	distVector := pointCloud.NewVector(0, 0, distFloat)
	pcToReturn.Set(distVector, basicData)

	return pcToReturn, nil
}

func (usvs *ultrasonicWrapper) Properties(ctx context.Context) (camera.Properties, error) {
	return camera.Properties{SupportsPCD: true, ImageType: camera.UnspecifiedStream}, nil
}

func (usvs *ultrasonicWrapper) Close(ctx context.Context) error {
	usvs.usSensor.Close(ctx)
	return nil
}

func (usvs *ultrasonicWrapper) Read(ctx context.Context) (image.Image, func(), error) {
	return nil, nil, errors.New("not yet implemented")
}
