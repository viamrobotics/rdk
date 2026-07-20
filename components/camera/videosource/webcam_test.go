package videosource_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/camera/videosource"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

// TestWebcamConfigDeviceID ensures the discovery-surfaced device_id attribute
// decodes into the native WebcamConfig instead of being silently dropped.
func TestWebcamConfigDeviceID(t *testing.T) {
	attrs := utils.AttributeMap{
		"device_id":  "usb-046d_webcam-video-index0",
		"video_path": "video4",
		"format":     "MJPEG",
		"width_px":   1920,
		"height_px":  1080,
	}

	native, err := resource.TransformAttributeMap[*videosource.WebcamConfig](attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, native.DeviceID, test.ShouldEqual, "usb-046d_webcam-video-index0")
	test.That(t, native.Path, test.ShouldEqual, "video4")
	test.That(t, native.Format, test.ShouldEqual, "MJPEG")
	test.That(t, native.Width, test.ShouldEqual, 1920)
}

func TestWebcamValidation(t *testing.T) {
	webCfg := &videosource.WebcamConfig{
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
