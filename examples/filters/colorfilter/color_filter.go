// Package colorfilter implements a modular camera that filters the output of an underlying camera and only keeps
// captured data if the vision service detects a certain color in the captured image.
package colorfilter

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"github.com/viamrobotics/gostream"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/services/vision"
)

var (
	// Model is the full model definition.
	Model            = resource.NewModel("example", "camera", "colorfilter")
	errUnimplemented = errors.New("unimplemented")
)

func init() {
	resource.RegisterComponent(camera.API, Model, resource.Registration[camera.Camera, *Config]{
		Constructor: newCamera,
	})
}

func newCamera(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (camera.Camera, error) {
	c := &colorFilterCam{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}
	if err := c.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return c, nil
}

// Config contains the name to the underlying camera and the name of the vision service to be used.
type Config struct {
	ActualCam     string `json:"actual_cam"`
	VisionService string `json:"vision_service"`
}

// Validate validates the config and returns implicit dependencies.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.ActualCam == "" {
		return nil, fmt.Errorf(`expected "actual_cam" attribute in %q`, path)
	}
	if cfg.VisionService == "" {
		return nil, fmt.Errorf(`expected "vision_service" attribute in %q`, path)
	}

	return []string{cfg.ActualCam, cfg.VisionService}, nil
}

// A colorFilterCam wraps the underlying camera `actualCam` and only keeps the data captured on the actual camera if `visionService`
// detects a certain color in the captured image.
type colorFilterCam struct {
	resource.Named
	actualCam     camera.Camera
	visionService vision.Service
	logger        golog.Logger
}

// Reconfigure reconfigures the modular component with new settings.
func (c *colorFilterCam) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	camConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	c.actualCam, err = camera.FromDependencies(deps, camConfig.ActualCam)
	if err != nil {
		return errors.Wrapf(err, "unable to get camera %v for colorfilter", camConfig.ActualCam)
	}

	c.visionService, err = vision.FromDependencies(deps, camConfig.VisionService)
	if err != nil {
		return errors.Wrapf(err, "unable to get vision service %v for colorfilter", camConfig.VisionService)
	}

	return nil
}

// DoCommand simply echoes whatever was sent.
func (c *colorFilterCam) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

// Close closes the underlying camera.
func (c *colorFilterCam) Close(ctx context.Context) error {
	return c.actualCam.Close(ctx)
}

// Images does nothing.
func (c *colorFilterCam) Images(ctx context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	return nil, resource.ResponseMetadata{}, errUnimplemented
}

// Stream returns a stream that filters the output of the underlying camera stream in the stream.Next method.
func (c *colorFilterCam) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	camStream, err := c.actualCam.Stream(ctx, errHandlers...)
	if err != nil {
		return nil, err
	}

	return filterStream{camStream, c.visionService}, nil
}

// NextPointCloud does nothing.
func (c *colorFilterCam) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	return nil, errUnimplemented
}

// Properties returns details about the camera.
func (c *colorFilterCam) Properties(ctx context.Context) (camera.Properties, error) {
	return c.actualCam.Properties(ctx)
}

// Projector does nothing.
func (c *colorFilterCam) Projector(ctx context.Context) (transform.Projector, error) {
	return nil, errUnimplemented
}

type filterStream struct {
	cameraStream  gostream.VideoStream
	visionService vision.Service
}

// Next contains the filtering logic and returns select data from the underlying camera.
func (fs filterStream) Next(ctx context.Context) (image.Image, func(), error) {
	if ctx.Value(data.FromDMContextKey{}) != true {
		// If not data management collector, return underlying stream contents without filtering.
		return fs.cameraStream.Next(ctx)
	}

	// Only return captured image if it contains a certain color set by the vision service.
	img, release, err := fs.cameraStream.Next(ctx)
	if err != nil {
		return nil, nil, errors.New("could not get next source image")
	}
	detections, err := fs.visionService.Detections(ctx, img, map[string]interface{}{})
	if err != nil {
		return nil, nil, errors.New("could not get detections")
	}

	if len(detections) == 0 {
		return nil, nil, data.ErrNoCaptureToStore
	}

	return img, release, err
}

// Close closes the stream.
func (fs filterStream) Close(ctx context.Context) error {
	return fs.cameraStream.Close(ctx)
}
