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
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(camera.Subtype, "single_stream",
		registry.Component{Constructor: func(ctx context.Context, _ registry.Dependencies,
			config config.Component, logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*ServerAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return NewServerSource(ctx, attrs, logger)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "single_stream",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf ServerAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*ServerAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&ServerAttrs{})

	registry.RegisterComponent(camera.Subtype, "dual_stream",
		registry.Component{Constructor: func(ctx context.Context, _ registry.Dependencies,
			config config.Component, logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*dualServerAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return newDualServerSource(ctx, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "dual_stream",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf dualServerAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*dualServerAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&dualServerAttrs{})
}

// dualServerSource stores two URLs, one which points the color source and the other to the
// depth source.
type dualServerSource struct {
	client                  http.Client
	ColorURL                string // this is for a generic image
	DepthURL                string // this suuports monochrome Z16 depth images
	Intrinsics              *transform.PinholeCameraIntrinsics
	Stream                  camera.StreamType // returns color or depth frame with calls of Next
	activeBackgroundWorkers sync.WaitGroup
}

// dualServerAttrs is the attribute struct for dualServerSource.
type dualServerAttrs struct {
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Stream               string                             `json:"stream"`
	Debug                bool                               `json:"debug,omitempty"`
	Color                string                             `json:"color_url"`
	Depth                string                             `json:"depth_url"`
}

// newDualServerSource creates the VideoSource that streams color/depth data from two external servers, one for each channel.
func newDualServerSource(ctx context.Context, cfg *dualServerAttrs) (camera.Camera, error) {
	if (cfg.Color == "") || (cfg.Depth == "") {
		return nil, errors.New("camera 'dual_stream' needs color and depth attributes")
	}
	videoSrc := &dualServerSource{
		ColorURL:   cfg.Color,
		DepthURL:   cfg.Depth,
		Intrinsics: cfg.CameraParameters,
		Stream:     camera.StreamType(cfg.Stream),
	}
	return camera.NewFromReader(
		ctx,
		videoSrc,
		&transform.PinholeCameraModel{cfg.CameraParameters, cfg.DistortionParameters},
		videoSrc.Stream,
	)
}

// Read requests either the color or depth frame, depending on what the config specifies.
func (ds *dualServerSource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "videosource::dualServerSource::Read")
	defer span.End()
	switch ds.Stream {
	case camera.ColorStream, camera.UnspecifiedStream:
		img, err := readColorURL(ctx, ds.client, ds.ColorURL)
		return img, func() {}, err
	case camera.DepthStream:
		depth, err := readDepthURL(ctx, ds.client, ds.DepthURL, false)
		return depth, func() {}, err
	default:
		return nil, nil, camera.NewUnsupportedStreamError(ds.Stream)
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
		colorImg, err := readColorURL(ctx, ds.client, ds.ColorURL)
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
		depthImg, err = readDepthURL(ctx, ds.client, ds.DepthURL, true)
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
	stream     camera.StreamType // specifies color, depth
	Intrinsics *transform.PinholeCameraIntrinsics
}

// ServerAttrs is the attribute struct for serverSource.
type ServerAttrs struct {
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
		img, err := readColorURL(ctx, s.client, s.URL)
		return img, func() {}, err
	case camera.DepthStream:
		depth, err := readDepthURL(ctx, s.client, s.URL, false)
		return depth, func() {}, err
	default:
		return nil, nil, camera.NewUnsupportedStreamError(s.stream)
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
		depth, err := readDepthURL(ctx, s.client, s.URL, true)
		if err != nil {
			return nil, err
		}
		return depthadapter.ToPointCloud(depth.(*rimage.DepthMap), s.Intrinsics), nil
	}
	return nil,
		errors.Errorf("no depth information in stream %q, cannot project to point cloud", s.stream)
}

// NewServerSource creates the VideoSource that streams color/depth data from an external server at a given URL.
func NewServerSource(ctx context.Context, cfg *ServerAttrs, logger golog.Logger) (camera.Camera, error) {
	if cfg.Stream == "" {
		return nil, errors.New("camera 'single_stream' needs attribute 'stream' (color, depth)")
	}
	if cfg.URL == "" {
		return nil, errors.New("camera 'single_stream' needs attribute 'url'")
	}
	videoSrc := &serverSource{
		URL:        cfg.URL,
		stream:     camera.StreamType(cfg.Stream),
		Intrinsics: cfg.CameraParameters,
	}
	return camera.NewFromReader(
		ctx,
		videoSrc,
		&transform.PinholeCameraModel{cfg.CameraParameters, cfg.DistortionParameters},
		videoSrc.stream,
	)
}
