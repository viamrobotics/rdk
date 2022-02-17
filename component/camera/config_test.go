package camera

import (
	"testing"

	"go.viam.com/test"
)

func TestDetectColor(t *testing.T) {
	// empty string
	attrs := &AttrConfig{DetectColorString: ""}
	result, err := attrs.DetectColor()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldHaveLength, 0)
	// not a pound sign
	attrs = &AttrConfig{DetectColorString: "$121CFF"}
	_, err = attrs.DetectColor()
	test.That(t, err.Error(), test.ShouldContainSubstring, "detect_color is ill-formed")
	// string too long
	attrs = &AttrConfig{DetectColorString: "#121CFF03"}
	_, err = attrs.DetectColor()
	test.That(t, err.Error(), test.ShouldContainSubstring, "detect_color is ill-formed")
	// string too short
	attrs = &AttrConfig{DetectColorString: "#121C"}
	_, err = attrs.DetectColor()
	test.That(t, err.Error(), test.ShouldContainSubstring, "detect_color is ill-formed")
	// not a hex string
	attrs = &AttrConfig{DetectColorString: "#1244GG"}
	_, err = attrs.DetectColor()
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid byte")
	// success
	attrs = &AttrConfig{DetectColorString: "#1244FF"}
	result, err = attrs.DetectColor()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldHaveLength, 3)
	test.That(t, result[0], test.ShouldEqual, 18)
	test.That(t, result[1], test.ShouldEqual, 68)
	test.That(t, result[2], test.ShouldEqual, 255)
}
