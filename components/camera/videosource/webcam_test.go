package videosource

import (
	"context"
	"errors"
	"testing"

	"github.com/pion/mediadevices/pkg/driver"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

func TestWebcamValidation(t *testing.T) {
	webCfg := &WebcamConfig{
		Width:     1280,
		Height:    640,
		FrameRate: 100,
	}

	// no error with positive width, height, and frame rate
	deps, _, err := webCfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldResemble, []string{})

	// no error with 0 width, 0 height and frame rate
	webCfg.Width = 0
	webCfg.Height = 0
	webCfg.FrameRate = 0
	deps, _, err = webCfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldResemble, []string{})

	// error with a negative width
	webCfg.Width = -200
	deps, _, err = webCfg.Validate("path")
	test.That(t, err.Error(), test.ShouldEqual,
		"got illegal negative dimensions for width_px and height_px (-200, 0) fields set for webcam camera")
	test.That(t, deps, test.ShouldBeNil)

	// error with a negative height
	webCfg.Width = 200
	webCfg.Height = -200
	deps, _, err = webCfg.Validate("path")
	test.That(t, err.Error(), test.ShouldEqual,
		"got illegal negative dimensions for width_px and height_px (200, -200) fields set for webcam camera")
	test.That(t, deps, test.ShouldBeNil)

	// error with a negative frame rate
	webCfg.Height = 200
	webCfg.FrameRate = -100
	deps, _, err = webCfg.Validate("path")
	test.That(t, err.Error(), test.ShouldEqual,
		"got illegal negative frame rate (-100.00) field set for webcam camera")
	test.That(t, deps, test.ShouldBeNil)
}

// waitForFrame blocks until the buffer worker has delivered a frame and returns it.
func waitForFrame(t *testing.T, cam *webcam) camera.NamedImage {
	t.Helper()
	var img camera.NamedImage
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		imgs, _, err := cam.Images(context.Background(), nil, nil)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, imgs, test.ShouldHaveLength, 1)
		if len(imgs) == 1 {
			img = imgs[0]
		}
	})
	return img
}

// waitForImagesError blocks until Images returns an error satisfying errors.Is(err, want).
func waitForImagesError(t *testing.T, cam *webcam, want error) {
	t.Helper()
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		_, _, err := cam.Images(context.Background(), nil, nil)
		test.That(tb, errors.Is(err, want), test.ShouldBeTrue)
	})
}

func TestNewWebcam(t *testing.T) {
	logger := logging.NewTestLogger(t)
	registerFakeDriver(t, "fake-cam", newFakeDriver(640, 480))

	cam, err := newTestWebcam(t, WebcamConfig{Path: "fake-cam"}, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cam.targetPath, test.ShouldEqual, "fake-cam")

	img := waitForFrame(t, cam)
	test.That(t, img.SourceName, test.ShouldEqual, "cam1")
	test.That(t, img.MimeType(), test.ShouldEqual, utils.MimeTypeJPEG)
	bounds, err := img.Bounds()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bounds.Dx(), test.ShouldEqual, 640)
	test.That(t, bounds.Dy(), test.ShouldEqual, 480)

	props, err := cam.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.SupportsPCD, test.ShouldBeFalse)
	test.That(t, props.ImageType, test.ShouldEqual, camera.ColorStream)
	test.That(t, props.FrameRate, test.ShouldEqual, defaultFrameRate)
	test.That(t, props.MimeTypes, test.ShouldResemble, []string{utils.MimeTypeJPEG, utils.MimeTypePNG, utils.MimeTypeRawRGBA})

	_, err = cam.NextPointCloud(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)

	geoms, err := cam.Geometries(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, geoms, test.ShouldBeEmpty)
}

func TestNewWebcamNotFound(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fake := newFakeDriver(640, 480)
	registerFakeDriver(t, "fake-cam", fake)

	_, err := newTestWebcam(t, WebcamConfig{Path: "missing"}, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "failed to find camera")

	opens, _ := fake.counts()
	test.That(t, opens, test.ShouldEqual, 0)
}

func TestNewWebcamAnyPath(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fake := registerFakeDriver(t, "fake-cam;Fake Cam", newFakeDriver(640, 480))
	skipUnlessOnlyFakesRegistered(t, fake)

	cam, err := newTestWebcam(t, WebcamConfig{}, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cam.targetPath, test.ShouldEqual, "fake-cam")
	waitForFrame(t, cam)
}

func TestWebcamClose(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fake := newFakeDriver(640, 480)
	d := registerFakeDriver(t, "fake-cam", fake)

	cam, err := newTestWebcam(t, WebcamConfig{Path: "fake-cam"}, logger)
	test.That(t, err, test.ShouldBeNil)
	waitForFrame(t, cam)
	_, closesBefore := fake.counts()

	test.That(t, cam.Close(context.Background()), test.ShouldBeNil)
	_, closesAfter := fake.counts()
	test.That(t, closesAfter, test.ShouldEqual, closesBefore+1)
	test.That(t, d.Status(), test.ShouldEqual, driver.StateClosed)

	_, _, err = cam.Images(context.Background(), nil, nil)
	test.That(t, err, test.ShouldBeError, errClosed)
	_, err = cam.Properties(context.Background())
	test.That(t, err, test.ShouldBeError, errClosed)

	err = cam.Close(context.Background())
	test.That(t, err, test.ShouldWrap, errClosed)
}

func TestWebcamReadError(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fake := newFakeDriver(640, 480)
	registerFakeDriver(t, "fake-cam", fake)
	readErr := errors.New("sensor glitch")
	fake.setReadErr(readErr)

	cam, err := newTestWebcam(t, WebcamConfig{Path: "fake-cam"}, logger)
	test.That(t, err, test.ShouldBeNil)
	waitForImagesError(t, cam, readErr)

	fake.setReadErr(nil)
	waitForFrame(t, cam)
}

func TestWebcamResolutionMismatch(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)
	// Advertise 320x240 so the exact constraint is satisfied, but deliver 640x480 frames.
	fake := newFakeDriver(320, 240)
	fake.setFrameSize(640, 480)
	registerFakeDriver(t, "fake-cam", fake)

	cam, err := newTestWebcam(t, WebcamConfig{Path: "fake-cam", Width: 320, Height: 240}, logger)
	test.That(t, err, test.ShouldBeNil)

	img := waitForFrame(t, cam)
	bounds, err := img.Bounds()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bounds.Dx(), test.ShouldEqual, 640)
	test.That(t, bounds.Dy(), test.ShouldEqual, 480)

	// The warning is rate limited, so a second read must not log it again.
	waitForFrame(t, cam)
	test.That(t, logs.FilterMessageSnippet("do not match actual webcam resolution").Len(), test.ShouldEqual, 1)
}

func TestWebcamDisconnectReconnect(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fake := newFakeDriver(640, 480)
	registerFakeDriver(t, "fake-cam", fake)

	cam, err := newTestWebcam(t, WebcamConfig{Path: "fake-cam"}, logger)
	test.That(t, err, test.ShouldBeNil)
	waitForFrame(t, cam)
	opensBefore, closesBefore := fake.counts()

	fake.setAvailable(false)
	waitForImagesError(t, cam, errDisconnected)
	_, err = cam.Properties(context.Background())
	test.That(t, err, test.ShouldBeError, errDisconnected)

	fake.setAvailable(true)
	waitForFrame(t, cam)

	// Reconnecting closes the old handle, then probes properties (open+close) and opens for streaming.
	opensAfter, closesAfter := fake.counts()
	test.That(t, opensAfter, test.ShouldEqual, opensBefore+2)
	test.That(t, closesAfter, test.ShouldEqual, closesBefore+2)
}

func TestWebcamCloseWhileDisconnected(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fake := newFakeDriver(640, 480)
	registerFakeDriver(t, "fake-cam", fake)

	cam, err := newTestWebcam(t, WebcamConfig{Path: "fake-cam"}, logger)
	test.That(t, err, test.ShouldBeNil)
	waitForFrame(t, cam)

	fake.setAvailable(false)
	waitForImagesError(t, cam, errDisconnected)

	test.That(t, cam.Close(context.Background()), test.ShouldBeNil)
	_, _, err = cam.Images(context.Background(), nil, nil)
	test.That(t, err, test.ShouldBeError, errClosed)
}
