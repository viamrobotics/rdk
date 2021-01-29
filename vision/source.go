package vision

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"io/ioutil"
	"net/http"

	"github.com/echolabsinc/robotcore/utils"

	"gocv.io/x/gocv"
)

type MatDepthSource interface {
	utils.MatSource
	NextMatDepthPair() (gocv.Mat, *DepthMap, error)
}

// -----
type StaticSource struct {
	pc *PointCloud
}

func (ss *StaticSource) NextMat() (gocv.Mat, error) {
	return ss.pc.Color.mat.Clone(), nil
}

func (ss *StaticSource) NextMatDepthPair() (gocv.Mat, *DepthMap, error) {
	mat, err := ss.NextMat()
	if err != nil {
		return mat, nil, err
	}
	return mat, ss.pc.Depth, nil
}

func (ss *StaticSource) Close() {
	ss.pc.Close()
}

// -----

type FileSource struct {
	ColorFN string
	DepthFN string
}

func (fs *FileSource) NextMat() (gocv.Mat, error) {
	var mat gocv.Mat
	if fs.ColorFN == "" {
		return mat, nil
	}
	return gocv.IMRead(fs.ColorFN, gocv.IMReadUnchanged), nil
}

func (fs *FileSource) NextMatDepthPair() (gocv.Mat, *DepthMap, error) {
	mat, err := fs.NextMat()
	if err != nil {
		return mat, nil, err
	}
	dm, err := ParseDepthMap(fs.DepthFN)
	return mat, dm, err
}

func (fs *FileSource) Close() {
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

func (hs *HTTPSource) NextMat() (gocv.Mat, error) {

	img := gocv.Mat{}

	colorData, err := readyBytesFromURL(hs.ColorURL)
	if err != nil {
		return img, fmt.Errorf("couldn't ready color url: %s", err)
	}

	img, err = gocv.IMDecode(colorData, gocv.IMReadUnchanged)
	return img, err
}

func (hs *HTTPSource) NextMatDepthPair() (gocv.Mat, *DepthMap, error) {

	img, err := hs.NextMat()
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

func (hs *HTTPSource) Close() {
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

func (s *IntelServerSource) Close() {
}

func (s *IntelServerSource) NextMat() (gocv.Mat, error) {
	m, _, err := s.NextMatDepthPair()
	return m, err
}

func (s *IntelServerSource) NextMatDepthPair() (gocv.Mat, *DepthMap, error) {
	allData, err := readyBytesFromURL(s.BothURL)
	if err != nil {
		return gocv.Mat{}, nil, fmt.Errorf("couldn't read url (%s): %s", s.BothURL, err)
	}

	return readNextMatDepthPairFromBoth(allData)
}

func readNextMatDepthPairFromBoth(allData []byte) (gocv.Mat, *DepthMap, error) {
	img := gocv.Mat{}

	reader := bufio.NewReader(bytes.NewReader(allData))
	depth, err := ReadDepthMap(reader)
	if err != nil {
		return img, nil, fmt.Errorf("couldn't read depth map (both): %w", err)
	}

	imgData, err := ioutil.ReadAll(reader)
	if err != nil {
		return img, nil, fmt.Errorf("couldn't read image (both): %w", err)
	}

	img, err = gocv.IMDecode(imgData, gocv.IMReadUnchanged)

	return img, depth, err
}

// ----

type SettableSource struct {
	TheImage    image.Image // we use this because it's easier to handle GC
	TheDepthMap DepthMap
}

func (ss *SettableSource) NextMat() (gocv.Mat, error) {
	m, err := gocv.ImageToMatRGB(ss.TheImage)
	return m, err
}

func (ss *SettableSource) NextMatDepthPair() (gocv.Mat, *DepthMap, error) {
	m, err := ss.NextMat()
	return m, &ss.TheDepthMap, err
}

func (ss *SettableSource) Close() {
}
