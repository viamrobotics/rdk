// Package imagesource defines various image sources typically registered as cameras in the API.
//
// Some sources are specific to a type of camera while some are general purpose sources that
// act as a component in an image transformation pipeline.
package imagesource

import (
	"bufio"
	"bytes"
	"context"
	// for embedding camera parameters.
	_ "embed"
	"fmt"
	"image"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/edaniels/golog"
	// register ppm.
	_ "github.com/lmittmann/ppm"
	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
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
			imgSrc := &fileSource{attrs.Color, attrs.Depth, attrs.Aligned}
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
	ColorFN   string
	DepthFN   string
	isAligned bool // are color and depth image already aligned
}

// fileSourceAttrs is the attribute struct for fileSource.
type fileSourceAttrs struct {
	*camera.AttrConfig
	Color   string `json:"color"`
	Depth   string `json:"depth"`
	Aligned bool   `json:"aligned"`
}

// Next returns the image stored in the color and depth files as an ImageWithDepth, or just an Image
// if Depth is not available.
func (fs *fileSource) Next(ctx context.Context) (image.Image, func(), error) {
	if fs.DepthFN == "" { // no depth info
		img, err := rimage.NewImageFromFile(fs.ColorFN)
		return img, func() {}, err
	}
	img, err := rimage.NewImageWithDepth(fs.ColorFN, fs.DepthFN, fs.isAligned)
	return img, func() {}, err
}

func decodeColor(colorData []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewBuffer(colorData))
	return img, err
}

func decodeDepth(depthData []byte) (*rimage.DepthMap, error) {
	return rimage.ReadDepthMap(bufio.NewReader(bytes.NewReader(depthData)))
}

func prepReadFromURL(ctx context.Context, client http.Client, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func readyBytesFromURL(ctx context.Context, client http.Client, url string) ([]byte, error) {
	body, err := prepReadFromURL(ctx, client, url)
	if err != nil {
		return nil, err
	}

	defer func() {
		viamutils.UncheckedError(body.Close())
	}()
	return ioutil.ReadAll(body)
}

// dualServerSource stores two URLs, one which points the color source and the other to the
// depth source.
type dualServerSource struct {
	client    http.Client
	ColorURL  string // this is for a generic image
	DepthURL  string // this is for my bizarre custom data format for depth data
	isAligned bool   // are the color and depth image already aligned
}

// dualServerAttrs is the attribute struct for dualServerSource.
type dualServerAttrs struct {
	*camera.AttrConfig
	Color   string `json:"color"`
	Depth   string `json:"depth"`
	Aligned bool   `json:"aligned"`
}

// newDualServerSource creates the ImageSource that streams color/depth/both data from two external servers, one for each channel.
func newDualServerSource(cfg *dualServerAttrs) (camera.Camera, error) {
	if (cfg.Color == "") || (cfg.Depth == "") {
		return nil, errors.New("camera 'dual_stream' needs color and depth attributes")
	}
	imgSrc := &dualServerSource{
		ColorURL:  cfg.Color,
		DepthURL:  cfg.Depth,
		isAligned: cfg.Aligned,
	}
	return camera.New(imgSrc, cfg.AttrConfig, nil)
}

// Next requests the next images from both the color and depth source, and combines them
// together as an ImageWithDepth before returning them.
func (ds *dualServerSource) Next(ctx context.Context) (image.Image, func(), error) {
	colorData, err := readyBytesFromURL(ctx, ds.client, ds.ColorURL)
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't ready color url")
	}
	img, err := decodeColor(colorData)
	if err != nil {
		return nil, nil, err
	}

	depthData, err := readyBytesFromURL(ctx, ds.client, ds.DepthURL)
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't ready depth url")
	}
	// do this first and make sure ok before creating any mats
	depth, err := decodeDepth(depthData)
	if err != nil {
		return nil, nil, err
	}

	return rimage.MakeImageWithDepth(rimage.ConvertImage(img), depth, ds.isAligned), func() {}, nil
}

// Close closes the connection to both servers.
func (ds *dualServerSource) Close() {
	ds.client.CloseIdleConnections()
}

// serverSource streams the color/depth/both camera data from an external server at a given URL.
type serverSource struct {
	client    http.Client
	URL       string
	host      string
	stream    camera.StreamType // specifies color, depth, or both stream
	isAligned bool              // are the color and depth image already aligned
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
func (s *serverSource) Next(ctx context.Context) (image.Image, func(), error) {
	var img *rimage.ImageWithDepth
	var err error

	in, err := prepReadFromURL(ctx, s.client, s.URL)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "couldn't read url (%s)", s.URL)
	}
	defer func() {
		viamutils.UncheckedError(in.Close())
	}()

	switch s.stream {
	case camera.ColorStream:
		color, _, err := image.Decode(in)
		if err != nil {
			return nil, nil, err
		}
		img = rimage.MakeImageWithDepth(rimage.ConvertImage(color), nil, false)
	case camera.DepthStream:
		depth, err := rimage.ReadDepthMap(bufio.NewReader(in))
		if err != nil {
			return nil, nil, err
		}
		return depth, func() {}, nil
	case camera.BothStream:
		img, err = rimage.ReadBothFromReader(bufio.NewReader(in), s.isAligned)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, errors.Errorf("do not know how to decode stream type %q", string(s.stream))
	}

	return img, func() {}, nil
}

// NewServerSource creates the ImageSource that streams color/depth/both data from an external server at a given URL.
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
