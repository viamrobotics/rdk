//go:build linux

package videosource

// This file is adapted from github.com/pion/mediadevices/pkg/driver/camera/camera_linux.go at v0.10.0.
//
// It exists to fix a use-after-release race in the upstream frame read loop. Upstream calls
// webcam.ReadFrame(), which dequeues a V4L2 buffer (VIDIOC_DQBUF), immediately re-enqueues it to the
// kernel (VIDIOC_QBUF), and only then returns a slice pointing into that same mmap'd buffer. Upstream
// copies the frame out of that slice afterwards, so the kernel may already be writing the next frame
// into the buffer while the copy is in progress. With the upstream default of 2 buffers this happens
// whenever the consumer is one frame behind, and on MJPEG cameras it surfaces as sporadic
// "invalid JPEG format: ..." decode errors (truncated or interleaved frames).
//
// This copy dequeues the buffer with GetFrame(), copies the bytes out, and only then releases the
// buffer with ReleaseFrame(). It also uses 4 buffers by default and replaces the upstream cgo fourcc
// constants with plain Go values so the file builds without cgo.
//
// Once the equivalent fix has landed upstream and RDK has bumped pion/mediadevices to a release that
// contains it, this file (and v4l2_driver_other.go) can be deleted and findReaderAndDriver can go back
// to calling mediadevicescamera.Initialize().

import (
	"context"
	"errors"
	"image"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	v4l2 "github.com/blackjack/webcam"
	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/driver/availability"
	mediadevicescamera "github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
)

const (
	maxEmptyFrameCount = 5
	prioritizedDevice  = "video0"

	// defaultV4L2BufferCount is the number of mmap'd V4L2 capture buffers requested from the kernel.
	// Upstream uses 2. Since the read loop below holds a buffer only for the duration of a memcpy,
	// 2 would be sufficient in theory; 4 gives slack for GC pauses and scheduling delays on small
	// boards and matches what most V4L2 userland (libuvc, GStreamer) requests.
	defaultV4L2BufferCount = 4
	// v4l2BufferCountEnv overrides defaultV4L2BufferCount. It is a diagnostic knob for comparing
	// buffer counts on real hardware and is intentionally undocumented.
	v4l2BufferCountEnv = "VIAM_WEBCAM_V4L2_BUFFERS"
	// v4l2ReadTimeoutEnv mirrors the upstream PION_MEDIADEVICES_CAMERA_READ_TIMEOUT override.
	v4l2ReadTimeoutEnv = "PION_MEDIADEVICES_CAMERA_READ_TIMEOUT"
)

var (
	errReadTimeout    = errors.New("read timeout")
	errEmptyFrame     = errors.New("empty frame")
	errTruncatedFrame = errors.New("truncated MJPEG frame")
	// Reference: https://commons.wikimedia.org/wiki/File:Vector_Video_Standards2.svg
	supportedResolutions = [][2]int{
		{320, 240},
		{640, 480},
		{768, 576},
		{800, 600},
		{1024, 768},
		{1280, 854},
		{1280, 960},
		{1280, 1024},
		{1400, 1050},
		{1600, 1200},
		{2048, 1536},
		{320, 200},
		{800, 480},
		{854, 480},
		{1024, 600},
		{1152, 768},
		{1280, 720},
		{1280, 768},
		{1366, 768},
		{1280, 800},
		{1440, 900},
		{1440, 960},
		{1680, 1050},
		{1920, 1080},
		{2048, 1080},
		{1920, 1200},
		{2560, 1600},
	}
)

// fourcc builds a V4L2 pixel format code the same way the v4l2_fourcc macro in
// <linux/videodev2.h> does, so this file does not need cgo.
func fourcc(a, b, c, d byte) v4l2.PixelFormat {
	return v4l2.PixelFormat(uint32(a) | uint32(b)<<8 | uint32(c)<<16 | uint32(d)<<24)
}

// V4L2 pixel formats supported by this driver. Values match V4L2_PIX_FMT_* in <linux/videodev2.h>.
var (
	pixFmtYUV420 = fourcc('Y', 'U', '1', '2')
	pixFmtNV21   = fourcc('N', 'V', '2', '1')
	pixFmtNV12   = fourcc('N', 'V', '1', '2')
	pixFmtYUYV   = fourcc('Y', 'U', 'Y', 'V')
	pixFmtUYVY   = fourcc('U', 'Y', 'V', 'Y')
	pixFmtMJPEG  = fourcc('M', 'J', 'P', 'G')
	pixFmtZ16    = fourcc('Z', '1', '6', ' ')
)

// v4l2Camera is a camera driver implementation using V4L2.
// Reference: https://linuxtv.org/downloads/v4l-dvb-apis/uapi/v4l/videodev.html#videodev
type v4l2Camera struct {
	path            string
	cam             *v4l2.Webcam
	formats         map[v4l2.PixelFormat]frame.Format
	reversedFormats map[frame.Format]v4l2.PixelFormat
	bufCount        int
	mutex           sync.Mutex
	cancel          func()
	prevFrameTime   time.Time
}

// initializeDrivers finds V4L2 camera devices and registers them with the mediadevices driver
// manager, replacing any video drivers registered before (including the ones the upstream
// mediadevices camera package registers from its init function).
func initializeDrivers() {
	manager := driver.GetManager()
	for _, d := range manager.Query(driver.FilterVideoRecorder()) {
		manager.Delete(d.ID())
	}
	discovered := make(map[string]struct{})
	discoverV4L2(discovered, "/dev/v4l/by-id/*")
	discoverV4L2(discovered, "/dev/v4l/by-path/*")
	discoverV4L2(discovered, "/dev/video*")
}

func discoverV4L2(discovered map[string]struct{}, pattern string) {
	devices, err := filepath.Glob(pattern)
	if err != nil {
		// No v4l device.
		return
	}
	for _, device := range devices {
		label := filepath.Base(device)
		reallink, err := os.Readlink(device)
		if err != nil {
			reallink = label
		} else {
			reallink = filepath.Base(reallink)
		}
		if _, ok := discovered[reallink]; ok {
			continue
		}

		discovered[reallink] = struct{}{}
		cam := newV4L2Camera(device)
		priority := driver.PriorityNormal
		if reallink == prioritizedDevice {
			priority = driver.PriorityHigh
		}

		var name, busInfo string
		if webcamCam, err := v4l2.Open(cam.path); err == nil {
			if n, err := webcamCam.GetName(); err == nil {
				name = n
			}
			if b, err := webcamCam.GetBusInfo(); err == nil {
				busInfo = b
			}
			//nolint:errcheck
			webcamCam.Close()
		}

		// Name and Label must stay in the same format as upstream mediadevices so that config
		// video_path values and the webcam discovery service keep working unchanged.
		//nolint:errcheck
		driver.GetManager().Register(cam, driver.Info{
			// 	Source: https://www.kernel.org/doc/html/v4.9/media/uapi/v4l/vidioc-querycap.html
			//	Name of the device, a NUL-terminated UTF-8 string. For example: “Yoyodyne TV/FM”. One driver may support
			//	different brands or models of video hardware. This information is intended for users, for example in a
			//	menu of available devices. Since multiple TV cards of the same brand may be installed which are
			//	supported by the same driver, this name should be combined with the character device file name
			//	(e.g. /dev/video2) or the bus_info string to avoid ambiguities.
			Name:       name + mediadevicescamera.LabelSeparator + busInfo,
			Label:      label + mediadevicescamera.LabelSeparator + reallink,
			DeviceType: driver.Camera,
			Priority:   priority,
		})
	}
}

func newV4L2Camera(path string) *v4l2Camera {
	formats := map[v4l2.PixelFormat]frame.Format{
		pixFmtYUV420: frame.FormatI420,
		pixFmtNV21:   frame.FormatNV21,
		pixFmtNV12:   frame.FormatNV12,
		pixFmtYUYV:   frame.FormatYUYV,
		pixFmtUYVY:   frame.FormatUYVY,
		pixFmtMJPEG:  frame.FormatMJPEG,
		pixFmtZ16:    frame.FormatZ16,
	}

	reversedFormats := make(map[frame.Format]v4l2.PixelFormat)
	for k, v := range formats {
		reversedFormats[v] = k
	}

	return &v4l2Camera{
		path:            path,
		formats:         formats,
		reversedFormats: reversedFormats,
		bufCount:        v4l2BufferCount(),
	}
}

// v4l2BufferCount returns the number of V4L2 buffers to request, honoring the env override.
func v4l2BufferCount() int {
	if val, ok := os.LookupEnv(v4l2BufferCountEnv); ok {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return defaultV4L2BufferCount
}

func getCameraReadTimeout() uint32 {
	// default to 5 seconds
	var readTimeoutSec uint32 = 5
	if val, ok := os.LookupEnv(v4l2ReadTimeoutEnv); ok {
		if valInt, err := strconv.Atoi(val); err == nil {
			if valInt > 0 {
				readTimeoutSec = uint32(valInt)
			}
		}
	}
	return readTimeoutSec
}

func (c *v4l2Camera) Open() error {
	cam, err := v4l2.Open(c.path)
	if err != nil {
		return err
	}

	// Buffering should be handled in higher level.
	err = cam.SetBufferCount(uint32(c.bufCount))
	if err != nil {
		return err
	}

	c.prevFrameTime = time.Now()
	c.cam = cam
	return nil
}

func (c *v4l2Camera) Close() error {
	if c.cam == nil {
		return nil
	}

	if c.cancel != nil {
		// Let the reader knows that the caller has closed the camera
		c.cancel()
		// Wait until the reader unref the buffer
		c.mutex.Lock()
		defer c.mutex.Unlock()

		// Note: StopStreaming frees frame buffers even if they are still used in Go code.
		//       There is currently no convenient way to do this safely.
		//       So, consumer of this stream must close camera after unusing all images.
		//nolint:errcheck
		c.cam.StopStreaming()
		c.cancel = nil
	}
	return c.cam.Close()
}

func (c *v4l2Camera) VideoRecord(p prop.Media) (video.Reader, error) {
	decoder, err := frame.NewDecoder(p.FrameFormat)
	if err != nil {
		return nil, err
	}

	pf := c.reversedFormats[p.FrameFormat]
	//nolint:dogsled
	_, _, _, err = c.cam.SetImageFormat(pf, uint32(p.Width), uint32(p.Height))
	if err != nil {
		return nil, err
	}

	if p.FrameRate > 0 {
		err = c.cam.SetFramerate(float32(p.FrameRate))
		if err != nil {
			return nil, err
		}
	}

	if err := c.cam.StartStreaming(); err != nil {
		return nil, err
	}

	cam := c.cam
	bufCount := c.bufCount

	readTimeoutSec := getCameraReadTimeout()

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	isMJPEG := p.FrameFormat == frame.FormatMJPEG
	var buf []byte
	r := video.ReaderFunc(func() (img image.Image, release func(), err error) {
		// Lock to avoid accessing the buffer after StopStreaming()
		c.mutex.Lock()
		defer c.mutex.Unlock()

		truncated := 0
		// Wait until a frame is ready
		for i := 0; i < maxEmptyFrameCount; i++ {
			if ctx.Err() != nil {
				// Return EOF if the camera is already closed.
				return nil, func() {}, io.EOF
			}

			if p.DiscardFramesOlderThan != 0 && time.Since(c.prevFrameTime) >= p.DiscardFramesOlderThan {
				// Drain the queue of stale frames. ReadFrame is fine here because the bytes are discarded.
				// Errors are not fatal: if nothing is queued there is nothing to drain.
				for i := 0; i < bufCount; i++ {
					if err := cam.WaitForFrame(readTimeoutSec); err != nil {
						break
					}
					if _, err := cam.ReadFrame(); err != nil {
						break
					}
				}
			}

			if err := cam.WaitForFrame(readTimeoutSec); err != nil {
				var timeout *v4l2.Timeout
				if errors.As(err, &timeout) {
					return nil, func() {}, errReadTimeout
				}
				// Camera has been stopped.
				return nil, func() {}, err
			}

			// Dequeue the buffer. Until ReleaseFrame is called the kernel will not write to it, so
			// the copy below is safe. (Upstream uses ReadFrame, which re-enqueues the buffer before
			// returning the slice, letting the kernel overwrite the frame while it is being copied.)
			b, index, err := cam.GetFrame()
			if err != nil {
				// Camera has been stopped. Nothing was dequeued, so nothing to release.
				return nil, func() {}, err
			}

			if len(b) > len(buf) {
				// Grow the intermediate buffer
				buf = make([]byte, len(b))
			}

			// move the memory from mmap to Go. This will guarantee that any data that's going out
			// from this reader will be Go safe. Otherwise, it's possible that outside of this reader
			// that this memory is still being used even after we close it.
			n := copy(buf, b)

			// Hand the buffer back to the kernel now that we have our own copy. This must happen on
			// every path after a successful GetFrame, otherwise the queue runs dry and reads time out.
			if err := cam.ReleaseFrame(index); err != nil {
				return nil, func() {}, err
			}

			if p.DiscardFramesOlderThan != 0 {
				c.prevFrameTime = time.Now()
			}

			// Frame is empty.
			// Retry reading and return errEmptyFrame if it exceeds maxEmptyFrameCount.
			if n == 0 {
				continue
			}

			// The kernel does not flag truncated compressed frames (see mjpegFrameComplete), so
			// treat one like an empty frame and try the next buffer rather than decoding it.
			if isMJPEG && !mjpegFrameComplete(buf[:n]) {
				truncated++
				continue
			}

			return decoder.Decode(buf[:n], p.Width, p.Height)
		}
		if truncated > 0 {
			return nil, func() {}, errTruncatedFrame
		}
		return nil, func() {}, errEmptyFrame
	})

	return r, nil
}

func (c *v4l2Camera) Properties() []prop.Media {
	properties := make([]prop.Media, 0)
	for format := range c.cam.GetSupportedFormats() {
		for _, frameSize := range c.cam.GetSupportedFrameSizes(format) {
			supportedFormat, ok := c.formats[format]
			if !ok {
				continue
			}

			if frameSize.StepWidth == 0 || frameSize.StepHeight == 0 {
				framerates := c.cam.GetSupportedFramerates(format, frameSize.MaxWidth, frameSize.MaxHeight)
				// If the camera doesn't support framerate, we just add the resolution and format
				if len(framerates) == 0 {
					properties = append(properties, prop.Media{
						Video: prop.Video{
							Width:       int(frameSize.MaxWidth),
							Height:      int(frameSize.MaxHeight),
							FrameFormat: supportedFormat,
						},
					})
					continue
				}

				for _, framerate := range framerates {
					for _, fps := range enumFramerate(framerate) {
						properties = append(properties, prop.Media{
							Video: prop.Video{
								Width:       int(frameSize.MaxWidth),
								Height:      int(frameSize.MaxHeight),
								FrameFormat: supportedFormat,
								FrameRate:   fps,
							},
						})
					}
				}
			} else {
				// FIXME: we should probably use a custom data structure to capture all of the supported resolutions
				for _, supportedResolution := range supportedResolutions {
					minWidth, minHeight := int(frameSize.MinWidth), int(frameSize.MinHeight)
					maxWidth, maxHeight := int(frameSize.MaxWidth), int(frameSize.MaxHeight)
					stepWidth, stepHeight := int(frameSize.StepWidth), int(frameSize.StepHeight)
					width, height := supportedResolution[0], supportedResolution[1]

					if width < minWidth || width > maxWidth ||
						height < minHeight || height > maxHeight {
						continue
					}

					if (width-minWidth)%stepWidth != 0 ||
						(height-minHeight)%stepHeight != 0 {
						continue
					}

					framerates := c.cam.GetSupportedFramerates(format, uint32(width), uint32(height))
					if len(framerates) == 0 {
						properties = append(properties, prop.Media{
							Video: prop.Video{
								Width:       width,
								Height:      height,
								FrameFormat: supportedFormat,
							},
						})
						continue
					}

					for _, framerate := range framerates {
						for _, fps := range enumFramerate(framerate) {
							properties = append(properties, prop.Media{
								Video: prop.Video{
									Width:       width,
									Height:      height,
									FrameFormat: supportedFormat,
									FrameRate:   fps,
								},
							})
						}
					}
				}
			}
		}
	}
	return properties
}

func (c *v4l2Camera) IsAvailable() (bool, error) {
	var err error

	// close the opened file descriptor as quickly as possible and in all cases, including panics
	func() {
		var cam *v4l2.Webcam
		if cam, err = v4l2.Open(c.path); err == nil {
			//nolint:errcheck
			defer cam.Close()
			var index int32
			// "Drivers must implement all the input ioctls when the device has one or more inputs..."
			// Source: https://www.kernel.org/doc/html/latest/userspace-api/media/v4l/video.html?highlight=vidioc_enuminput
			if index, err = cam.GetInput(); err == nil {
				err = cam.SelectInput(uint32(index))
			}
		}
	}()

	var errno syscall.Errno
	errors.As(err, &errno)

	// See https://man7.org/linux/man-pages/man3/errno.3.html
	switch {
	case err == nil:
		return true, nil
	case errno == syscall.EBUSY:
		return false, availability.ErrBusy
	case errno == syscall.ENODEV || errno == syscall.ENOENT:
		return false, availability.ErrNoDevice
	default:
		return false, availability.NewError(errno.Error())
	}
}

// enumFramerate returns a list of fps options from a FrameRate struct.
// discrete framerates will return a list of 1 fps element.
// stepwise framerates will return a list of all possible fps options.
func enumFramerate(framerate v4l2.FrameRate) []float32 {
	var framerates []float32
	if framerate.StepNumerator == 0 && framerate.StepDenominator == 0 {
		fr, err := calcFramerate(framerate.MaxNumerator, framerate.MaxDenominator)
		if err != nil {
			return framerates
		}
		framerates = append(framerates, fr)
	} else {
		for n := framerate.MinNumerator; n <= framerate.MaxNumerator; n += framerate.StepNumerator {
			for d := framerate.MinDenominator; d <= framerate.MaxDenominator; d += framerate.StepDenominator {
				fr, err := calcFramerate(n, d)
				if err != nil {
					continue
				}
				framerates = append(framerates, fr)
			}
		}
	}
	return framerates
}

// calcFramerate turns fraction into a float32 fps value.
func calcFramerate(numerator, denominator uint32) (float32, error) {
	if denominator == 0 {
		return 0, errors.New("framerate denominator is zero")
	}
	// round to three decimal places to avoid floating point precision issues
	return float32(math.Round(1000.0/((float64(numerator))/float64(denominator))) / 1000), nil
}
