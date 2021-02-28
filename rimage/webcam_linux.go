// +build linux

package rimage

import (
	"encoding/binary"
	"fmt"
	"image"
	"time"

	"github.com/blackjack/webcam"

	"github.com/edaniels/golog"
)

const depth16Bit = 540422490

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

type webcamDepthSource struct {
	cam       *webcam.Webcam
	w, h      uint32
	numFrames int
}

func (w *webcamDepthSource) Next() (*DepthMap, error) {
	f, err := readFrame(w.cam, w.numFrames == 0)
	if err != nil {
		return nil, err
	}
	w.numFrames++
	return decode16BitDepth(f, w.w, w.h), nil
}

func (w *webcamDepthSource) Close() error {
	return w.cam.Close()
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
