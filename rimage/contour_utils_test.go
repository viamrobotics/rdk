package rimage

import (
	"testing"

	"github.com/golang/geo/r2"
	"github.com/stretchr/testify/assert"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"gonum.org/v1/gonum/mat"
)

func TestBorderType(t *testing.T) {
	test.That(t, Hole, test.ShouldEqual, 1)
	test.That(t, Outer, test.ShouldEqual, 2)
}

func TestCreateHoleBorder(t *testing.T) {
	border := CreateHoleBorder()
	test.That(t, border.borderType, test.ShouldEqual, Hole)
	test.That(t, border.borderType, test.ShouldEqual, Outer)
	assert.Equal(t, border.borderType, Outer)
}

func TestPointMat(t *testing.T) {
	p := PointMat{
		Row: 0,
		Col: 0,
	}
	assert.Equal(t, p.Row, 0)
	assert.Equal(t, p.Col, 0)
	p.Set(1, 2)
	assert.Equal(t, p.Row, 1)
	assert.Equal(t, p.Col, 2)
	q := PointMat{
		Row: 1,
		Col: 2,
	}
	assert.True(t, p.SamePoint(&q))
	out := isPointOutOfBounds(&p, 2, 2)
	assert.True(t, out)
	out2 := isPointOutOfBounds(&p, 3, 3)
	assert.False(t, out2)
}

func TestNode(t *testing.T) {
	node := Node{
		parent:      0,
		firstChild:  0,
		nextSibling: 0,
		border:      Border{},
	}
	assert.Equal(t, node.parent, 0)
	assert.Equal(t, node.firstChild, 0)
	assert.Equal(t, node.nextSibling, 0)
	assert.Equal(t, node.border.borderType, 0)
	node.reset()
	assert.Equal(t, node.parent, -1)
	assert.Equal(t, node.firstChild, -1)
	assert.Equal(t, node.nextSibling, -1)
}

func TestMarkAsExamined(t *testing.T) {
	center := PointMat{1, 1}
	mark0 := PointMat{1, 2}
	mark1 := PointMat{2, 1}
	mark2 := PointMat{1, 0}
	mark3 := PointMat{0, 1}
	checked := make([]bool, 4)
	assert.False(t, checked[0])
	markExamined(mark0, center, checked)
	assert.True(t, checked[0])
	assert.True(t, isExamined(checked))
	assert.False(t, checked[1])
	markExamined(mark1, center, checked)
	assert.True(t, checked[1])
	assert.False(t, checked[2])
	markExamined(mark2, center, checked)
	assert.True(t, checked[2])
	assert.False(t, checked[3])
	markExamined(mark3, center, checked)
	assert.True(t, checked[3])
}

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
