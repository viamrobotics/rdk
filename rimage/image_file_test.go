package rimage

import (
	"bytes"
	"image/png"
	"os"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestPngEncodings(t *testing.T) {
	openBytes, err := os.ReadFile(artifact.MustPath("rimage/opencv_encoded_image.png"))
	test.That(t, err, test.ShouldBeNil)
	openGray16, err := png.Decode(bytes.NewReader(openBytes))
	test.That(t, err, test.ShouldBeNil)
	goBytes, err := os.ReadFile(artifact.MustPath("rimage/go_encoded_image.png"))
	test.That(t, err, test.ShouldBeNil)
	goGray16, err := png.Decode(bytes.NewReader(goBytes))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, openBytes, test.ShouldNotResemble, goBytes) // go and openCV encode PNGs differently
	test.That(t, openGray16, test.ShouldResemble, goGray16)  // but they decode to the same image
	var goEncodedOpenCVBytes bytes.Buffer
	err = png.Encode(&goEncodedOpenCVBytes, openGray16)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, goEncodedOpenCVBytes.Bytes(), test.ShouldResemble, goBytes)
}
