// Package imagesource defines various image sources typically registered as cameras in the API.
//
// Some sources are specific to a type of camera while some are general purpose sources that
// act as a component in an image transformation pipeline.
package imagesource

import (
	"bufio"
	"bytes"
	"context"
	_ "embed" // for embedding camera parameters
	"fmt"
	"image"
	"io/ioutil"
	"net/http"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/component/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/transform"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	_ "github.com/lmittmann/ppm" // register ppm
)

//go:embed intel515_parameters.json
var intel515json []byte

func init() {
	registry.RegisterComponent(camera.Subtype, "single_stream", registry.Component{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
		if len(config.Attributes) == 0 {
			return nil, errors.New("camera 'single_stream' needs attribute 'stream' (color, depth, or both)")
		}
		_, has := config.Attributes["stream"]
		if !has {
			return nil, errors.New("camera 'single_stream' needs attribute 'stream' (color, depth, or both)")
		}
		source, err := NewServerSource(config.Host, config.Port, config.Attributes, logger)
		if err != nil {
			return nil, err
		}
		return &camera.ImageSource{ImageSource: source}, nil
	}})
	registry.RegisterComponent(camera.Subtype, "dual_stream", registry.Component{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
		if len(config.Attributes) == 0 {
			return nil, errors.New("camera 'dual_stream' needs a color and depth attribute")
		}
		x, has := config.Attributes["aligned"]
		if !has {
			return nil, errors.New("camera 'dual_stream' needs bool attribute 'aligned'")
		}
		aligned, ok := x.(bool)
		if !ok {
			return nil, errors.New("attribute 'aligned' must be a bool")
		}
		return &camera.ImageSource{ImageSource: &dualServerSource{
			ColorURL:  config.Attributes.String("color"),
			DepthURL:  config.Attributes.String("depth"),
			isAligned: aligned,
		}}, nil
	}})

	registry.RegisterComponent(camera.Subtype, "intel", registry.Component{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
		source, err := NewIntelServerSource(config.Host, config.Port, config.Attributes, logger)
		if err != nil {
			return nil, err
		}
		return &camera.ImageSource{ImageSource: source}, nil
	}})
	registry.RegisterComponent(camera.Subtype, "eliot", *registry.ComponentLookup(camera.Subtype, "intel"))

	registry.RegisterComponent(camera.Subtype, "file", registry.Component{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
		x, has := config.Attributes["aligned"]
		if !has {
			return nil, errors.New("config for file needs bool attribute 'aligned'")
		}
		aligned, ok := x.(bool)
		if !ok {
			return nil, errors.New("attribute 'aligned' must be a bool")
		}
		return &camera.ImageSource{ImageSource: &fileSource{config.Attributes.String("color"), config.Attributes.String("depth"), aligned}}, nil
	}})
}

// staticSource is a fixed, stored image
type staticSource struct {
	Img image.Image
}

// Next returns the stored image
func (ss *staticSource) Next(ctx context.Context) (image.Image, func(), error) {
	return ss.Img, func() {}, nil
}

// Close does nothing
func (ss *staticSource) Close() error {
	return nil
}

// fileSource stores the paths to a color and depth image.
type fileSource struct {
	ColorFN   string
	DepthFN   string
	isAligned bool // are color and depth image already aligned
}

// IsAligned returns a bool that is true if the color and depth images are aligned.
func (fs *fileSource) IsAligned() bool {
	return fs.isAligned
}

// Next returns the image stored in the color and depth files as an ImageWithDepth.
func (fs *fileSource) Next(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewImageWithDepth(fs.ColorFN, fs.DepthFN, fs.IsAligned())
	return img, func() {}, err
}

// Close closes the source (does nothing)
func (fs *fileSource) Close() error {
	return nil
}

func decodeColor(colorData []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewBuffer(colorData))
	return img, err
}

func decodeDepth(depthData []byte) (*rimage.DepthMap, error) {
	return rimage.ReadDepthMap(bufio.NewReader(bytes.NewReader(depthData)))
}

func decodeBoth(bothData []byte, aligned bool) (*rimage.ImageWithDepth, error) {
	return rimage.ReadBothFromBytes(bothData, aligned)
}

func readyBytesFromURL(client http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	defer utils.UncheckedErrorFunc(resp.Body.Close)
	return ioutil.ReadAll(resp.Body)
}

// dualServerSource stores two URLs, one which points the color source and the other to the
// depth source.
type dualServerSource struct {
	client    http.Client
	ColorURL  string // this is for a generic image
	DepthURL  string // this is for my bizarre custom data format for depth data
	isAligned bool   // are the color and depth image already aligned
}

// IsAligned returns true if the images returned from the two servers are already aligned
// with each other.
func (ds *dualServerSource) IsAligned() bool {
	return ds.isAligned
}

// Next requests the next images from both the color and depth source, and combines them
// together as an ImageWithDepth before returning them.
func (ds *dualServerSource) Next(ctx context.Context) (image.Image, func(), error) {
	colorData, err := readyBytesFromURL(ds.client, ds.ColorURL)
	if err != nil {
		return nil, nil, errors.Errorf("couldn't ready color url: %w", err)
	}
	img, err := decodeColor(colorData)
	if err != nil {
		return nil, nil, err
	}

	depthData, err := readyBytesFromURL(ds.client, ds.DepthURL)
	if err != nil {
		return nil, nil, errors.Errorf("couldn't ready depth url: %w", err)
	}
	// do this first and make sure ok before creating any mats
	depth, err := decodeDepth(depthData)
	if err != nil {
		return nil, nil, err
	}

	return rimage.MakeImageWithDepth(rimage.ConvertImage(img), depth, ds.IsAligned(), nil), func() {}, nil
}

// Close closes the connection to both servers.
func (ds *dualServerSource) Close() error {
	ds.client.CloseIdleConnections()
	return nil
}

// StreamType specifies what kind of image stream is coming from the camera
type StreamType string

// The allowed types of streams that can come from an ImageSource
const (
	ColorStream = StreamType("color")
	DepthStream = StreamType("depth")
	BothStream  = StreamType("both")
)

// serverSource streams the color/depth/both camera data from an external server at a given URL.
type serverSource struct {
	client    http.Client
	URL       string
	host      string
	stream    StreamType // specifies color, depth, or both stream
	isAligned bool       // are the color and depth image already aligned
	camera    rimage.CameraSystem
}

// IsAligned is a bool that returns true if both.gz image is already aligned. If the server is only returning a single stream
// (either color or depth) IsAligned will return false.
func (s *serverSource) IsAligned() bool {
	return s.isAligned
}

// CameraSystem is the combined Aligner that aligns the depth and color images,
// and the Projector which projects between 3D and 2D representations
func (s *serverSource) CameraSystem() rimage.CameraSystem {
	return s.camera
}

// Close closes the server connection
func (s *serverSource) Close() error {
	s.client.CloseIdleConnections()
	return nil
}

// Next returns the next image in the queue from the server
func (s *serverSource) Next(ctx context.Context) (image.Image, func(), error) {
	var img *rimage.ImageWithDepth
	var err error

	allData, err := readyBytesFromURL(s.client, s.URL)
	if err != nil {
		return nil, nil, errors.Errorf("couldn't read url (%s): %w", s.URL, err)
	}

	switch s.stream {
	case ColorStream:
		color, err := decodeColor(allData)
		if err != nil {
			return nil, nil, err
		}
		img = rimage.MakeImageWithDepth(rimage.ConvertImage(color), nil, false, s.CameraSystem())
	case DepthStream:
		depth, err := decodeDepth(allData)
		if err != nil {
			return nil, nil, err
		}
		img = rimage.MakeImageWithDepth(rimage.ConvertImage(depth.ToGray16Picture()), depth, false, s.CameraSystem())
	case BothStream:
		img, err = decodeBoth(allData, s.isAligned)
		if err != nil {
			return nil, nil, err
		}
		img.SetCameraSystem(s.CameraSystem())
	default:
		return nil, nil, errors.Errorf("do not know how to decode stream type %q", string(s.stream))
	}

	return img, func() {}, nil
}

// NewServerSource creates the ImageSource that streams color/depth/both data from an external server at a given URL.
func NewServerSource(host string, port int, attrs config.AttributeMap, logger golog.Logger) (gostream.ImageSource, error) {
	camera, _, err := getCameraSystems(attrs, logger)
	if err != nil {
		return nil, err
	}
	stream := attrs.String("stream")

	return &serverSource{
		URL:       fmt.Sprintf("http://%s:%d/%s", host, port, attrs.String("args")),
		host:      host,
		stream:    StreamType(stream),
		isAligned: attrs.Bool("aligned", false),
		camera:    camera,
	}, nil
}

// NewIntelServerSource is the ImageSource for an Intel515 RGBD camera that streams both
// color and depth information.
// DEPRECATED: use NewServerSource directly instead with 'single_stream' model.
func NewIntelServerSource(host string, port int, attrs config.AttributeMap, logger golog.Logger) (gostream.ImageSource, error) {
	num := "0"
	numString, has := attrs["num"]
	if has {
		num = numString.(string)
	}
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromBytes(intel515json)
	if err != nil {
		return nil, err
	}
	conf := config.AttributeMap{}
	conf["intrinsic_extrinsic"] = camera
	conf["stream"] = "both"
	conf["args"] = fmt.Sprintf("both?num=%s", num)
	conf["aligned"] = attrs.Bool("aligned", true)

	return NewServerSource(host, port, conf, logger)
}
