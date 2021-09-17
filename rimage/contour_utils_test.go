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
}

func TestCreateOuterBorder(t *testing.T) {
	border := CreateOuterBorder()
	test.That(t, border.borderType, test.ShouldEqual, Outer)
}

func TestPointMat(t *testing.T) {
	p := PointMat{
		Row: 0,
		Col: 0,
	}
	test.That(t, p.Row, test.ShouldEqual, 0)
	test.That(t, p.Col, test.ShouldEqual, 0)
	p.Set(1, 2)
	test.That(t, p.Row, test.ShouldEqual, 1)
	test.That(t, p.Col, test.ShouldEqual, 2)
	q := PointMat{
		Row: 1,
		Col: 2,
	}
	test.That(t, p.SamePoint(&q), test.ShouldBeTrue)
	out := isPointOutOfBounds(&p, 2, 2)
	test.That(t, out, test.ShouldBeTrue)
	out2 := isPointOutOfBounds(&p, 3, 3)
	test.That(t, out2, test.ShouldBeFalse)
}

func TestNode(t *testing.T) {
	node := Node{
		parent:      0,
		firstChild:  0,
		nextSibling: 0,
		border:      Border{},
	}
	// test creation
	test.That(t, node.parent, test.ShouldEqual, 0)
	test.That(t, node.firstChild, test.ShouldEqual, 0)
	test.That(t, node.nextSibling, test.ShouldEqual, 0)
	test.That(t, node.border.borderType, test.ShouldEqual, 0)
	// test reset function
	node.reset()
	test.That(t, node.parent, test.ShouldEqual, -1)
	test.That(t, node.firstChild, test.ShouldEqual, -1)
	test.That(t, node.nextSibling, test.ShouldEqual, -1)
}

func TestMarkAsExamined(t *testing.T) {
	center := PointMat{1, 1}
	mark0 := PointMat{1, 2}
	mark1 := PointMat{2, 1}
	mark2 := PointMat{1, 0}
	mark3 := PointMat{0, 1}
	checked := make([]bool, 4)
	test.That(t, checked[0], test.ShouldBeFalse)
	markExamined(mark0, center, checked)
	test.That(t, checked[0], test.ShouldBeTrue)
	test.That(t, isExamined(checked), test.ShouldBeTrue)
	test.That(t, checked[1], test.ShouldBeFalse)
	markExamined(mark1, center, checked)
	test.That(t, checked[1], test.ShouldBeTrue)
	test.That(t, checked[2], test.ShouldBeFalse)
	markExamined(mark2, center, checked)
	test.That(t, checked[2], test.ShouldBeTrue)
	test.That(t, checked[3], test.ShouldBeFalse)
	markExamined(mark3, center, checked)
	test.That(t, checked[3], test.ShouldBeTrue)
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
	test.That(t, len(hierarchy), test.ShouldEqual, 5)
	test.That(t, hierarchy[0].parent, test.ShouldEqual, -1)
	test.That(t, hierarchy[1].parent, test.ShouldEqual, 1)
	test.That(t, hierarchy[2].parent, test.ShouldEqual, 2)
	test.That(t, hierarchy[1].parent, test.ShouldEqual, 1)
	test.That(t, hierarchy[4].parent, test.ShouldEqual, 4)
	// Test contours length
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

func TestGetAreaCoveredByConvexContour(t *testing.T) {
	// create the contour of a square 4x4
	contour := []PointMat{{0, 0}, {0, 4}, {4, 4}, {4, 0}}
	area := GetAreaCoveredByConvexContour(contour)
	test.That(t, area, test.ShouldEqual, 16.0)
}

//func TestSortPointsQuad(t *testing.T) {
//	pts1 := []r2.Point{{0, 0}, {1, 0}, {0, 1}, {1, 1}}
//	out := SortPointsQuad(pts1)
//	test.That(t, out[0]==r2.Point{0, 0}, test.ShouldBeTrue)
//	test.That(t, out[1]==r2.Point{1, 0}, test.ShouldBeTrue)
//	test.That(t, out[2]==r2.Point{1, 1}, test.ShouldBeTrue)
//	test.That(t, out[3]==r2.Point{0, 1}, test.ShouldBeTrue)
//
//	//pts2 := []r2.Point{{0, 0}, {5, 0}, {4, 5}, {9, 5}}
//	//out2 := SortPointsQuad(pts2)
//	//fmt.Println(out2)
//	//assert.Equal(t, out2[0], r2.Point{0, 0})
//	//assert.Equal(t, out2[1], r2.Point{5, 0})
//	//assert.Equal(t, out2[2], r2.Point{9, 5})
//	//assert.Equal(t, out2[3], r2.Point{4, 5})
//	//TODO(louise) add non orthogonal quadrilaterals
//}

func TestArcLength(t *testing.T) {
	// rectangle 10x5 -> perimeter = 2*(10+5) = 30
	contour := []PointMat{{0, 0}, {0, 10}, {5, 10}, {5, 0}}
	l := ArcLength(contour)
	test.That(t, l, test.ShouldEqual, 30.0)
}
