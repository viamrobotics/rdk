package rimage

import (
	"fmt"
	"testing"

	"github.com/golang/geo/r2"
	"github.com/stretchr/testify/assert"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"gonum.org/v1/gonum/mat"
)

func TestBorderType(t *testing.T) {
	assert.Equal(t, Hole, 1)
	assert.Equal(t, Outer, 2)
}

func TestCreateHoleBorder(t *testing.T) {
	border := CreateHoleBorder()
	assert.Equal(t, border.borderType, Hole)
}

func TestCreateOuterBorder(t *testing.T) {
	border := CreateOuterBorder()
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

	assert.Equal(t, len(hierarchy), 5)
	assert.Equal(t, hierarchy[0].parent, -1)
	assert.Equal(t, hierarchy[1].parent, 1)
	assert.Equal(t, hierarchy[2].parent, 2)
	assert.Equal(t, hierarchy[1].parent, 1)
	assert.Equal(t, hierarchy[4].parent, 4)

	assert.Equal(t, len(contours), 4)
	assert.Equal(t, len(contours[0]), 800)
	assert.Equal(t, len(contours[1]), 408)
	assert.Equal(t, len(contours[2]), 800)
	assert.Equal(t, len(contours[3]), 568)
}

func TestApproxContourDP(t *testing.T) {
	c1 := make([]r2.Point, 3)
	// half a 50x50 square contour
	c1[0] = r2.Point{50, 50}   //nolint:govet
	c1[1] = r2.Point{100, 50}  //nolint:govet
	c1[2] = r2.Point{100, 100} //nolint:govet
	//var approxContourTests = []struct {
	//	contour        []r2.Point // input
	//	expected       []r2.Point // expected result
	//}{
	//	{1, 1},
	//	{2, 1},
	//	{3, 2},
	//	{4, 3},
	//	{5, 5},
	//	{6, 8},
	//	{7, 13},
	//}

	// small epsilon: c1 and its approximation should be equal
	c1Approx1 := ApproxContourDP(c1, 0.5)
	assert.Equal(t, c1[0], c1Approx1[0])
	assert.Equal(t, c1[1], c1Approx1[1])
	assert.Equal(t, c1[2], c1Approx1[2])
	// epsilon larger than square diagonal: approximation should be equal to diagonal
	c1Approx2 := ApproxContourDP(c1, 71)
	assert.Equal(t, len(c1Approx2), 2)
	assert.Equal(t, c1[0], c1Approx2[0])
	assert.Equal(t, c1[2], c1Approx2[1])
}

func TestGetAreaCoveredByConvexContour(t *testing.T) {
	// create the contour of a square 4x4
	contour := []PointMat{{0, 0}, {0, 4}, {4, 4}, {4, 0}}
	area := GetAreaCoveredByConvexContour(contour)
	assert.Equal(t, area, 16.)
}

func TestSortPointsQuad(t *testing.T) {
	pts1 := []r2.Point{{0, 0}, {1, 0}, {0, 1}, {1, 1}}
	out := SortPointsQuad(pts1)
	fmt.Println(out)
	assert.Equal(t, out[0], r2.Point{0, 0})
	assert.Equal(t, out[1], r2.Point{1, 0})
	assert.Equal(t, out[2], r2.Point{1, 1})
	assert.Equal(t, out[3], r2.Point{0, 1})

	pts2 := []r2.Point{{0, 0}, {5, 0}, {4, 5}, {9, 5}}
	out2 := SortPointsQuad(pts2)
	fmt.Println(out2)
	assert.Equal(t, out2[0], r2.Point{0, 0})
	assert.Equal(t, out2[1], r2.Point{5, 0})
	assert.Equal(t, out2[2], r2.Point{9, 5})
	assert.Equal(t, out2[3], r2.Point{4, 5})
	//TODO(louise) add non orthogonal quadrilaterals
}

func TestArcLength(t *testing.T) {
	// rectangle 10x5 -> perimeter = 2*(10+5) = 30
	contour := []PointMat{{0, 0}, {0, 10}, {5, 10}, {5, 0}}
	l := ArcLength(contour)
	assert.Equal(t, l, 30.0)
}
