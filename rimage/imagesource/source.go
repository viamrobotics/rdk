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
	"errors"
	"fmt"
	"image"
	"io/ioutil"
	"net/http"

	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/transform"
	"go.viam.com/core/robot"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	_ "github.com/lmittmann/ppm" // register ppm
)

//go:embed intel515_parameters.json
var intel515json []byte

func init() {
	registry.RegisterCamera("intel", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gostream.ImageSource, error) {
		return NewIntelServerSource(config.Host, config.Port, config.Attributes)
	})
	registry.RegisterCamera("eliot", registry.CameraLookup("intel"))

	registry.RegisterCamera("url", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gostream.ImageSource, error) {
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
		return &HTTPSource{
			ColorURL:  config.Attributes.String("color"),
			DepthURL:  config.Attributes.String("depth"),
			isAligned: aligned,
		}, nil
	})

	registry.RegisterCamera("file", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gostream.ImageSource, error) {
		x, has := config.Attributes["aligned"]
		if !has {
			return nil, errors.New("config for file needs bool attribute 'aligned'")
		}
		aligned, ok := x.(bool)
		if !ok {
			return nil, errors.New("attribute 'aligned' must be a bool")
		}
		return &FileSource{config.Attributes.String("color"), config.Attributes.String("depth"), aligned}, nil
	})
}

// StaticSource TODO
type StaticSource struct {
	Img *rimage.ImageWithDepth
}

// Next TODO
func (ss *StaticSource) Next(ctx context.Context) (image.Image, func(), error) {
	return ss.Img, func() {}, nil
}

// Close TODO
func (ss *StaticSource) Close() error {
	return nil
}

// FileSource TODO
type FileSource struct {
	ColorFN   string
	DepthFN   string
	isAligned bool // are color and depth image already aligned
}

// IsAligned TODO
func (fs *FileSource) IsAligned() bool {
	return fs.isAligned
}

// Next TODO
func (fs *FileSource) Next(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewImageWithDepth(fs.ColorFN, fs.DepthFN, fs.IsAligned())
	return img, func() {}, err
}

// Close TODO
func (fs *FileSource) Close() error {
	return nil
}

// HTTPSource TODO
type HTTPSource struct {
	client    http.Client
	ColorURL  string // this is for a generic image
	DepthURL  string // this is for my bizarre custom data format for depth data
	isAligned bool   // are the color and depth image already aligned
}

// IsAligned TODO
func (hs *HTTPSource) IsAligned() bool {
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
func (hs *HTTPSource) Next(ctx context.Context) (image.Image, func(), error) {
	colorData, err := readyBytesFromURL(hs.client, hs.ColorURL)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't ready color url: %w", err)
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
		return nil, nil, fmt.Errorf("couldn't ready depth url: %w", err)
	}

	// do this first and make sure ok before creating any mats
	depth, err := rimage.ReadDepthMap(bufio.NewReader(bytes.NewReader(depthData)))
	if err != nil {
		return nil, nil, err
	}
	return rimage.MakeImageWithDepth(rimage.ConvertImage(img), depth, hs.IsAligned(), nil), func() {}, nil
}

// Close TODO
func (hs *HTTPSource) Close() error {
	hs.client.CloseIdleConnections()
	return nil
}

// IntelServerSource TODO
type IntelServerSource struct {
	client    http.Client
	BothURL   string
	host      string
	isAligned bool // are the color and depth image already aligned
	camera    rimage.CameraSystem
}

// IsAligned TODO
func (s *IntelServerSource) IsAligned() bool {
	return s.isAligned
}

// CameraSystem TODO
func (s *IntelServerSource) CameraSystem() rimage.CameraSystem {
	return s.camera
}

// NewIntelServerSource TODO
func NewIntelServerSource(host string, port int, attrs config.AttributeMap) (*IntelServerSource, error) {
	num := "0"
	numString, has := attrs["num"]
	if has {
		num = numString.(string)
	}
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromBytes(intel515json)
	if err != nil {
		return nil, err
	}
	return &IntelServerSource{
		BothURL:   fmt.Sprintf("http://%s:%d/both?num=%s", host, port, num),
		host:      host,
		isAligned: attrs.Bool("aligned", true),
		camera:    camera,
	}, nil
}

// Close TODO
func (s *IntelServerSource) Close() error {
	return nil
}

// Next TODO
func (s *IntelServerSource) Next(ctx context.Context) (image.Image, func(), error) {
	allData, err := readyBytesFromURL(s.client, s.BothURL)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't read url (%s): %w", s.BothURL, err)
	}

	img, err := rimage.ReadBothFromBytes(allData, s.IsAligned())
	if err != nil {
		return nil, nil, err
	}
	img.SetCameraSystem(s.CameraSystem())
	return img, func() {}, err
}
