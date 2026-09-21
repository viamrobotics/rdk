package videosource

import (
	"errors"
	"testing"

	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestLabelFilter(t *testing.T) {
	d := registerFakeDriver(t, "video0;My Cam", newFakeDriver(640, 480))

	test.That(t, labelFilter("video0;My Cam", false)(d), test.ShouldBeTrue)
	test.That(t, labelFilter("video0", false)(d), test.ShouldBeFalse)
	test.That(t, labelFilter("My Cam", false)(d), test.ShouldBeFalse)

	test.That(t, labelFilter("video0", true)(d), test.ShouldBeTrue)
	test.That(t, labelFilter("My Cam", true)(d), test.ShouldBeTrue)
	test.That(t, labelFilter("video1", true)(d), test.ShouldBeFalse)
}

func TestMakeConstraints(t *testing.T) {
	logger := logging.NewTestLogger(t)

	videoConstraints := func(conf *WebcamConfig) mediadevices.MediaTrackConstraints {
		var vc mediadevices.MediaTrackConstraints
		makeConstraints(conf, logger).Video(&vc)
		return vc
	}

	t.Run("defaults are ranged", func(t *testing.T) {
		vc := videoConstraints(&WebcamConfig{})
		test.That(t, vc.Width, test.ShouldResemble, prop.IntRanged{Min: minResolutionDimension, Ideal: 640, Max: 4096})
		test.That(t, vc.Height, test.ShouldResemble, prop.IntRanged{Min: minResolutionDimension, Ideal: 480, Max: 2160})
		test.That(t, vc.FrameRate, test.ShouldResemble, prop.FloatRanged{Min: 0.0, Ideal: 30.0, Max: 140.0})
		test.That(t, vc.FrameFormat, test.ShouldHaveSameTypeAs, prop.FrameFormatOneOf{})
	})

	t.Run("explicit values are exact", func(t *testing.T) {
		vc := videoConstraints(&WebcamConfig{Width: 1280, Height: 720, FrameRate: 15, Format: string(frame.FormatMJPEG)})
		test.That(t, vc.Width, test.ShouldResemble, prop.IntExact(1280))
		test.That(t, vc.Height, test.ShouldResemble, prop.IntExact(720))
		test.That(t, vc.FrameRate, test.ShouldResemble, prop.FloatExact(15))
		test.That(t, vc.FrameFormat, test.ShouldResemble, prop.FrameFormatExact(frame.FormatMJPEG))
	})
}

func TestGetReaderAndDriver(t *testing.T) {
	logger := logging.NewTestLogger(t)
	loRes := newFakeDriver(640, 480)
	hiRes := newFakeDriver(1280, 720)
	registerFakeDriver(t, "fake-lo", loRes)
	hiResDriver := registerFakeDriver(t, "fake-hi", hiRes)

	t.Run("selects the driver matching exact resolution", func(t *testing.T) {
		constraints := makeConstraints(&WebcamConfig{Width: 1280, Height: 720}, logger)
		reader, d, err := getReaderAndDriver("fake-hi", constraints, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() { test.That(t, d.Close(), test.ShouldBeNil) }()
		test.That(t, d.ID(), test.ShouldEqual, hiResDriver.ID())

		img, release, err := reader.Read()
		test.That(t, err, test.ShouldBeNil)
		defer release()
		test.That(t, img.Bounds().Dx(), test.ShouldEqual, 1280)
		test.That(t, img.Bounds().Dy(), test.ShouldEqual, 720)
	})

	t.Run("unknown label", func(t *testing.T) {
		constraints := makeConstraints(&WebcamConfig{}, logger)
		_, _, err := getReaderAndDriver("does-not-exist", constraints, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no queryable drivers for video path: 'does-not-exist'")
	})

	t.Run("unsatisfiable constraints", func(t *testing.T) {
		constraints := makeConstraints(&WebcamConfig{Width: 999, Height: 999}, logger)
		_, _, err := getReaderAndDriver("fake-lo", constraints, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed to find a queryable driver that matches the config constraints")
		test.That(t, err.Error(), test.ShouldContainSubstring, "fake-lo")
	})

	t.Run("driver that fails to open is skipped", func(t *testing.T) {
		loRes.setOpenErr(errors.New("device busy"))
		defer loRes.setOpenErr(nil)

		constraints := makeConstraints(&WebcamConfig{}, logger)
		_, _, err := getReaderAndDriver("fake-lo", constraints, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no queryable drivers for video path: 'fake-lo'")
	})

	t.Run("driver already opened elsewhere is skipped", func(t *testing.T) {
		test.That(t, hiResDriver.Open(), test.ShouldBeNil)
		defer func() { test.That(t, hiResDriver.Close(), test.ShouldBeNil) }()

		constraints := makeConstraints(&WebcamConfig{}, logger)
		_, _, err := getReaderAndDriver("fake-hi", constraints, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no queryable drivers for video path: 'fake-hi'")
	})
}

func TestFindReaderAndDriver(t *testing.T) {
	isolateDriverRegistry(t)
	logger := logging.NewTestLogger(t)
	fake := registerFakeDriver(t, "fake-cam;Fake Cam", newFakeDriver(640, 480))

	t.Run("by path", func(t *testing.T) {
		_, d, path, err := findReaderAndDriver(&WebcamConfig{}, "fake-cam", logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() { test.That(t, d.Close(), test.ShouldBeNil) }()
		test.That(t, d.ID(), test.ShouldEqual, fake.ID())
		test.That(t, path, test.ShouldEqual, "fake-cam")
	})

	t.Run("by name", func(t *testing.T) {
		_, d, path, err := findReaderAndDriver(&WebcamConfig{}, "Fake Cam", logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() { test.That(t, d.Close(), test.ShouldBeNil) }()
		test.That(t, d.ID(), test.ShouldEqual, fake.ID())
		test.That(t, path, test.ShouldEqual, "Fake Cam")
	})

	t.Run("unknown path", func(t *testing.T) {
		_, _, _, err := findReaderAndDriver(&WebcamConfig{}, "nope", logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no queryable drivers for video path: 'nope'")
	})

	t.Run("any path resolves to first label segment", func(t *testing.T) {
		skipUnlessOnlyFakesRegistered(t, fake)
		_, d, path, err := findReaderAndDriver(&WebcamConfig{}, "", logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() { test.That(t, d.Close(), test.ShouldBeNil) }()
		test.That(t, d.ID(), test.ShouldEqual, fake.ID())
		test.That(t, path, test.ShouldEqual, "fake-cam")
	})
}
