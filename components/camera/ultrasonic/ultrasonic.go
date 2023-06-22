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

// Config is used for converting config attributes.
// type Config struct {
// 	TriggerPin    string `json:"trigger_pin"`
// 	EchoInterrupt string `json:"echo_interrupt_pin"`
// 	Board         string `json:"board"`
// 	TimeoutMs     uint   `json:"timeout_ms,omitempty"`
// }

type ultrasonicWrapper struct {
	usSensor sensor.Sensor
}

// pointcloud and read on usCam struct

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

				// usSensor, err := sen

				usSensor, err := ultrasense.NewSensor(ctx, deps, conf.ResourceName(), newConf)
				if err != nil {
					return nil, err
				}
				usWrapper := ultrasonicWrapper{usSensor: usSensor}
				usVideoSource, err := camera.NewVideoSourceFromReader(ctx, &usWrapper, nil, camera.UnspecifiedStream)

				return camera.FromVideoSource(conf.ResourceName(), usVideoSource), nil
				// struct embedding
			},
		})
}

// Stream returns a stream that makes a best effort to return consecutive images
// that may have a MIME type hint dictated in the context via gostream.WithMIMETypeHint.
// videosource from reader should make stream function
func (usvs *ultrasonicWrapper) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	return nil, errors.New("Not yet implemented")
}

// NextPointCloud returns the next immediately available point cloud, not necessarily one
// a part of a sequence. In the future, there could be streaming of point clouds.
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
	basicData.SetValue(1)
	distVector := pointCloud.NewVector(0, 0, distFloat)
	fmt.Println("Added point at ", distVector)
	pcToReturn.Set(distVector, basicData)

	return pcToReturn, nil
}

// Properties returns properties that are intrinsic to the particular
// implementation of a camera
func (usvs *ultrasonicWrapper) Properties(ctx context.Context) (camera.Properties, error) {
	return camera.Properties{SupportsPCD: true, ImageType: camera.UnspecifiedStream}, nil
}

func (usvs *ultrasonicWrapper) Close(ctx context.Context) error {
	usvs.usSensor.Close(ctx)
	return nil
}

func (usvs *ultrasonicWrapper) Read(ctx context.Context) (image.Image, func(), error) {
	return nil, nil, errors.New("Not yet implemented")
}

// func newCamera(ctx context.Context, deps resource.Dependencies, name resource.Name, config *ultrasense.Config) (camera.Camera, error) {
// 	usCam := &ultraSonicCamera{
// 		Named:  name.AsNamed(),
// 		config: config,
// 	}

// 	cancelCtx, cancelFunc := context.WithCancel(context.Background())
// 	s.cancelCtx = cancelCtx
// 	s.cancelFunc = cancelFunc

// }

// Validate function should already be defined in sensor/ultrasonic.go
// func (conf *ultrasense.Config) Validate(path string) ([]string, error) {
// 	return conf.Validate(path)
// }
