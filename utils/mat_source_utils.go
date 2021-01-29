package utils

import (
	"fmt"

	"gocv.io/x/gocv"
)

type RotateMatSource struct {
	Original MatSource
}

func (rms *RotateMatSource) NextMat() (gocv.Mat, error) {
	m, err := rms.Original.NextMat()
	if err != nil {
		return m, err
	}
	gocv.Rotate(m, &m, gocv.Rotate180Clockwise)
	return m, nil
}

func (rms *RotateMatSource) Close() {
	rms.Original.Close()
}

type WebcamSource struct {
	deviceID int
	webcam   *gocv.VideoCapture
}

func (we *WebcamSource) Close() {
	we.webcam.Close()
}

func (we *WebcamSource) NextMat() (gocv.Mat, error) {
	img := gocv.NewMat()

	ok := we.webcam.Read(&img)
	if !ok {
		img.Close()
		return gocv.Mat{}, fmt.Errorf("cannot read webcam device: %d", we.deviceID)
	}

	return img, nil
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
