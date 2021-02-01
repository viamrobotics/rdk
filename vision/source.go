package vision

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/edaniels/gostream"

	// register ppm
	_ "github.com/lmittmann/ppm"
)

type ImageDepthSource interface {
	gostream.ImageSource
	NextImageDepthPair(ctx context.Context) (image.Image, *DepthMap, error)
}

// -----
type StaticSource struct {
	pc *PointCloud
}

func (ss *StaticSource) Next(ctx context.Context) (image.Image, error) {
	return ss.pc.Color.Image(), nil
}

func (ss *StaticSource) NextImageDepthPair(ctx context.Context) (image.Image, *DepthMap, error) {
	mat, err := ss.Next(ctx)
	if err != nil {
		return mat, nil, err
	}
	return mat, ss.pc.Depth, nil
}

func (ss *StaticSource) Close() error {
	ss.pc.Close()
	return nil
}

// -----

type FileSource struct {
	ColorFN string
	DepthFN string
}

func (fs *FileSource) Next(ctx context.Context) (image.Image, error) {
	if fs.ColorFN == "" {
		return nil, nil
	}
	f, err := os.Open(fs.ColorFN)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	return img, err
}

func (fs *FileSource) NextImageDepthPair(ctx context.Context) (image.Image, *DepthMap, error) {
	mat, err := fs.Next(ctx)
	if err != nil {
		return mat, nil, err
	}
	dm, err := ParseDepthMap(fs.DepthFN)
	return mat, dm, err
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
	return img, err
}

func (hs *HTTPSource) NextImageDepthPair(ctx context.Context) (image.Image, *DepthMap, error) {

	img, err := hs.Next(ctx)
	if err != nil {
		return img, nil, err
	}

	var depth *DepthMap
	var depthData []byte
	if hs.DepthURL != "" {
		depthData, err = readyBytesFromURL(hs.DepthURL)
		if err != nil {
			return img, nil, fmt.Errorf("couldn't ready depth url: %s", err)
		}

		// do this first and make sure ok before creating any mats
		depth, err = ReadDepthMap(bufio.NewReader(bytes.NewReader(depthData)))
		if err != nil {
			return img, nil, err
		}
	}

	return img, depth, err
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

func (s *IntelServerSource) ColorURL() string {
	return fmt.Sprintf("http://%s/pic.ppm", s.host)
}

func (s *IntelServerSource) Close() error {
	return nil
}

func (s *IntelServerSource) Next(ctx context.Context) (image.Image, error) {
	m, _, err := s.NextImageDepthPair(ctx)
	return m, err
}

func (s *IntelServerSource) NextImageDepthPair(ctx context.Context) (image.Image, *DepthMap, error) {
	allData, err := readyBytesFromURL(s.BothURL)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't read url (%s): %s", s.BothURL, err)
	}

	return readNextImageDepthPairFromBoth(allData)
}

func readNextImageDepthPairFromBoth(allData []byte) (image.Image, *DepthMap, error) {
	reader := bufio.NewReader(bytes.NewReader(allData))
	depth, err := ReadDepthMap(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't read depth map (both): %w", err)
	}

	img, _, err := image.Decode(reader)
	return img, depth, err
}

// ----

type SettableSource struct {
	TheImage    image.Image // we use this because it's easier to handle GC
	TheDepthMap DepthMap
}

func (ss *SettableSource) Next(ctx context.Context) (image.Image, error) {
	return ss.TheImage, nil
}

func (ss *SettableSource) NextImageDepthPair(ctx context.Context) (image.Image, *DepthMap, error) {
	m, err := ss.Next(ctx)
	return m, &ss.TheDepthMap, err
}

func (ss *SettableSource) Close() error {
	return nil
}
