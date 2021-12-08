package rimage

import (
	"testing"

	"github.com/golang/geo/r2"
	"github.com/stretchr/testify/assert"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"gonum.org/v1/gonum/mat"
)

func TestFindContours(t *testing.T) {
	img, err := readImageFromFile(artifact.MustPath("rimage/binary_image.jpg"), false)
	test.That(t, err, test.ShouldBeNil)
	bounds := img.Bounds()
	dims := bounds.Max
	nRows, nCols := dims.Y, dims.X
	binary := mat.NewDense(nRows, nCols, nil)
	for r := 0; r < nRows; r++ {
		for c := 0; c < nCols; c++ {
			color, _, _, _ := img.At(c, r).RGBA()
			val := float64(color) / 255.
			outVal := 0.
			if val > 127 {
				outVal = 1.
			}

			binary.Set(r, c, outVal)

		}
	}
	contours, hierarchy := FindContours(binary)

	// Test hierarchy values
	test.That(t, len(hierarchy), test.ShouldEqual, 5) // number of contours + root
	test.That(t, hierarchy[0].FirstChild, test.ShouldEqual, 4)
	test.That(t, hierarchy[1].FirstChild, test.ShouldEqual, 3)
	test.That(t, hierarchy[2].FirstChild, test.ShouldEqual, -1)
	test.That(t, hierarchy[3].FirstChild, test.ShouldEqual, 5)
	test.That(t, hierarchy[4].FirstChild, test.ShouldEqual, -1)

	// Test contours length and numbers
	test.That(t, len(contours), test.ShouldEqual, 4)
	test.That(t, len(contours[0]), test.ShouldEqual, 800)
	test.That(t, len(contours[1]), test.ShouldEqual, 404)
	test.That(t, len(contours[2]), test.ShouldEqual, 564)
	test.That(t, len(contours[3]), test.ShouldEqual, 396)

	// Test hierarchy values
	test.That(t, len(hierarchy), test.ShouldEqual, 5) // number of contours + root
	test.That(t, hierarchy[0].FirstChild, test.ShouldEqual, 4)
	test.That(t, hierarchy[1].FirstChild, test.ShouldEqual, 3)
	test.That(t, hierarchy[2].FirstChild, test.ShouldEqual, -1)
	test.That(t, hierarchy[3].FirstChild, test.ShouldEqual, 5)
	test.That(t, hierarchy[4].FirstChild, test.ShouldEqual, -1)

	// Test contours length and numbers
	test.That(t, len(contours), test.ShouldEqual, 4)
	test.That(t, len(contours[0]), test.ShouldEqual, 800)
	test.That(t, len(contours[1]), test.ShouldEqual, 404)
	test.That(t, len(contours[2]), test.ShouldEqual, 564)
	test.That(t, len(contours[3]), test.ShouldEqual, 396)

}

func TestApproxContourDP(t *testing.T) {
	c1 := make([]r2.Point, 3)
	// half a 50x50 square contour
	c1[0] = r2.Point{50, 50}   //nolint:govet
	c1[1] = r2.Point{100, 50}  //nolint:govet
	c1[2] = r2.Point{100, 100} //nolint:govet

	// small epsilon: c1 and its approximation should be equal
	c1Approx1 := ApproxContourDP(c1, 0.5)
	test.That(t, c1[0].X, test.ShouldEqual, c1Approx1[0].X)
	test.That(t, c1[0].Y, test.ShouldEqual, c1Approx1[0].Y)
	test.That(t, c1[1].X, test.ShouldEqual, c1Approx1[1].X)
	test.That(t, c1[1].Y, test.ShouldEqual, c1Approx1[1].Y)
	test.That(t, c1[2].X, test.ShouldEqual, c1Approx1[2].X)
	test.That(t, c1[2].Y, test.ShouldEqual, c1Approx1[2].Y)

	// epsilon larger than square diagonal: approximation should be equal to diagonal
	c1Approx2 := ApproxContourDP(c1, 71)
	assert.Equal(t, len(c1Approx2), 2)
	test.That(t, c1[0].X, test.ShouldEqual, c1Approx2[0].X)
	test.That(t, c1[0].Y, test.ShouldEqual, c1Approx2[0].Y)
	test.That(t, c1[2].X, test.ShouldEqual, c1Approx2[1].X)
	test.That(t, c1[2].Y, test.ShouldEqual, c1Approx2[1].Y)
}
