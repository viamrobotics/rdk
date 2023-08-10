// Package colorfilter implements a modular camera that filters the output of an underlying camera and only keeps
// captured data if the vision service detects a certain color in the captured image.
package colorfilter

import (
	"context"
	"fmt"
	"image"
	"time"

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
	Model            = resource.NewModel("example", "filter", "colorfilter")
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

type Config struct {
	ActualCam     string `json:"actual_cam"`
	VisionService string `json:"vision_service"`
}

func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.ActualCam == "" {
		return nil, fmt.Errorf(`expected "actual_cam" attribute in %q`, path)
	}

	return []string{cfg.ActualCam}, nil
}

type colorFilterCam struct {
	resource.Named
	actualCam     camera.Camera
	visionService vision.Service
	logger        golog.Logger
}

// resource.Resource methods
func (c *colorFilterCam) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	c.actualCam = nil
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

func (c *colorFilterCam) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

func (c *colorFilterCam) Close(ctx context.Context) error {
	return c.actualCam.Close(ctx)
}

// VideoStream methods
func (c *colorFilterCam) Images(ctx context.Context) ([]image.Image, time.Time, error) {
	return nil, time.Time{}, errUnimplemented
}

func (c *colorFilterCam) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	camStream, err := c.actualCam.Stream(ctx, errHandlers...)
	if err != nil {
		return nil, err
	}

	return filterStream{camStream, c.visionService}, nil
}

func (c *colorFilterCam) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	return nil, errUnimplemented
}

func (c *colorFilterCam) Properties(ctx context.Context) (camera.Properties, error) {
	return camera.Properties{}, nil
}

// Projector methods
func (c *colorFilterCam) Projector(ctx context.Context) (transform.Projector, error) {
	return nil, errUnimplemented
}

// Filter code:
type filterStream struct {
	cameraStream  gostream.VideoStream
	visionService vision.Service
}

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

func (fs filterStream) Close(ctx context.Context) error {
	return fs.cameraStream.Close(ctx)
}
