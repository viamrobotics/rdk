package rimage

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

func TestLazyEncodedImage(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 8))
	img.Set(3, 3, Red)

	var pngBuf bytes.Buffer
	test.That(t, png.Encode(&pngBuf, img), test.ShouldBeNil)
	var jpegBuf bytes.Buffer
	test.That(t, jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 100}), test.ShouldBeNil)
	jpegImg, err := jpeg.Decode(bytes.NewBuffer(jpegBuf.Bytes()))
	test.That(t, err, test.ShouldBeNil)

	imgLazy := NewLazyEncodedImage(pngBuf.Bytes(), utils.MimeTypePNG)
	test.That(t, imgLazy.(*LazyEncodedImage).DecodeAll(), test.ShouldBeNil)
	test.That(t, imgLazy.(*LazyEncodedImage).MIMEType(), test.ShouldEqual, utils.MimeTypePNG)
	test.That(t, NewColorFromColor(imgLazy.At(0, 0)), test.ShouldEqual, Black)
	test.That(t, NewColorFromColor(imgLazy.At(3, 3)), test.ShouldEqual, Red)
	test.That(t, imgLazy.Bounds(), test.ShouldResemble, img.Bounds())
	test.That(t, imgLazy.ColorModel(), test.ShouldResemble, img.ColorModel())

	img2, err := png.Decode(bytes.NewBuffer(imgLazy.(*LazyEncodedImage).RawData()))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img2, test.ShouldResemble, img)

	// a bad image though :(
	imgLazy = NewLazyEncodedImage([]byte{1, 2, 3}, utils.MimeTypePNG)

	test.That(t, imgLazy.(*LazyEncodedImage).MIMEType(), test.ShouldEqual, utils.MimeTypePNG)
	err = imgLazy.(*LazyEncodedImage).DecodeAll()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, image.ErrFormat)
	test.That(t, func() { imgLazy.Bounds() }, test.ShouldPanic)
	test.That(t, func() { imgLazy.ColorModel() }, test.ShouldPanicWith, image.ErrFormat)
	test.That(t, func() { NewColorFromColor(imgLazy.At(0, 0)) }, test.ShouldPanicWith, image.ErrFormat)
	test.That(t, func() { NewColorFromColor(imgLazy.At(4, 4)) }, test.ShouldPanicWith, image.ErrFormat)

	imgLazy = NewLazyEncodedImage([]byte{1, 2, 3}, "weeeee")

	test.That(t, imgLazy.(*LazyEncodedImage).MIMEType(), test.ShouldEqual, "weeeee")
	err = imgLazy.(*LazyEncodedImage).DecodeAll()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, image.ErrFormat)
	test.That(t, func() { imgLazy.Bounds() }, test.ShouldPanic)
	test.That(t, func() { imgLazy.ColorModel() }, test.ShouldPanicWith, image.ErrFormat)
	test.That(t, func() { NewColorFromColor(imgLazy.At(0, 0)) }, test.ShouldPanicWith, image.ErrFormat)
	test.That(t, func() { NewColorFromColor(imgLazy.At(4, 4)) }, test.ShouldPanicWith, image.ErrFormat)

	// png without a mime type
	imgLazy = NewLazyEncodedImage(pngBuf.Bytes(), "")
	test.That(t, imgLazy.(*LazyEncodedImage).MIMEType(), test.ShouldEqual, utils.MimeTypePNG)
	test.That(t, imgLazy.(*LazyEncodedImage).DecodeAll(), test.ShouldBeNil)
	test.That(t, NewColorFromColor(imgLazy.At(0, 0)), test.ShouldEqual, Black)
	test.That(t, NewColorFromColor(imgLazy.At(3, 3)), test.ShouldEqual, Red)
	test.That(t, imgLazy.Bounds(), test.ShouldResemble, img.Bounds())
	test.That(t, imgLazy.ColorModel(), test.ShouldResemble, img.ColorModel())

	img2, err = png.Decode(bytes.NewBuffer(imgLazy.(*LazyEncodedImage).RawData()))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img2, test.ShouldResemble, img)

	// jpeg without a mime type
	imgLazy = NewLazyEncodedImage(jpegBuf.Bytes(), "")
	test.That(t, imgLazy.(*LazyEncodedImage).MIMEType(), test.ShouldEqual, utils.MimeTypeJPEG)
	test.That(t, imgLazy.(*LazyEncodedImage).DecodeAll(), test.ShouldBeNil)
	test.That(t, imgLazy.Bounds(), test.ShouldResemble, jpegImg.Bounds())
	test.That(t, imgLazy.ColorModel(), test.ShouldResemble, jpegImg.ColorModel())

	img2, err = jpeg.Decode(bytes.NewBuffer(imgLazy.(*LazyEncodedImage).RawData()))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img2, test.ShouldResemble, jpegImg)
}

func TestLazyEncodedImageSafeUnsafe(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 8))
	img.Set(3, 3, Red)

	var pngBuf bytes.Buffer
	test.That(t, png.Encode(&pngBuf, img), test.ShouldBeNil)

	badImgLazy := NewLazyEncodedImage([]byte{1, 2, 3}, utils.MimeTypePNG)
	goodImgLazy := NewLazyEncodedImage(pngBuf.Bytes(), utils.MimeTypePNG)

	t.Run("BoundsSafe", func(t *testing.T) {
		t.Run("with bad image", func(t *testing.T) {
			bounds, err := badImgLazy.(*LazyEncodedImage).BoundsSafe()
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err, test.ShouldBeError, image.ErrFormat)
			test.That(t, bounds, test.ShouldResemble, image.Rectangle{})
		})

		t.Run("with good image", func(t *testing.T) {
			bounds, err := goodImgLazy.(*LazyEncodedImage).BoundsSafe()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, bounds, test.ShouldResemble, img.Bounds())
		})
	})

	t.Run("ColorModelSafe", func(t *testing.T) {
		t.Run("with bad image", func(t *testing.T) {
			colorModel, err := badImgLazy.(*LazyEncodedImage).ColorModelSafe()
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err, test.ShouldBeError, image.ErrFormat)
			test.That(t, colorModel, test.ShouldBeNil)
		})

		t.Run("with good image", func(t *testing.T) {
			colorModel, err := goodImgLazy.(*LazyEncodedImage).ColorModelSafe()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, colorModel, test.ShouldNotBeNil)
		})
	})

	t.Run("AtSafe", func(t *testing.T) {
		t.Run("with bad image", func(t *testing.T) {
			pixelColor, err := badImgLazy.(*LazyEncodedImage).AtSafe(0, 0)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err, test.ShouldBeError, image.ErrFormat)
			test.That(t, pixelColor, test.ShouldBeNil)
		})

		t.Run("with good image", func(t *testing.T) {
			pixelColor, err := goodImgLazy.(*LazyEncodedImage).AtSafe(0, 0)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pixelColor, test.ShouldNotBeNil)
		})
	})

	t.Run("DecodedImage", func(t *testing.T) {
		t.Run("with bad image", func(t *testing.T) {
			decodedImg, err := badImgLazy.(*LazyEncodedImage).DecodedImage()
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err, test.ShouldBeError, image.ErrFormat)
			test.That(t, decodedImg, test.ShouldBeNil)
		})

		t.Run("with good image", func(t *testing.T) {
			decodedImg, err := goodImgLazy.(*LazyEncodedImage).DecodedImage()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, decodedImg, test.ShouldNotBeNil)
		})
	})

	t.Run("Bounds", func(t *testing.T) {
		t.Run("with bad image", func(t *testing.T) {
			test.That(t, func() { badImgLazy.Bounds() }, test.ShouldPanic)
		})
		t.Run("with good image", func(t *testing.T) {
			test.That(t, func() { goodImgLazy.Bounds() }, test.ShouldNotPanic)
			test.That(t, goodImgLazy.Bounds(), test.ShouldResemble, img.Bounds())
		})
	})

	t.Run("ColorModel", func(t *testing.T) {
		t.Run("with bad image", func(t *testing.T) {
			test.That(t, func() { badImgLazy.ColorModel() }, test.ShouldPanicWith, image.ErrFormat)
		})
		t.Run("with good image", func(t *testing.T) {
			test.That(t, func() { goodImgLazy.ColorModel() }, test.ShouldNotPanic)
			test.That(t, goodImgLazy.ColorModel(), test.ShouldNotBeNil)
		})
	})

	t.Run("At", func(t *testing.T) {
		t.Run("with bad image", func(t *testing.T) {
			test.That(t, func() { badImgLazy.At(0, 0) }, test.ShouldPanicWith, image.ErrFormat)
			test.That(t, func() { badImgLazy.At(4, 4) }, test.ShouldPanicWith, image.ErrFormat)
		})
		t.Run("with good image", func(t *testing.T) {
			test.That(t, func() { goodImgLazy.At(0, 0) }, test.ShouldNotPanic)
			test.That(t, NewColorFromColor(goodImgLazy.At(3, 3)), test.ShouldEqual, Red)
		})
	})
}
