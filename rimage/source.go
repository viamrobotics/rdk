package rimage

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"io/ioutil"
	"net/http"

	_ "github.com/lmittmann/ppm" // register ppm
)

type StaticSource struct {
	Img *ImageWithDepth
}

func (ss *StaticSource) Next(ctx context.Context) (image.Image, error) {
	return ss.Img, nil
}

func (ss *StaticSource) Close() error {
	return nil
}

// -----

type FileSource struct {
	ColorFN string
	DepthFN string
}

func (fs *FileSource) Next(ctx context.Context) (image.Image, error) {
	return NewImageWithDepth(fs.ColorFN, fs.DepthFN)
}

func (fs *FileSource) Close() error {
	return nil
}

// -------

type HTTPSource struct {
	ColorURL string // this is for a generic image
	DepthURL string // this is for my bizarre custom data format for depth data
}

func readyBytesFromURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func (hs *HTTPSource) Next(ctx context.Context) (image.Image, error) {
	colorData, err := readyBytesFromURL(hs.ColorURL)
	if err != nil {
		return nil, fmt.Errorf("couldn't ready color url: %s", err)
	}

	img, _, err := image.Decode(bytes.NewBuffer(colorData))
	if err != nil {
		return nil, err
	}

	if hs.DepthURL == "" {
		return img, nil
	}

	depthData, err := readyBytesFromURL(hs.DepthURL)
	if err != nil {
		return nil, fmt.Errorf("couldn't ready depth url: %s", err)
	}

	// do this first and make sure ok before creating any mats
	depth, err := ReadDepthMap(bufio.NewReader(bytes.NewReader(depthData)))
	if err != nil {
		return nil, err
	}

	return &ImageWithDepth{ConvertImage(img), depth}, nil
}

func (hs *HTTPSource) Close() error {
	return nil
}

// ------

type IntelServerSource struct {
	BothURL string
	host    string
}

func NewIntelServerSource(host string, port int, attrs map[string]string) *IntelServerSource {
	num := "0"
	numString, has := attrs["num"]
	if has {
		num = numString
	}
	return &IntelServerSource{fmt.Sprintf("http://%s:%d/both?num=%s", host, port, num), host}
}

func (s *IntelServerSource) Close() error {
	return nil
}

func (s *IntelServerSource) Next(ctx context.Context) (image.Image, error) {
	allData, err := readyBytesFromURL(s.BothURL)
	if err != nil {
		return nil, fmt.Errorf("couldn't read url (%s): %s", s.BothURL, err)
	}

	return BothReadFromBytes(allData)
}
