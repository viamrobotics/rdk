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

	t.Run("lazy", func(t *testing.T) {
		decoded, err := DecodeImage(context.Background(), buf.Bytes(), utils.WithLazyMIMEType(utils.MimeTypePNG), 4, 8)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, decoded, test.ShouldHaveSameTypeAs, &LazyEncodedImage{})
		decodedLazy := decoded.(*LazyEncodedImage)
		test.That(t, decodedLazy.RawData(), test.ShouldResemble, buf.Bytes())
		test.That(t, decodedLazy.Bounds(), test.ShouldResemble, img.Bounds())
	})
}

func TestEncodeImage(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 8))
	img.Set(3, 3, Red)

	var buf bytes.Buffer
	test.That(t, png.Encode(&buf, img), test.ShouldBeNil)

	var bufJPEG bytes.Buffer
	test.That(t, jpeg.Encode(&bufJPEG, img, nil), test.ShouldBeNil)

	t.Run("lazy", func(t *testing.T) {
		// fast
		lazyImg := NewLazyEncodedImage(buf.Bytes(), "hehe", 4, 8)
		encoded, err := EncodeImage(context.Background(), lazyImg, "hehe")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, encoded, test.ShouldResemble, buf.Bytes())

		lazyImg = NewLazyEncodedImage(buf.Bytes(), "hehe", 4, 8)
		encoded, err = EncodeImage(context.Background(), lazyImg, utils.WithLazyMIMEType("hehe"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, encoded, test.ShouldResemble, buf.Bytes())

		// slower
		lazyImg = NewLazyEncodedImage(buf.Bytes(), utils.MimeTypePNG, 4, 8)
		encoded, err = EncodeImage(context.Background(), lazyImg, utils.MimeTypeJPEG)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, encoded, test.ShouldResemble, bufJPEG.Bytes())

		lazyImg = NewLazyEncodedImage(buf.Bytes(), utils.MimeTypePNG, 4, 8)
		encoded, err = EncodeImage(context.Background(), lazyImg, utils.WithLazyMIMEType(utils.MimeTypeJPEG))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, encoded, test.ShouldResemble, bufJPEG.Bytes())
	})
}
