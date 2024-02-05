package rtsp

import (
	"testing"

	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/rimage/transform"
)

func TestRTSPConfig(t *testing.T) {
	// success
	rtspConf := &Config{Address: "rtsp://example.com:5000"}
	_, err := rtspConf.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	// badly formatted rtsp address
	rtspConf = &Config{Address: "http://example.com"}
	_, err = rtspConf.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unsupported scheme")
	// bad intrinsic parameters
	rtspConf = &Config{
		Address:         "rtsp://example.com:5000",
		IntrinsicParams: &transform.PinholeCameraIntrinsics{},
	}
	_, err = rtspConf.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)
	// good intrinsic parameters
	rtspConf = &Config{
		Address:         "rtsp://example.com:5000",
		IntrinsicParams: &transform.PinholeCameraIntrinsics{1, 2, 3, 4, 5, 6},
	}
	_, err = rtspConf.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	// no distortion parameters is OK
	rtspConf.DistortionParams = &transform.BrownConrady{}
	test.That(t, err, test.ShouldBeNil)
}
