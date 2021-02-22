package vision

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/blackjack/webcam"

	"github.com/edaniels/golog"
)

const (
	// from https://github.com/blackjack/webcam/blob/master/examples/http_mjpeg_streamer/webcam.go
	v4l2PixFmtYuyv = 0x56595559
	jpegVideo      = 1196444237
)

type WebcamSource struct {
	cam           *webcam.Webcam
	format        webcam.PixelFormat
	width, height uint32
}

func (s *WebcamSource) Close() error {
	return s.cam.Close()
}

func (s *WebcamSource) NextImageDepthPair(ctx context.Context) (image.Image, *DepthMap, error) {
	i, err := s.Next(ctx)
	return i, nil, err
}

func (s *WebcamSource) decode(frame []byte) (image.Image, error) {

	switch s.format {
	case v4l2PixFmtYuyv:
		yuyv := image.NewYCbCr(image.Rect(0, 0, int(s.width), int(s.height)), image.YCbCrSubsampleRatio422)
		for i := range yuyv.Cb {
			ii := i * 4
			yuyv.Y[i*2] = frame[ii]
			yuyv.Y[i*2+1] = frame[ii+2]
			yuyv.Cb[i] = frame[ii+1]
			yuyv.Cr[i] = frame[ii+3]

		}
		return yuyv, nil
	case jpegVideo:
		return jpeg.Decode(bytes.NewReader(frame))
	default:
		panic("invalid format ? - should be impossible")
	}
}

func (s *WebcamSource) Next(ctx context.Context) (image.Image, error) {

	err := s.cam.WaitForFrame(1)
	if err != nil {
		return nil, fmt.Errorf("couldn't get webcam frame: %s", err)
	}

	frame, err := s.cam.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("couldn't read webcam frame: %s", err)
	}

	if len(frame) == 0 {
		return nil, fmt.Errorf("why is frame empty")
	}

	return s.decode(frame)
}

func tryWebcamOpen(path string) (ImageDepthSource, error) {
	cam, err := webcam.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open webcam [%s] : %s", path, err)
	}

	formats := cam.GetSupportedFormats()
	format := webcam.PixelFormat(0)

	goodFormats := []webcam.PixelFormat{v4l2PixFmtYuyv, jpegVideo}
	for _, f := range goodFormats {
		_, ok := formats[f]
		if !ok {
			continue
		}

		if len(cam.GetSupportedFrameSizes(f)) == 0 {
			continue
		}

		format = f
		break
	}

	if format == 0 {
		return nil, fmt.Errorf("no supported format, supported ones: %v", formats)
	}

	sizes := cam.GetSupportedFrameSizes(format)
	bestSize := 0
	for idx, s := range sizes {
		if s.MaxWidth > sizes[bestSize].MaxWidth {
			bestSize = idx
		}
	}

	format, w, h, err := cam.SetImageFormat(format, sizes[bestSize].MaxWidth, sizes[bestSize].MaxHeight)
	if err != nil {
		return nil, fmt.Errorf("cannot set image format: %s", err)
	}

	err = cam.SetBufferCount(2)
	if err != nil {
		return nil, fmt.Errorf("cannot SetBufferCount stream for %s : %s", path, err)
	}

	err = cam.StartStreaming()
	if err != nil {
		return nil, fmt.Errorf("cannot start webcam stream for %s : %s", path, err)
	}

	return &WebcamSource{cam, format, w, h}, nil
}

func NewWebcamSource(attrs map[string]string) (ImageDepthSource, error) {

	path := attrs["path"]

	if path != "" {
		return tryWebcamOpen(path)
	}

	for i := 0; i <= 20; i++ {
		path := fmt.Sprintf("/dev/video%d", i)
		s, err := tryWebcamOpen(path)
		if err == nil {
			golog.Global.Debugf("found webcam %s", path)
			return s, nil
		}
	}

	return nil, fmt.Errorf("could find no webcams")
}
