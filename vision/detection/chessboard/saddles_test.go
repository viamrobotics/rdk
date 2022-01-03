package chessboard

import (
	"math"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/rimage"
)

func TestComputeSaddleMap(t *testing.T) {
	imgPng, err := rimage.NewImageFromFile(artifact.MustPath("rimage/image_2021-07-16-15-59-18.png"))
	test.That(t, err, test.ShouldBeNil)
	bounds := imgPng.Bounds()
	dims := bounds.Max
	h, w := dims.Y, dims.X
	img := rimage.ConvertImage(imgPng)
	m := rimage.ConvertColorImageToLuminanceFloat(*img)

	saddle, saddlePoints, err := GetSaddleMapPoints(m, &DefaultSaddleConf)
	test.That(t, err, test.ShouldBeNil)
	h1, w1 := saddle.Dims()
	test.That(t, h, test.ShouldEqual, h1)
	test.That(t, w, test.ShouldEqual, w1)
	test.That(t, mat.Min(saddle), test.ShouldEqual, 0)                         // GT obtained from python prototype code
	test.That(t, mat.Max(saddle), test.ShouldBeGreaterThan, 1*math.Pow(10, 6)) // GT obtained from python prototype code
	test.That(t, len(saddlePoints), test.ShouldEqual, 116)                     // GT obtained from python prototype code
}

func TestNonMaxSuppression(t *testing.T) {
	// loading an image with 2 points (50,100) and (100,150) convolved with a gaussian
	imgPng, err := rimage.NewImageFromFile(artifact.MustPath("rimage/nms_test_50_100_100_150.png"))
	test.That(t, err, test.ShouldBeNil)
	bounds := imgPng.Bounds()
	dims := bounds.Max
	h, w := dims.Y, dims.X
	img := rimage.ConvertImage(imgPng)
	m := rimage.ConvertColorImageToLuminanceFloat(*img)

	nonMaxSuppressed := nonMaxSuppression(m, 10)
	h1, w1 := nonMaxSuppressed.Dims()
	test.That(t, h, test.ShouldEqual, h1)
	test.That(t, w, test.ShouldEqual, w1)
	// expecting (50, 100) to be non-zero and neighboring points to be 0
	test.That(t, nonMaxSuppressed.At(49, 99), test.ShouldEqual, 0)
	// test.That(t, nonMaxSuppressed.At(50, 100), test.ShouldEqual, 0)
	test.That(t, nonMaxSuppressed.At(51, 101), test.ShouldEqual, 0)
}
