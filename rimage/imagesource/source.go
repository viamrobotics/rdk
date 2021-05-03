package imagesource

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"io/ioutil"
	"net/http"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	_ "github.com/lmittmann/ppm" // register ppm
)

func init() {
	api.RegisterCamera("intel", func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (gostream.ImageSource, error) {
		return NewIntelServerSource(config.Host, config.Port, config.Attributes), nil
	})
	api.RegisterCamera("eliot", api.CameraLookup("intel"))

	api.RegisterCamera("url", func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (gostream.ImageSource, error) {
		if len(config.Attributes) == 0 {
			return nil, fmt.Errorf("camera 'url' needs a color attribute (and a depth if you have it)")
		}
		x, has := config.Attributes["aligned"]
		if !has {
			return nil, fmt.Errorf("camera 'url' needs bool attribute 'aligned'")
		}
		aligned, ok := x.(bool)
		if !ok {
			return nil, fmt.Errorf("attribute 'aligned' must be a bool")
		}
		return &HTTPSource{
			ColorURL:  config.Attributes.GetString("color"),
			DepthURL:  config.Attributes.GetString("depth"),
			isAligned: aligned,
		}, nil
	})

	api.RegisterCamera("file", func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (gostream.ImageSource, error) {
		x, has := config.Attributes["aligned"]
		if !has {
			return nil, fmt.Errorf("config for file needs bool attribute 'aligned'")
		}
		aligned, ok := x.(bool)
		if !ok {
			return nil, fmt.Errorf("attribute 'aligned' must be a bool")
		}
		return &FileSource{config.Attributes.GetString("color"), config.Attributes.GetString("depth"), aligned}, nil
	})
}

type StaticSource struct {
	Img *rimage.ImageWithDepth
}

func (ss *StaticSource) Next(ctx context.Context) (image.Image, func(), error) {
	return ss.Img, func() {}, nil
}

func (ss *StaticSource) Close() error {
	return nil
}

// -----

type FileSource struct {
	ColorFN   string
	DepthFN   string
	isAligned bool // are color and depth image already aligned
}

func (fs *FileSource) IsAligned() bool {
	return fs.isAligned
}

func (fs *FileSource) Next(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewImageWithDepth(fs.ColorFN, fs.DepthFN, fs.IsAligned())
	return img, func() {}, err
}

func (fs *FileSource) Close() error {
	return nil
}

// -------

type HTTPSource struct {
	client    http.Client
	ColorURL  string // this is for a generic image
	DepthURL  string // this is for my bizarre custom data format for depth data
	isAligned bool   // are the color and depth image already aligned
}

func (hs *HTTPSource) IsAligned() bool {
	return hs.isAligned
}

func readyBytesFromURL(client http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func (hs *HTTPSource) Next(ctx context.Context) (image.Image, func(), error) {
	colorData, err := readyBytesFromURL(hs.client, hs.ColorURL)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't ready color url: %s", err)
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
		return nil, nil, fmt.Errorf("couldn't ready depth url: %s", err)
	}

	// do this first and make sure ok before creating any mats
	depth, err := rimage.ReadDepthMap(bufio.NewReader(bytes.NewReader(depthData)))
	if err != nil {
		return nil, nil, err
	}
	return rimage.MakeImageWithDepth(rimage.ConvertImage(img), depth, hs.IsAligned(), nil), func() {}, nil
}

func (hs *HTTPSource) Close() error {
	hs.client.CloseIdleConnections()
	return nil
}

// ------

type IntelServerSource struct {
	client    http.Client
	BothURL   string
	host      string
	isAligned bool // are the color and depth image already aligned
}

func (s *IntelServerSource) IsAligned() bool {
	return s.isAligned
}

func NewIntelServerSource(host string, port int, attrs api.AttributeMap) *IntelServerSource {
	num := "0"
	numString, has := attrs["num"]
	if has {
		num = numString.(string)
	}
	return &IntelServerSource{
		BothURL:   fmt.Sprintf("http://%s:%d/both?num=%s", host, port, num),
		host:      host,
		isAligned: attrs.GetBool("aligned", true),
	}
}

func (s *IntelServerSource) Close() error {
	return nil
}

func (s *IntelServerSource) Next(ctx context.Context) (image.Image, func(), error) {
	allData, err := readyBytesFromURL(s.client, s.BothURL)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't read url (%s): %s", s.BothURL, err)
	}

	img, err := rimage.BothReadFromBytes(allData, s.IsAligned())
	return img, func() {}, err
}
