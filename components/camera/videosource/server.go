//go:build !no_cgo

// Package videosource defines various image sources typically registered as cameras in the API.
package videosource

import (
	"context"
	// for embedding camera parameters.
	_ "embed"
	"image"
	"net/http"
	"sync"

	"github.com/edaniels/golog"
	// register ppm.
	_ "github.com/lmittmann/ppm"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
)

var (
	modelSingle = resource.DefaultModelFamily.WithModel("single_stream")
	modelDual   = resource.DefaultModelFamily.WithModel("dual_stream")
)

func init() {
	resource.RegisterComponent(camera.API, modelSingle,
		resource.Registration[camera.Camera, *ServerConfig]{
			Constructor: func(ctx context.Context, _ resource.Dependencies,
				conf resource.Config, logger golog.Logger,
			) (camera.Camera, error) {
				newConf, err := resource.NativeConfig[*ServerConfig](conf)
				if err != nil {
					return nil, err
				}
				src, err := NewServerSource(ctx, newConf, logger)
				if err != nil {
					return nil, err
				}
				return camera.FromVideoSource(conf.ResourceName(), src, logger), nil
			},
		})
	resource.RegisterComponent(camera.API, modelDual,
		resource.Registration[camera.Camera, *dualServerConfig]{
			Constructor: func(ctx context.Context, _ resource.Dependencies,
				conf resource.Config, logger golog.Logger,
			) (camera.Camera, error) {
				newConf, err := resource.NativeConfig[*dualServerConfig](conf)
				if err != nil {
					return nil, err
				}
				src, err := newDualServerSource(ctx, newConf, logger)
				if err != nil {
					return nil, err
				}
				return camera.FromVideoSource(conf.ResourceName(), src, logger), nil
			},
		})
}

// dualServerSource stores two URLs, one which points the color source and the other to the
// depth source.
type dualServerSource struct {
	client                  http.Client
	ColorURL                string // this is for a generic image
	DepthURL                string // this suuports monochrome Z16 depth images
	Intrinsics              *transform.PinholeCameraIntrinsics
	Stream                  camera.ImageType // returns color or depth frame with calls of Next
	activeBackgroundWorkers sync.WaitGroup
	logger                  golog.Logger
}

// dualServerConfig is the attribute struct for dualServerSource.
type dualServerConfig struct {
	resource.TriviallyValidateConfig
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Stream               string                             `json:"stream"`
	Debug                bool                               `json:"debug,omitempty"`
	Color                string                             `json:"color_url"`
	Depth                string                             `json:"depth_url"`
}

// newDualServerSource creates the VideoSource that streams color/depth data from two external servers, one for each channel.
func newDualServerSource(
	ctx context.Context,
	cfg *dualServerConfig,
	logger golog.Logger,
) (camera.VideoSource, error) {
	if (cfg.Color == "") || (cfg.Depth == "") {
		return nil, errors.New("camera 'dual_stream' needs color and depth attributes")
	}
	videoSrc := &dualServerSource{
		ColorURL:   cfg.Color,
		DepthURL:   cfg.Depth,
		Intrinsics: cfg.CameraParameters,
		Stream:     camera.ImageType(cfg.Stream),
		logger:     logger,
	}
	cameraModel := camera.NewPinholeModelWithBrownConradyDistortion(cfg.CameraParameters, cfg.DistortionParameters)
	return camera.NewVideoSourceFromReader(
		ctx,
		videoSrc,
		&cameraModel,
		videoSrc.Stream,
	)
}

// Read requests either the color or depth frame, depending on what the config specifies.
func (ds *dualServerSource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "videosource::dualServerSource::Read")
	defer span.End()
	switch ds.Stream {
	case camera.ColorStream, camera.UnspecifiedStream:
		img, err := readColorURL(ctx, ds.client, ds.ColorURL, ds.logger)
		return img, func() {}, err
	case camera.DepthStream:
		depth, err := readDepthURL(ctx, ds.client, ds.DepthURL, false, ds.logger)
		return depth, func() {}, err
	default:
		return nil, nil, camera.NewUnsupportedImageTypeError(ds.Stream)
	}
}

func (ds *dualServerSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "videosource::dualServerSource::NextPointCloud")
	defer span.End()
	if ds.Intrinsics == nil {
		return nil, transform.NewNoIntrinsicsError("dualServerSource has nil intrinsic_parameters")
	}
	var color *rimage.Image
	var depth *rimage.DepthMap
	// do a parallel request for the color and depth image
	// get color image
	ds.activeBackgroundWorkers.Add(1)
	viamutils.PanicCapturingGo(func() {
		defer ds.activeBackgroundWorkers.Done()
		var err error
		colorImg, err := readColorURL(ctx, ds.client, ds.ColorURL, ds.logger)
		if err != nil {
			panic(err)
		}
		color = colorImg.(*rimage.Image)
	})
	// get depth image
	ds.activeBackgroundWorkers.Add(1)
	viamutils.PanicCapturingGo(func() {
		defer ds.activeBackgroundWorkers.Done()
		var err error
		var depthImg image.Image
		depthImg, err = readDepthURL(ctx, ds.client, ds.DepthURL, true, ds.logger)
		depth = depthImg.(*rimage.DepthMap)
		if err != nil {
			panic(err)
		}
	})
	ds.activeBackgroundWorkers.Wait()
	return ds.Intrinsics.RGBDToPointCloud(color, depth)
}

// Close closes the connection to both servers.
func (ds *dualServerSource) Close(ctx context.Context) error {
	ds.client.CloseIdleConnections()
	return nil
}

// serverSource streams the color/depth camera data from an external server at a given URL.
type serverSource struct {
	client     http.Client
	URL        string
	stream     camera.ImageType // specifies color, depth
	Intrinsics *transform.PinholeCameraIntrinsics
	logger     golog.Logger
}

// ServerConfig is the attribute struct for serverSource.
type ServerConfig struct {
	resource.TriviallyValidateConfig
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Stream               string                             `json:"stream"`
	Debug                bool                               `json:"debug,omitempty"`
	URL                  string                             `json:"url"`
}

// Close closes the server connection.
func (s *serverSource) Close(ctx context.Context) error {
	s.client.CloseIdleConnections()
	return nil
}

// Read returns the next image in the queue from the server.
func (s *serverSource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "videosource::serverSource::Read")
	defer span.End()
	switch s.stream {
	case camera.ColorStream, camera.UnspecifiedStream:
		img, err := readColorURL(ctx, s.client, s.URL, s.logger)
		return img, func() {}, err
	case camera.DepthStream:
		depth, err := readDepthURL(ctx, s.client, s.URL, false, s.logger)
		return depth, func() {}, err
	default:
		return nil, nil, camera.NewUnsupportedImageTypeError(s.stream)
	}
}

// serverSource can only produce a PointCloud from a DepthMap.
func (s *serverSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "videosource::serverSource::NextPointCloud")
	defer span.End()
	if s.Intrinsics == nil {
		return nil, transform.NewNoIntrinsicsError("single serverSource has nil intrinsic_parameters")
	}
	if s.stream == camera.DepthStream {
		depth, err := readDepthURL(ctx, s.client, s.URL, true, s.logger)
		if err != nil {
			return nil, err
		}
		return depthadapter.ToPointCloud(depth.(*rimage.DepthMap), s.Intrinsics), nil
	}
	return nil,
		errors.Errorf("no depth information in stream %q, cannot project to point cloud", s.stream)
}

// NewServerSource creates the VideoSource that streams color/depth data from an external server at a given URL.
func NewServerSource(ctx context.Context, cfg *ServerConfig, logger golog.Logger) (camera.VideoSource, error) {
	if cfg.Stream == "" {
		return nil, errors.New("camera 'single_stream' needs attribute 'stream' (color, depth)")
	}
	if cfg.URL == "" {
		return nil, errors.New("camera 'single_stream' needs attribute 'url'")
	}
	videoSrc := &serverSource{
		URL:        cfg.URL,
		stream:     camera.ImageType(cfg.Stream),
		Intrinsics: cfg.CameraParameters,
		logger:     logger,
	}
	cameraModel := camera.NewPinholeModelWithBrownConradyDistortion(cfg.CameraParameters, cfg.DistortionParameters)
	return camera.NewVideoSourceFromReader(
		ctx,
		videoSrc,
		&cameraModel,
		videoSrc.stream,
	)
}
