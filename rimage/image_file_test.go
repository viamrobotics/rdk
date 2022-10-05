package rimage

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/utils"
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

func TestDecodeImage(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 8))
	img.Set(3, 3, Red)

	var buf bytes.Buffer
	test.That(t, png.Encode(&buf, img), test.ShouldBeNil)
}

func TestEncodeImage(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 8))
	img.Set(3, 3, Red)

	var buf bytes.Buffer
	test.That(t, png.Encode(&buf, img), test.ShouldBeNil)

	var bufJPEG bytes.Buffer
	test.That(t, jpeg.Encode(&bufJPEG, img, nil), test.ShouldBeNil)
}

func TestRawRGBAEncodingDecoding(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 8))
	img.Set(3, 3, Red)

	encodedImgBytes, err := EncodeImage(context.Background(), img, utils.MimeTypeRawRGBA)
	test.That(t, err, test.ShouldBeNil)

	decodedImg, err := DecodeImage(context.Background(), encodedImgBytes, utils.MimeTypeRawRGBA, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, decodedImg.Bounds(), test.ShouldResemble, img.Bounds())
	imgR, imgG, imgB, imgA := img.At(3, 3).RGBA()
	decodedImgR, decodedImgG, decodedImgB, decodedImgA := decodedImg.At(3, 3).RGBA()
	test.That(t, imgR, test.ShouldResemble, decodedImgR)
	test.That(t, imgG, test.ShouldResemble, decodedImgG)
	test.That(t, imgB, test.ShouldResemble, decodedImgB)
	test.That(t, imgA, test.ShouldResemble, decodedImgA)
}
