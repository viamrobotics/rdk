package rimage

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"time"

	"github.com/blackjack/webcam"

	"github.com/edaniels/golog"
)

const (
	// from https://github.com/blackjack/webcam/blob/master/examples/http_mjpeg_streamer/webcam.go
	v4l2PixFmtYuyv = 0x56595559
	jpegVideo      = 1196444237
	depth16Bit     = 540422490
)

type WebcamSource struct {
	cam           *webcam.Webcam
	format        webcam.PixelFormat
	width, height uint32
	numFrames     int
	depth         *webcamDepthSource
}

func (s *WebcamSource) Close() error {
	return s.cam.Close()
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
	f, err := readFrame(s.cam, s.numFrames == 0)
	if err != nil {
		return nil, err
	}

	s.numFrames++
	img, err := s.decode(f)
	if err != nil {
		return nil, err
	}

	if s.depth == nil {
		return img, nil
	}

	dm, err := s.depth.Next()
	if err != nil {
		return nil, err
	}
	return &ImageWithDepth{ConvertImage(img), dm}, nil

}

func readFrame(cam *webcam.Webcam, first bool) ([]byte, error) {
	var err error
	var frame []byte

	for tries := 0; tries == 0 || (first && tries < 5); tries++ {
		if tries > 0 {
			time.Sleep(50 * time.Millisecond)
		}

		err = cam.WaitForFrame(1)
		if err != nil {
			err = fmt.Errorf("couldn't get webcam frame: %s", err)
			continue
		}

		frame, err = cam.ReadFrame()
		if err != nil {
			err = fmt.Errorf("couldn't read webcam frame: %s", err)
			continue
		}

		if len(frame) == 0 {
			err = fmt.Errorf("why is frame empty")
			continue
		}
		return frame, nil
	}

	return nil, err
}

func pickSizeAndStart(cam *webcam.Webcam, format webcam.PixelFormat, desiredSize *image.Point) (webcam.PixelFormat, uint32, uint32, error) {
	sizes := cam.GetSupportedFrameSizes(format)
	bestSize := -1
	for idx, s := range sizes {
		if desiredSize != nil &&
			desiredSize.X != int(s.MaxWidth) &&
			desiredSize.Y != int(s.MaxHeight) {
			continue
		}

		if bestSize < 0 || s.MaxWidth > sizes[bestSize].MaxWidth {
			bestSize = idx
		}
	}

	if bestSize < 0 {
		return format, 0, 0, fmt.Errorf("no matching size")
	}

	newFormat, w, h, err := cam.SetImageFormat(format, sizes[bestSize].MaxWidth, sizes[bestSize].MaxHeight)
	if err != nil {
		err = fmt.Errorf("cannot set image format: %s", err)
	} else if newFormat != format {
		err = fmt.Errorf("setting format didn't stick")
	} else {
		err = cam.SetBufferCount(2)
		if err != nil {
			err = fmt.Errorf("cannot SetBufferCount stream: %s", err)
		} else {
			err = cam.StartStreaming()
			if err != nil {
				err = fmt.Errorf("cannot start webcam stream for: %s", err)
			}
		}
	}

	return newFormat, w, h, err
}

type webcamDepthSource struct {
	cam       *webcam.Webcam
	w, h      uint32
	numFrames int
}

func decode16BitDepth(b []byte, w uint32, h uint32) *DepthMap {
	if int(w*h*2) != len(b) {
		panic(fmt.Errorf("bad args to decode16BitDepth w: %d h: %d len: %d", w, h, len(b)))
	}

	// v4l seems to rotate??
	dm := NewEmptyDepthMap(int(h), int(w))

	for x := 0; x < dm.width; x++ {
		for y := 0; y < dm.height; y++ {
			//k := (y*dm.width) + x
			k := (x * dm.height) + y
			v := binary.LittleEndian.Uint16(b[(2 * k):])
			dm.Set(dm.width-x-1, dm.height-y-1, int(v))
		}
	}

	return &dm
}

func (w *webcamDepthSource) Next() (*DepthMap, error) {
	f, err := readFrame(w.cam, w.numFrames == 0)
	if err != nil {
		return nil, err
	}
	w.numFrames++
	return decode16BitDepth(f, w.w, w.h), nil
}

func findWebcamDepth(debug bool) (*webcamDepthSource, error) {
	for i := 0; i <= 20; i++ {
		path := fmt.Sprintf("/dev/video%d", i)
		if debug {
			golog.Global.Debugf("wc %s", path)
		}

		cam, err := openWebcamDepth(path, debug)
		if err != nil {
			if debug {
				golog.Global.Debugf("\t %s", err)
			}
			continue
		}

		return cam, nil
	}

	return nil, fmt.Errorf("no depth camera found")
}

func openWebcamDepth(path string, debug bool) (*webcamDepthSource, error) {
	cam, err := webcam.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open webcam [%s] : %s", path, err)
	}

	formats := cam.GetSupportedFormats()
	format := webcam.PixelFormat(depth16Bit)

	_, ok := formats[format]
	if !ok {
		return nil, fmt.Errorf("depth not supported on %s", path)
	}

	_, w, h, err := pickSizeAndStart(cam, format, nil)
	if err != nil {
		return nil, err
	}

	return &webcamDepthSource{cam, w, h, 0}, nil
}

func tryWebcamOpen(path string, debug bool, desiredSize *image.Point) (*WebcamSource, error) {
	cam, err := webcam.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open webcam [%s] : %s", path, err)
	}

	formats := cam.GetSupportedFormats()
	format := webcam.PixelFormat(0)

	if debug && len(formats) > 0 {
		for k, v := range formats {
			golog.Global.Debugf("\t %v : %v", k, v)
			for _, s := range cam.GetSupportedFrameSizes(k) {
				golog.Global.Debugf("\t\t %v", s)
			}
		}
	}

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

	format, w, h, err := pickSizeAndStart(cam, format, desiredSize)
	if err != nil {
		return nil, err
	}

	if debug {
		golog.Global.Debugf("\t CHOSE %s format: %v w: %v h: %v", path, format, w, h)
	}

	return &WebcamSource{cam, format, w, h, 0, nil}, nil
}

func NewWebcamSource(attrs map[string]string) (gostream.ImageSource, error) {

	debug := attrs["debug"] == "true"
	var desiredSize *image.Point = nil

	// TODO(erh): this is gross, but not sure what right config options are yet
	isIntel515 := attrs["model"] == "intel515"

	if isIntel515 {
		desiredSize = &image.Point{1280, 720}
	}

	path := attrs["path"]
	if path != "" {
		return tryWebcamOpen(path, debug, desiredSize)
	}

	for i := 0; i <= 20; i++ {
		path := fmt.Sprintf("/dev/video%d", i)
		if debug {
			golog.Global.Debugf("%s", path)
		}

		s, err := tryWebcamOpen(path, debug, desiredSize)
		if err == nil {
			if debug {
				golog.Global.Debugf("\t USING")
			}

			if isIntel515 {
				s.depth, err = findWebcamDepth(debug)
				if err != nil {
					return nil, fmt.Errorf("found intel camera point no matching depth")
				}
			}

			return s, nil
		}
		if debug {
			golog.Global.Debugf("\t %s", err)
		}
	}

	return nil, fmt.Errorf("could find no webcams")
}
