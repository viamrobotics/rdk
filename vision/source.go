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
	NextColorDepthPair() (*gocv.Mat, DepthMap, error)

	Close()
}

// ------

type WebcamSource struct {
	deviceId int
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
		return gocv.Mat{}, DepthMap{}, fmt.Errorf("cannot read webcam device: %d", we.deviceId)
	}

	return img, DepthMap{}, nil
}

func NewWebcamSource(deviceId int) (*WebcamSource, error) {
	var err error
	source := &WebcamSource{}

	source.deviceId = deviceId
	source.webcam, err = gocv.OpenVideoCapture(deviceId)
	if err != nil {
		return nil, err
	}

	return source, nil
}

// -------

type HttpSource struct {
	ColorURL string // this is for a generic image
	DepthURL string // this is for my bizarre custom data format for depth data
}

func _readyBytesFromUrl(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func (hs *HttpSource) NextColorDepthPair() (gocv.Mat, DepthMap, error) {

	img := gocv.Mat{}
	var depth DepthMap

	colorData, err := _readyBytesFromUrl(hs.ColorURL)
	if err != nil {
		return img, depth, fmt.Errorf("couldn't ready color url: %s", err)
	}

	depthData, err := _readyBytesFromUrl(hs.DepthURL)
	if err != nil {
		return img, depth, fmt.Errorf("couldn't ready depth url: %s", err)
	}

	// do this first and make sure ok before creating any mats
	dm, err := ReadDepthMap(bytes.NewReader(depthData))
	if err != nil {
		return img, depth, err
	}

	img, err = gocv.IMDecode(colorData, gocv.IMReadUnchanged)
	if err != nil {
		return img, depth, err
	}

	return img, dm, nil
}

func (hs *HttpSource) Close() {
}

func NewHttpSourceIntelEliot(root string) *HttpSource {
	return &HttpSource{
		fmt.Sprintf("http://%s/pic.png", root),
		fmt.Sprintf("http://%s/depth.dat", root),
	}
}
