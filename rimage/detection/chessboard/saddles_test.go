package chessboard

import (
	"go.viam.com/core/rimage"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
	"gonum.org/v1/gonum/mat"
	"image"
	"math"
	"os"
	"strings"
	"testing"
)

// readImageFromFile extracts the RGB, Z16, or "both" data from an image file.
// Aligned matters if you are reading a .both.gz file and both the rgb and d image are already aligned.
// Otherwise, if you are just reading an image, aligned is a moot parameter and should be false.
func readImageFromFile(path string, aligned bool) (image.Image, error) {
	if strings.HasSuffix(path, ".both.gz") {
		return rimage.ReadBothFromFile(path, aligned)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(f.Close)

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func TestComputeSaddleMap(t *testing.T) {
	imgPng, err :=readImageFromFile(artifact.MustPath("rimage/image_2021-07-16-15-59-18.png"), false)
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
	test.That(t, mat.Min(saddle), test.ShouldEqual, 0)  // GT obtained from python prototype code
	test.That(t, mat.Max(saddle), test.ShouldBeGreaterThan, 1*math.Pow(10, 6))  // GT obtained from python prototype code
	test.That(t, len(saddlePoints), test.ShouldEqual, 394)  // GT obtained from python prototype code

}

func TestNonMaxSuppression(t *testing.T) {
	// loading an image with 2 points (50,100) and (100,150) convolved with a gaussian
	imgPng, err :=readImageFromFile(artifact.MustPath("rimage/nms_test_50_100_100_150.png"), false)
	test.That(t, err, test.ShouldBeNil)
	bounds := imgPng.Bounds()
	dims := bounds.Max
	h, w := dims.Y, dims.X
	img := rimage.ConvertImage(imgPng)
	m := rimage.ConvertColorImageToLuminanceFloat(*img)

	nonMaxSuppressed := NonMaxSuppression(m, 10)
	h1, w1 := nonMaxSuppressed.Dims()
	test.That(t, h, test.ShouldEqual, h1)
	test.That(t, w, test.ShouldEqual, w1)
	// expecting (50, 100) to b e non-zero and neighboring points to be 0
	test.That(t, nonMaxSuppressed.At(49, 99), test.ShouldEqual, 0)
	test.That(t, nonMaxSuppressed.At(50, 100), test.ShouldEqual, 40)
	test.That(t, nonMaxSuppressed.At(51, 101), test.ShouldEqual, 0)

}
