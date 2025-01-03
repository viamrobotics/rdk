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
	test.That(t, func() { imgLazy.Bounds() }, test.ShouldPanic)
	test.That(t, func() { imgLazy.ColorModel() }, test.ShouldPanicWith, image.ErrFormat)
	test.That(t, func() { NewColorFromColor(imgLazy.At(0, 0)) }, test.ShouldPanicWith, image.ErrFormat)
	test.That(t, func() { NewColorFromColor(imgLazy.At(4, 4)) }, test.ShouldPanicWith, image.ErrFormat)

	imgLazy = NewLazyEncodedImage([]byte{1, 2, 3}, "weeeee")

	test.That(t, imgLazy.(*LazyEncodedImage).MIMEType(), test.ShouldEqual, "weeeee")
	test.That(t, func() { imgLazy.Bounds() }, test.ShouldPanic)
	test.That(t, func() { imgLazy.ColorModel() }, test.ShouldPanic)
	test.That(t, func() { NewColorFromColor(imgLazy.At(0, 0)) }, test.ShouldPanic)
	test.That(t, func() { NewColorFromColor(imgLazy.At(4, 4)) }, test.ShouldPanic)

	// png without a mime type
	imgLazy = NewLazyEncodedImage(pngBuf.Bytes(), "")
	test.That(t, imgLazy.(*LazyEncodedImage).MIMEType(), test.ShouldEqual, utils.MimeTypePNG)
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
	test.That(t, imgLazy.Bounds(), test.ShouldResemble, jpegImg.Bounds())
	test.That(t, imgLazy.ColorModel(), test.ShouldResemble, jpegImg.ColorModel())

	img2, err = jpeg.Decode(bytes.NewBuffer(imgLazy.(*LazyEncodedImage).RawData()))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img2, test.ShouldResemble, jpegImg)
}
