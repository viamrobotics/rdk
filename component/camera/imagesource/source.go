// Package imagesource defines various image sources typically registered as cameras in the API.
//
// Some sources are specific to a type of camera while some are general purpose sources that
// act as a component in an image transformation pipeline.
package imagesource

import (
	"context"
	"sync"

	// for embedding camera parameters.
	_ "embed"
	"fmt"
	"image"
	"net/http"

	"github.com/edaniels/golog"
	"go.opencensus.io/trace"

	// register ppm.
	_ "github.com/lmittmann/ppm"
	"github.com/pkg/errors"

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
			return NewServerSource(attrs, logger)
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
			return newDualServerSource(attrs)
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

	registry.RegisterComponent(camera.Subtype, "file",
		registry.Component{Constructor: func(ctx context.Context, _ registry.Dependencies,
			config config.Component, logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*fileSourceAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			imgSrc := &fileSource{attrs.Color, attrs.Depth, attrs.CameraParameters}
			return camera.New(imgSrc, attrs.AttrConfig, nil)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "file",
		func(attributes config.AttributeMap) (interface{}, error) {
			cameraAttrs, err := camera.CommonCameraAttributes(attributes)
			if err != nil {
				return nil, err
			}
			var conf fileSourceAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*fileSourceAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			result.AttrConfig = cameraAttrs
			return result, nil
		},
		&fileSourceAttrs{})
}

// StaticSource is a fixed, stored image.
type StaticSource struct {
	Img image.Image
}

// Next returns the stored image.
func (ss *StaticSource) Next(ctx context.Context) (image.Image, func(), error) {
	return ss.Img, func() {}, nil
}

// fileSource stores the paths to a color and depth image.
type fileSource struct {
	ColorFN    string
	DepthFN    string
	Intrinsics *transform.PinholeCameraIntrinsics
}

// fileSourceAttrs is the attribute struct for fileSource.
type fileSourceAttrs struct {
	*camera.AttrConfig
	Color string `json:"color"`
	Depth string `json:"depth"`
}

// Next returns just the RGB image if it is present, or the depth map if the RGB image is not present.
func (fs *fileSource) Next(ctx context.Context) (image.Image, func(), error) {
	if fs.ColorFN == "" { // only depth info
		img, err := rimage.NewDepthMapFromFile(fs.DepthFN)
		return img, func() {}, err
	}
	img, err := rimage.NewImageFromFile(fs.ColorFN)
	return img, func() {}, err
}

// Next PointCloud returns the point cloud from projecting the rgb and depth image using the intrinsic parameters.
func (fs *fileSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if fs.Intrinsics == nil {
		return nil, transform.NewNoIntrinsicsError("camera intrinsics not found in config")
	}
	img, err := rimage.NewImageFromFile(fs.ColorFN)
	if err != nil {
		return nil, err
	}
	dm, err := rimage.NewDepthMapFromFile(fs.DepthFN)
	if err != nil {
		return nil, err
	}
	return fs.Intrinsics.RGBDToPointCloud(img, dm)
}

// dualServerSource stores two URLs, one which points the color source and the other to the
// depth source.
type dualServerSource struct {
	client     http.Client
	ColorURL   string // this is for a generic image
	DepthURL   string // this is for my bizarre custom data format for depth data
	Intrinsics *transform.PinholeCameraIntrinsics
	Stream     StreamType // returns color or depth frame with calls of Next
}

// dualServerAttrs is the attribute struct for dualServerSource.
type dualServerAttrs struct {
	*camera.AttrConfig
	Color string `json:"color"`
	Depth string `json:"depth"`
}

// newDualServerSource creates the ImageSource that streams color/depth/both data from two external servers, one for each channel.
func newDualServerSource(cfg *dualServerAttrs) (camera.Camera, error) {
	if (cfg.Color == "") || (cfg.Depth == "") {
		return nil, errors.New("camera 'dual_stream' needs color and depth attributes")
	}
	imgSrc := &dualServerSource{
		ColorURL:   cfg.Color,
		DepthURL:   cfg.Depth,
		Intrinsics: cfg.CameraParameters,
		Stream:     StreamType(cfg.Stream),
	}
	return camera.New(imgSrc, cfg.AttrConfig, nil)
}

// Next requests either the color or depth frame, depending on what the config specifies.
func (ds *dualServerSource) Next(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "imagesource::dualServerSource::Next")
	defer span.End()
	switch ds.Stream {
	case ColorStream:
		img, err := readColorURL(ctx, ds.client, ds.ColorURL)
		return img, func() {}, err
	case DepthStream:
		depth, err := readDepthURL(ctx, ds.client, ds.DepthURL)
		return depth, func() {}, err
	default:
		return nil, nil, errors.Errorf("stream of type %q not supported", ds.Stream)
	}
}

func (ds *dualServerSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "imagesource::dualServerSource::NextPointCloud")
	defer span.End()
	if ds.Intrinsics == nil {
		return nil, transform.NewNoIntrinsicsError("camera intrinsics not found in config")
	}
	var color *rimage.Image
	var depth *rimage.DepthMap
	var err error
	// do a parallel request for the color and depth image
	wg := sync.WaitGroup{}
	// get color image
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		color, err = readColorURL(ctx, ds.client, ds.ColorURL)
		if err != nil {
			return nil, err
		}
	}(ctx)
	// get depth image
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		depth, err = readDepthURL(ctx, ds.client, ds.DepthURL)
		if err != nil {
			return nil, err
		}
	}(ctx)
	wg.Wait()
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
	host       string
	stream     camera.StreamType // specifies color, depth, or both stream
	Intrinsics *transform.PinholeCameraIntrinsics
}

// ServerAttrs is the attribute struct for serverSource.
type ServerAttrs struct {
	*camera.AttrConfig
	Aligned bool   `json:"aligned"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Args    string `json:"args"`
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
	case ColorStream, BothStream:
		img, err := readColorURL(ctx, s.client, s.URL)
		return img, func() {}, err
	case DepthStream:
		depth, err := readDepthURL(ctx, s.client, s.URL)
		return depth, func() {}, err
	default:
		return nil, nil, errors.Errorf("stream of type %q not supported", s.stream)
	}
}

// serverSource can only produce a PointCloud from a DepthMap.
// BothStream is deprecated and will be removed.
func (s *serverSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "imagesource::serverSource::NextPointCloud")
	defer span.End()
	if s.Intrinsics == nil {
		return nil, transform.NewNoIntrinsicsError("camera intrinsics not found in config")
	}
	if s.stream == DepthStream || s.stream == BothStream {
		depth, err := readDepthURL(ctx, s.client, s.URL)
		if err != nil {
			return nil, err
		}
		return depth.ToPointCloud(s.Intrinsics)
	}
	return nil,
		errors.Errorf("no depth information in stream %q, cannot project to point cloud", s.stream)
}

// NewServerSource creates the ImageSource that streams color/depth data from an external server at a given URL.
func NewServerSource(cfg *ServerAttrs, logger golog.Logger) (camera.Camera, error) {
	if cfg.Stream == "" {
		return nil, errors.New("camera 'single_stream' needs attribute 'stream' (color, depth, or both)")
	}
	if cfg.Host == "" {
		return nil, errors.New("camera 'single_stream' needs attribute 'host'")
	}
	imgSrc := &serverSource{
		URL:       fmt.Sprintf("http://%s:%d/%s", cfg.Host, cfg.Port, cfg.Args),
		host:      cfg.Host,
		stream:    camera.StreamType(cfg.Stream),
		isAligned: cfg.Aligned,
	}
	return camera.New(imgSrc, cfg.AttrConfig, nil)
}
