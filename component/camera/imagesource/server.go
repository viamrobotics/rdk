// Package imagesource defines various image sources typically registered as cameras in the API.
package imagesource

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

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
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
			cameraAttrs, err := camera.CommonCameraAttributes(attributes)
			if err != nil {
				return nil, err
			}
			var conf ServerAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*ServerAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			result.AttrConfig = cameraAttrs
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
			cameraAttrs, err := camera.CommonCameraAttributes(attributes)
			if err != nil {
				return nil, err
			}
			var conf dualServerAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*dualServerAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			result.AttrConfig = cameraAttrs
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
	*camera.AttrConfig
	Color string `json:"color"`
	Depth string `json:"depth"`
}

// newDualServerSource creates the ImageSource that streams color/depth/both data from two external servers, one for each channel.
func newDualServerSource(ctx context.Context, cfg *dualServerAttrs) (camera.Camera, error) {
	if (cfg.Color == "") || (cfg.Depth == "") {
		return nil, errors.New("camera 'dual_stream' needs color and depth attributes")
	}
	imgSrc := &dualServerSource{
		ColorURL:   cfg.Color,
		DepthURL:   cfg.Depth,
		Intrinsics: cfg.CameraParameters,
		Stream:     camera.StreamType(cfg.Stream),
	}
	proj, _ := camera.GetProjector(ctx, cfg.AttrConfig, nil)
	return camera.New(imgSrc, proj)
}

// Next requests either the color or depth frame, depending on what the config specifies.
func (ds *dualServerSource) Next(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "imagesource::dualServerSource::Next")
	defer span.End()
	switch ds.Stream {
	case camera.ColorStream, camera.BothStream, camera.UnspecifiedStream:
		img, err := readColorURL(ctx, ds.client, ds.ColorURL)
		return img, func() {}, err
	case camera.DepthStream:
		depth, err := readDepthURL(ctx, ds.client, ds.DepthURL)
		return depth, func() {}, err
	default:
		return nil, nil, camera.NewUnsupportedStreamError(ds.Stream)
	}
}

func (ds *dualServerSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "imagesource::dualServerSource::NextPointCloud")
	defer span.End()
	if ds.Intrinsics == nil {
		return nil, transform.NewNoIntrinsicsError("dualServerSource has nil intrinsics")
	}
	var color *rimage.Image
	var depth *rimage.DepthMap
	// do a parallel request for the color and depth image
	// get color image
	ds.activeBackgroundWorkers.Add(1)
	viamutils.PanicCapturingGo(func() {
		defer ds.activeBackgroundWorkers.Done()
		var err error
		color, err = readColorURL(ctx, ds.client, ds.ColorURL)
		if err != nil {
			panic(err)
		}
	})
	// get depth image
	ds.activeBackgroundWorkers.Add(1)
	viamutils.PanicCapturingGo(func() {
		defer ds.activeBackgroundWorkers.Done()
		var err error
		depth, err = readDepthURL(ctx, ds.client, ds.DepthURL)
		if err != nil {
			panic(err)
		}
	})
	ds.activeBackgroundWorkers.Wait()
	return ds.Intrinsics.RGBDToPointCloud(color, depth)
}

// Close closes the connection to both servers.
func (ds *dualServerSource) Close() {
	ds.client.CloseIdleConnections()
}

// serverSource streams the color/depth/both camera data from an external server at a given URL.
type serverSource struct {
	client     http.Client
	URL        string
	stream     camera.StreamType // specifies color, depth
	Intrinsics *transform.PinholeCameraIntrinsics
}

// ServerAttrs is the attribute struct for serverSource.
type ServerAttrs struct {
	*camera.AttrConfig
	URL string `json:"url"`
}

// Close closes the server connection.
func (s *serverSource) Close() {
	s.client.CloseIdleConnections()
}

// Next returns the next image in the queue from the server.
// BothStream is deprecated and will be removed.
func (s *serverSource) Next(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "imagesource::serverSource::Next")
	defer span.End()
	switch s.stream {
	case camera.ColorStream, camera.BothStream, camera.UnspecifiedStream:
		img, err := readColorURL(ctx, s.client, s.URL)
		return img, func() {}, err
	case camera.DepthStream:
		depth, err := readDepthURL(ctx, s.client, s.URL)
		return depth, func() {}, err
	default:
		return nil, nil, camera.NewUnsupportedStreamError(s.stream)
	}
}

// serverSource can only produce a PointCloud from a DepthMap.
// BothStream is deprecated and will be removed.
func (s *serverSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "imagesource::serverSource::NextPointCloud")
	defer span.End()
	if s.Intrinsics == nil {
		return nil, transform.NewNoIntrinsicsError("single serverSource has nil intrinsics")
	}
	if s.stream == camera.DepthStream || s.stream == camera.BothStream {
		depth, err := readDepthURL(ctx, s.client, s.URL)
		if err != nil {
			return nil, err
		}
		return depth.ToPointCloud(s.Intrinsics), nil
	}
	return nil,
		errors.Errorf("no depth information in stream %q, cannot project to point cloud", s.stream)
}

// NewServerSource creates the ImageSource that streams color/depth data from an external server at a given URL.
func NewServerSource(ctx context.Context, cfg *ServerAttrs, logger golog.Logger) (camera.Camera, error) {
	if cfg.Stream == "" {
		return nil, errors.New("camera 'single_stream' needs attribute 'stream' (color, depth, or both)")
	}
	if cfg.URL == "" {
		return nil, errors.New("camera 'single_stream' needs attribute 'url'")
	}
	imgSrc := &serverSource{
		URL:        cfg.URL,
		stream:     camera.StreamType(cfg.Stream),
		Intrinsics: cfg.AttrConfig.CameraParameters,
	}
	proj, _ := camera.GetProjector(ctx, cfg.AttrConfig, nil)
	return camera.New(imgSrc, proj)
}
