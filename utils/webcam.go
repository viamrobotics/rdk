package utils

import (
	"context"
	"fmt"
	"image"

	"gocv.io/x/gocv"
)

type WebcamSource struct {
	deviceID int
	webcam   *gocv.VideoCapture
}

func (we *WebcamSource) Close() error {
	return we.webcam.Close()
}

func (we *WebcamSource) Next(ctx context.Context) (image.Image, error) {
	img := gocv.NewMat()

	ok := we.webcam.Read(&img)
	if !ok {
		img.Close()
		return nil, fmt.Errorf("cannot read webcam device: %d", we.deviceID)
	}

	defer img.Close()
	return img.ToImage()
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
