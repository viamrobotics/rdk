// Package katcam implements a base that only supports SetPower (basic forward/back/turn controls.)
package katcam

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
	// Model is the full model definition.
	Model            = resource.NewModel("filters", "demo", "katcam")
	errUnimplemented = errors.New("unimplemented")
)

func init() {
	resource.RegisterComponent(camera.API, Model, resource.Registration[camera.Camera, *Config]{
		Constructor: newCamera,
	})
}

func newCamera(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (camera.Camera, error) {
	c := &katCam{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
		// count:  new(int32),
	}
	if err := c.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return c, nil
}

type Config struct {
	ActualCam     string `json:"actualCam"`
	VisionService string `json:"visionService"`
}

func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.ActualCam == "" {
		return nil, fmt.Errorf(`expected "actualCam" attribute for katcam %q`, path)
	}

	return []string{cfg.ActualCam}, nil
}

type katCam struct {
	resource.Named
	actualCam camera.Camera
	logger    golog.Logger

	// // Case 1: Only store one out of every 10 captures.
	// count *int32

	// Case 2:
	visionService vision.Service
}

// resource.Resource methods
func (c *katCam) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	c.actualCam = nil
	camConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	c.actualCam, err = camera.FromDependencies(deps, camConfig.ActualCam)
	if err != nil {
		return errors.Wrapf(err, "unable to get camera %v for katcam", camConfig.ActualCam)
	}

	// Case 2:
	c.visionService, err = vision.FromDependencies(deps, camConfig.VisionService)
	if err != nil {
		return errors.Wrapf(err, "unable to get vision service %v for katcam", camConfig.VisionService)
	}

	return nil
}

func (c *katCam) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

func (c *katCam) Close(ctx context.Context) error {
	return c.actualCam.Close(ctx)
}

// VideoStream methods
func (c *katCam) Images(ctx context.Context) ([]image.Image, time.Time, error) {
	return nil, time.Time{}, errUnimplemented
}

// Can implement Read instead of Stream - investigate which cams are MediaReader vs. MediaStream?
// func (c *katCam) Read(ctx context.Context) (image.Image, func(), error) {
// 	return nil, nil, nil
// }

func (c *katCam) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	camStream, err := c.actualCam.Stream(ctx, errHandlers...)
	if err != nil {
		return nil, err
	}
	// filterStream := filterStream{camStream}

	// // Case 1:
	// filterStream := filterStream{camStream, c.count}

	// Case 2:
	filterStream := filterStream{camStream, c.visionService}

	return filterStream, nil
}

func (c *katCam) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	return nil, errUnimplemented
}

func (c *katCam) Properties(ctx context.Context) (camera.Properties, error) {
	return camera.Properties{}, nil
}

// Projector methods
func (c *katCam) Projector(ctx context.Context) (transform.Projector, error) {
	return nil, errUnimplemented
}

// Filter code:
type filterStream struct {
	cameraStream gostream.VideoStream

	// // Case 1: Only store one out of every 10 captures.
	// count *int32

	// Case 2:
	visionService vision.Service
}

func (fs filterStream) Next(ctx context.Context) (image.Image, func(), error) {
	if ctx.Value(data.CtxKeyDM) != true {
		return nil, nil, errors.New("Cannot access filter stream if not DM collector")
	}
	// return fs.cameraStream.Next(ctx)

	// // Case 1: Only store one out of every 10 captures.
	// *fs.count++
	// if *fs.count%10 == 0 {
	// 	return fs.cameraStream.Next(ctx)
	// }
	// return nil, nil, data.ErrNoCaptureToStore

	// Case 2: Only store capture if it contains certain color.
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
