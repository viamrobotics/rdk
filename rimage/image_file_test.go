package rimage

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"testing"

	libjpeg "github.com/viam-labs/go-libjpeg/jpeg"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/utils"
)

func TestPngEncodings(t *testing.T) {
	t.Parallel()
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
		decoded, err := DecodeImage(context.Background(), buf.Bytes(), utils.WithLazyMIMEType(utils.MimeTypePNG))
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
	test.That(t, libjpeg.Encode(&bufJPEG, img, jpegEncoderOptions), test.ShouldBeNil)

	t.Run("lazy", func(t *testing.T) {
		// fast
		lazyImg := NewLazyEncodedImage(buf.Bytes(), "hehe")
		encoded, err := EncodeImage(context.Background(), lazyImg, "hehe")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, encoded, test.ShouldResemble, buf.Bytes())

		lazyImg = NewLazyEncodedImage(buf.Bytes(), "hehe")
		encoded, err = EncodeImage(context.Background(), lazyImg, utils.WithLazyMIMEType("hehe"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, encoded, test.ShouldResemble, buf.Bytes())
	})
	t.Run("jpeg from png", func(t *testing.T) {
		lazyImg := NewLazyEncodedImage(buf.Bytes(), utils.MimeTypePNG)
		encoded, err := EncodeImage(context.Background(), lazyImg, utils.MimeTypeJPEG)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, encoded, test.ShouldResemble, bufJPEG.Bytes())
	})
}

func TestRawRGBAEncodingDecoding(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 8))
	img.Set(3, 3, Red)

	encodedImgBytes, err := EncodeImage(context.Background(), img, utils.MimeTypeRawRGBA)
	test.That(t, err, test.ShouldBeNil)
	reader := bytes.NewReader(encodedImgBytes)

	conf, header, err := image.DecodeConfig(reader)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, header, test.ShouldEqual, "vnd.viam.rgba")
	test.That(t, conf.Width, test.ShouldEqual, img.Bounds().Dx())
	test.That(t, conf.Height, test.ShouldEqual, img.Bounds().Dy())

	// decode with image package
	reader = bytes.NewReader(encodedImgBytes)
	imgDecoded, header, err := image.Decode(reader)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, header, test.ShouldEqual, "vnd.viam.rgba")
	test.That(t, imgDecoded.Bounds(), test.ShouldResemble, img.Bounds())
	test.That(t, imgDecoded.At(3, 6), test.ShouldResemble, img.At(3, 6))
	test.That(t, imgDecoded.At(1, 3), test.ShouldResemble, img.At(1, 3))

	decodedImg, err := DecodeImage(context.Background(), encodedImgBytes, utils.MimeTypeRawRGBA)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, decodedImg.Bounds(), test.ShouldResemble, img.Bounds())
	imgR, imgG, imgB, imgA := img.At(3, 3).RGBA()
	decodedImgR, decodedImgG, decodedImgB, decodedImgA := decodedImg.At(3, 3).RGBA()
	test.That(t, imgR, test.ShouldResemble, decodedImgR)
	test.That(t, imgG, test.ShouldResemble, decodedImgG)
	test.That(t, imgB, test.ShouldResemble, decodedImgB)
	test.That(t, imgA, test.ShouldResemble, decodedImgA)
}

func TestRawDepthEncodingDecoding(t *testing.T) {
	img := NewEmptyDepthMap(4, 8)
	for x := 0; x < 4; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, Depth(x*y))
		}
	}
	encodedImgBytes, err := EncodeImage(context.Background(), img, utils.MimeTypeRawDepth)
	test.That(t, err, test.ShouldBeNil)
	reader := bytes.NewReader(encodedImgBytes)

	// decode Header
	conf, header, err := image.DecodeConfig(reader)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, header, test.ShouldEqual, "vnd.viam.dep")
	test.That(t, conf.Width, test.ShouldEqual, img.Bounds().Dx())
	test.That(t, conf.Height, test.ShouldEqual, img.Bounds().Dy())

	// decode with image package
	reader = bytes.NewReader(encodedImgBytes)
	imgDecoded, header, err := image.Decode(reader)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, header, test.ShouldEqual, "vnd.viam.dep")
	test.That(t, imgDecoded.Bounds(), test.ShouldResemble, img.Bounds())
	test.That(t, imgDecoded.At(2, 3), test.ShouldResemble, img.At(2, 3))
	test.That(t, imgDecoded.At(1, 0), test.ShouldResemble, img.At(1, 0))

	// decode with rimage package
	decodedImg, err := DecodeImage(context.Background(), encodedImgBytes, utils.MimeTypeRawDepth)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, decodedImg.Bounds(), test.ShouldResemble, img.Bounds())
	decodedDm, ok := decodedImg.(*DepthMap)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, decodedDm.GetDepth(2, 3), test.ShouldEqual, img.GetDepth(2, 3))
	test.That(t, decodedDm.GetDepth(1, 0), test.ShouldEqual, img.GetDepth(1, 0))
}
