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
	registry.RegisterComponent(camera.Subtype, "intel", registry.Component{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
		source, err := NewIntelServerSource(config.Host, config.Port, config.Attributes)
		if err != nil {
			return nil, err
		}
		return &camera.ImageSource{ImageSource: source}, nil
	}})
	registry.RegisterComponent(camera.Subtype, "eliot", *registry.ComponentLookup(camera.Subtype, "intel"))

	registry.RegisterComponent(camera.Subtype, "url", registry.Component{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
		if len(config.Attributes) == 0 {
			return nil, errors.New("camera 'url' needs a color attribute (and a depth if you have it)")
		}
		x, has := config.Attributes["aligned"]
		if !has {
			return nil, errors.New("camera 'url' needs bool attribute 'aligned'")
		}
		aligned, ok := x.(bool)
		if !ok {
			return nil, errors.New("attribute 'aligned' must be a bool")
		}
		return &camera.ImageSource{ImageSource: &httpSource{
			ColorURL:  config.Attributes.String("color"),
			DepthURL:  config.Attributes.String("depth"),
			isAligned: aligned,
		}}, nil
	}})

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

// staticSource TODO
type staticSource struct {
	Img image.Image
}

// Next TODO
func (ss *staticSource) Next(ctx context.Context) (image.Image, func(), error) {
	return ss.Img, func() {}, nil
}

// Close TODO
func (ss *staticSource) Close() error {
	return nil
}

// fileSource TODO
type fileSource struct {
	ColorFN   string
	DepthFN   string
	isAligned bool // are color and depth image already aligned
}

// IsAligned TODO
func (fs *fileSource) IsAligned() bool {
	return fs.isAligned
}

// Next TODO
func (fs *fileSource) Next(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewImageWithDepth(fs.ColorFN, fs.DepthFN, fs.IsAligned())
	return img, func() {}, err
}

// Close TODO
func (fs *fileSource) Close() error {
	return nil
}

// httpSource TODO
type httpSource struct {
	client    http.Client
	ColorURL  string // this is for a generic image
	DepthURL  string // this is for my bizarre custom data format for depth data
	isAligned bool   // are the color and depth image already aligned
}

// IsAligned TODO
func (hs *httpSource) IsAligned() bool {
	return hs.isAligned
}

func readyBytesFromURL(client http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	defer utils.UncheckedErrorFunc(resp.Body.Close)
	return ioutil.ReadAll(resp.Body)
}

// Next TODO
func (hs *httpSource) Next(ctx context.Context) (image.Image, func(), error) {
	colorData, err := readyBytesFromURL(hs.client, hs.ColorURL)
	if err != nil {
		return nil, nil, errors.Errorf("couldn't ready color url: %w", err)
	}

	img, _, err := image.Decode(bytes.NewBuffer(colorData))
	if err != nil {
		return nil, nil, err
	}

	if hs.DepthURL == "" {
		return img, func() {}, nil
	}

	depthData, err := readyBytesFromURL(hs.client, hs.DepthURL)
	if err != nil {
		return nil, nil, errors.Errorf("couldn't ready depth url: %w", err)
	}

	// do this first and make sure ok before creating any mats
	depth, err := rimage.ReadDepthMap(bufio.NewReader(bytes.NewReader(depthData)))
	if err != nil {
		return nil, nil, err
	}
	return rimage.MakeImageWithDepth(rimage.ConvertImage(img), depth, hs.IsAligned(), nil), func() {}, nil
}

// Close TODO
func (hs *httpSource) Close() error {
	hs.client.CloseIdleConnections()
	return nil
}

// intelServerSource TODO
type intelServerSource struct {
	client    http.Client
	BothURL   string
	host      string
	isAligned bool // are the color and depth image already aligned
	camera    rimage.CameraSystem
}

// IsAligned TODO
func (s *intelServerSource) IsAligned() bool {
	return s.isAligned
}

// CameraSystem TODO
func (s *intelServerSource) CameraSystem() rimage.CameraSystem {
	return s.camera
}

// NewIntelServerSource TODO
func NewIntelServerSource(host string, port int, attrs config.AttributeMap) (gostream.ImageSource, error) {
	num := "0"
	numString, has := attrs["num"]
	if has {
		num = numString.(string)
	}
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromBytes(intel515json)
	if err != nil {
		return nil, err
	}
	return &intelServerSource{
		BothURL:   fmt.Sprintf("http://%s:%d/both?num=%s", host, port, num),
		host:      host,
		isAligned: attrs.Bool("aligned", true),
		camera:    camera,
	}, nil
}

// Close TODO
func (s *intelServerSource) Close() error {
	s.client.CloseIdleConnections()
	return nil
}

// Next TODO
func (s *intelServerSource) Next(ctx context.Context) (image.Image, func(), error) {
	allData, err := readyBytesFromURL(s.client, s.BothURL)
	if err != nil {
		return nil, nil, errors.Errorf("couldn't read url (%s): %w", s.BothURL, err)
	}

	img, err := rimage.ReadBothFromBytes(allData, s.IsAligned())
	if err != nil {
		return nil, nil, err
	}
	img.SetCameraSystem(s.CameraSystem())
	return img, func() {}, err
}
