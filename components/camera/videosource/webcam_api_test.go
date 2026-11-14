package videosource

import (
	"context"
	"errors"
	"image/jpeg"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

const resMismatchSnippet = "do not match actual webcam resolution"

func TestWebcamImages(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the buffered frame as a jpeg named image", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		c := startFakeWebcam(t, WebcamConfig{FrameRate: 100}, logger, &fakeReader{}, &fakeDriver{}, &fakeOpener{})
		waitFrame(t, c)

		imgs, meta, err := c.Images(ctx, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imgs, test.ShouldHaveLength, 1)
		test.That(t, imgs[0].SourceName, test.ShouldEqual, "cam")
		test.That(t, imgs[0].MimeType(), test.ShouldEqual, utils.MimeTypeJPEG)
		test.That(t, meta.CapturedAt.IsZero(), test.ShouldBeFalse)
		test.That(t, logs.FilterMessageSnippet(resMismatchSnippet).Len(), test.ShouldEqual, 0)
	})

	for _, tc := range []struct {
		name  string
		err   error
		msg   string
		level zapcore.Level
	}{
		{"read error is returned and logged at error", errReadFailed, "error reading frame", zapcore.ErrorLevel},
		{"corrupt jpeg is returned and logged at debug", jpeg.FormatError("short Huffman data"), "dropped corrupt frame", zapcore.DebugLevel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger, logs := logging.NewObservedTestLogger(t)
			reader := &fakeReader{}
			c := startFakeWebcam(t, WebcamConfig{FrameRate: 100}, logger, reader, &fakeDriver{}, &fakeOpener{})
			waitFrame(t, c)

			reader.failWith(tc.err)
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				_, _, err := c.Images(ctx, nil, nil)
				test.That(tb, errors.Is(err, tc.err), test.ShouldBeTrue)
			})
			entries := logs.FilterMessageSnippet(tc.msg)
			test.That(t, entries.Len(), test.ShouldBeGreaterThan, 0)
			test.That(t, entries.FilterLevelExact(tc.level).Len(), test.ShouldEqual, entries.Len())

			reader.failWith(nil)
			waitFrame(t, c)
			test.That(t, c.Close(ctx), test.ShouldBeNil)
			assertAllReleased(t, reader)
		})
	}

	t.Run("warns once when resolution does not match config", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		conf := WebcamConfig{FrameRate: 100, Width: 640, Height: 480}
		c := startFakeWebcam(t, conf, logger, &fakeReader{}, &fakeDriver{}, &fakeOpener{})
		waitFrame(t, c)

		_, _, err := c.Images(ctx, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, logs.FilterMessageSnippet(resMismatchSnippet).Len(), test.ShouldEqual, 1)
	})
}

func TestWebcamProperties(t *testing.T) {
	ctx := context.Background()

	t.Run("defaults", func(t *testing.T) {
		c := startFakeWebcam(t, WebcamConfig{}, logging.NewTestLogger(t), &fakeReader{}, &fakeDriver{}, &fakeOpener{})

		props, err := c.Properties(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, props.SupportsPCD, test.ShouldBeFalse)
		test.That(t, props.ImageType, test.ShouldEqual, camera.ColorStream)
		test.That(t, props.FrameRate, test.ShouldEqual, defaultFrameRate)
		test.That(t, props.IntrinsicParams, test.ShouldBeNil)
		test.That(t, props.DistortionParams, test.ShouldBeNil)
		test.That(t, props.MimeTypes, test.ShouldResemble, []string{utils.MimeTypeJPEG, utils.MimeTypePNG, utils.MimeTypeRawRGBA})
	})

	t.Run("from config", func(t *testing.T) {
		intrinsics := &transform.PinholeCameraIntrinsics{Width: 640, Height: 480, Fx: 500, Fy: 500, Ppx: 320, Ppy: 240}
		distortion := &transform.BrownConrady{RadialK1: 0.1}
		conf := WebcamConfig{FrameRate: 15, CameraParameters: intrinsics, DistortionParameters: distortion}
		c := startFakeWebcam(t, conf, logging.NewTestLogger(t), &fakeReader{}, &fakeDriver{}, &fakeOpener{})

		props, err := c.Properties(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, props.FrameRate, test.ShouldEqual, float32(15))
		test.That(t, props.IntrinsicParams, test.ShouldResemble, intrinsics)
		test.That(t, props.DistortionParams, test.ShouldResemble, distortion)
	})
}

func TestWebcamClose(t *testing.T) {
	ctx := context.Background()

	t.Run("closes the driver once", func(t *testing.T) {
		driver := &fakeDriver{}
		c := newFakeWebcam(t, 0, &fakeReader{}, driver, &fakeOpener{})

		test.That(t, c.Close(ctx), test.ShouldBeNil)
		err := c.Close(ctx)
		test.That(t, errors.Is(err, errClosed), test.ShouldBeTrue)
		test.That(t, driver.closes.Load(), test.ShouldEqual, int32(1))

		_, _, err = c.Images(ctx, nil, nil)
		test.That(t, errors.Is(err, errClosed), test.ShouldBeTrue)
		_, err = c.Properties(ctx)
		test.That(t, errors.Is(err, errClosed), test.ShouldBeTrue)
	})

	t.Run("returns driver close error", func(t *testing.T) {
		c := newFakeWebcam(t, 0, &fakeReader{}, &fakeDriver{closeErr: errCloseFailed}, &fakeOpener{})

		err := c.Close(ctx)
		test.That(t, errors.Is(err, errCloseFailed), test.ShouldBeTrue)
		_, _, err = c.Images(ctx, nil, nil)
		test.That(t, errors.Is(err, errClosed), test.ShouldBeTrue)
	})

	t.Run("releases callers waiting on a wake", func(t *testing.T) {
		// Workers are never started, so the wake is never served and only Close can release the caller.
		c := newWebcam(resource.NewName(camera.API, "cam"), WebcamConfig{FrameRate: 100}, "fake",
			&fakeReader{}, &fakeDriver{}, (&fakeOpener{}).open, logging.NewTestLogger(t))
		c.idleState = stateIdle
		c.readyCh = make(chan struct{})
		c.wakeTimeout = time.Second

		errCh := make(chan error, 1)
		go func() {
			_, _, err := c.Images(ctx, nil, nil)
			errCh <- err
		}()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, len(c.wakeCh), test.ShouldEqual, 1)
		})

		test.That(t, c.Close(ctx), test.ShouldBeNil)
		test.That(t, errors.Is(<-errCh, errClosed), test.ShouldBeTrue)
	})
}
