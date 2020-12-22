package vision

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	"gocv.io/x/gocv"
)

type MatSource interface {

	// first back is the regular img
	// second is the depth if it exists
	NextColorDepthPair() (gocv.Mat, DepthMap, error)

	Close()
}

// ------

type WebcamSource struct {
	deviceID int
	webcam   *gocv.VideoCapture
}

func (we *WebcamSource) Close() {
	we.webcam.Close()
}

func (we *WebcamSource) NextColorDepthPair() (gocv.Mat, DepthMap, error) {
	img := gocv.NewMat()

	ok := we.webcam.Read(&img)
	if !ok {
		img.Close()
		return gocv.Mat{}, DepthMap{}, fmt.Errorf("cannot read webcam device: %d", we.deviceID)
	}

	return img, DepthMap{}, nil
}

func NewWebcamSource(deviceID int) (*WebcamSource, error) {
	var err error
	source := &WebcamSource{}

	source.deviceID = deviceID
	source.webcam, err = gocv.OpenVideoCapture(deviceID)
	if err != nil {
		return nil, err
	}

	return source, nil
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

func (hs *HTTPSource) NextColorDepthPair() (gocv.Mat, DepthMap, error) {

	img := gocv.Mat{}
	var depth DepthMap

	colorData, err := readyBytesFromURL(hs.ColorURL)
	if err != nil {
		return img, depth, fmt.Errorf("couldn't ready color url: %s", err)
	}

	var depthData []byte
	if hs.DepthURL != "" {
		depthData, err = readyBytesFromURL(hs.DepthURL)
		if err != nil {
			return img, depth, fmt.Errorf("couldn't ready depth url: %s", err)
		}

		// do this first and make sure ok before creating any mats
		depth, err = ReadDepthMap(bytes.NewReader(depthData))
		if err != nil {
			return img, depth, err
		}
	}

	img, err = gocv.IMDecode(colorData, gocv.IMReadUnchanged)

	return img, depth, err
}

func (hs *HTTPSource) Close() {
}

func NewHTTPSourceIntelEliot(root string) *HTTPSource {
	return &HTTPSource{
		fmt.Sprintf("http://%s/pic.ppm", root),
		fmt.Sprintf("http://%s/depth.dat", root),
	}
}
