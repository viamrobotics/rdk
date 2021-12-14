package rimage

import (
	"image"
	"math"

	"github.com/golang/geo/r2"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

// This package implements:
// 1 . The algorithm to detect individual contours in a contour map and creates their hierarchy.
//       Suzuki, S. and Abe, K., Topological Structural Analysis of Digitized Binary
//      Images by Border Following. CVGIP 30 1, pp 32-46 (1985)
// 2. The Douglas Peucker algorithm to approximate a contour by a polygon
//    David Douglas & Thomas Peucker, "Algorithms for the reduction of the number of points required to represent a
//   digitized line or its caricature", The Canadian Cartographer 10(2), 112–122 (1973)


// ContourNode is a structure storing data from each contour to form a tree
type ContourNode struct {
	Parent      int
	FirstChild  int
	NextSibling int
	Border      Border
}

// reset resets a ContourNode to default values
func (n *ContourNode) reset() {
	n.Parent = -1
	n.FirstChild = -1
	n.NextSibling = -1
}

//stepCCW8 performs a step around a pixel CCW in the 8-connect neighborhood.
func (p *PointMat) stepCCW8(pivot *PointMat) {
	if p.Row == pivot.Row && p.Col > pivot.Col {
		p.Row = pivot.Row - 1
		p.Col = pivot.Col + 1
	} else if p.Col > pivot.Col && p.Row < pivot.Row {
		p.Row = pivot.Row - 1
		p.Col = pivot.Col
	} else if p.Row < pivot.Row && p.Col == pivot.Col {
		p.Row = pivot.Row - 1
		p.Col = pivot.Col - 1
	} else if p.Row < pivot.Row && p.Col < pivot.Col {
		p.Row = pivot.Row
		p.Col = pivot.Col - 1
	} else if p.Row == pivot.Row && p.Col < pivot.Col {
		p.Row = pivot.Row + 1
		p.Col = pivot.Col - 1
	} else if p.Row > pivot.Row && p.Col < pivot.Col {
		p.Row = pivot.Row + 1
		p.Col = pivot.Col
	} else if p.Row > pivot.Row && p.Col == pivot.Col {
		p.Row = pivot.Row + 1
		p.Col = pivot.Col + 1
	} else if p.Row > pivot.Row && p.Col > pivot.Col {
		p.Row = pivot.Row
		p.Col = pivot.Col + 1
	}
}

//stepCW8 performs a step around a pixel CCW in the 8-connect neighborhood.
func (p *PointMat) stepCW8(pivot *PointMat) {
	if p.Row == pivot.Row && p.Col > pivot.Col {
		p.Row = pivot.Row + 1
		p.Col = pivot.Col + 1
	} else if p.Col > pivot.Col && p.Row < pivot.Row {
		p.Row = pivot.Row
		p.Col = pivot.Col + 1
	} else if p.Row < pivot.Row && p.Col == pivot.Col {
		p.Row = pivot.Row - 1
		p.Col = pivot.Col + 1
	} else if p.Row < pivot.Row && p.Col < pivot.Col {
		p.Row = pivot.Row - 1
		p.Col = pivot.Col
	} else if p.Row == pivot.Row && p.Col < pivot.Col {
		p.Row = pivot.Row - 1
		p.Col = pivot.Col - 1
	} else if p.Row > pivot.Row && p.Col < pivot.Col {
		p.Row = pivot.Row
		p.Col = pivot.Col - 1
	} else if p.Row > pivot.Row && p.Col == pivot.Col {
		p.Row = pivot.Row + 1
		p.Col = pivot.Col - 1
	} else if p.Row > pivot.Row && p.Col > pivot.Col {
		p.Row = pivot.Row + 1
		p.Col = pivot.Col
	}
}

// isPointOutOfBounds checks if a given pixel is out of bounds of the image
func isPointOutOfBounds(p *PointMat, h, w int) bool {
	return p.Col >= w || p.Row >= h || p.Col < 0 || p.Row < 0
}

//marks a pixel as examined after passing through
func markExamined(mark, center PointMat, checked []bool) {
	loc := -1
	//    3
	//  2 x 0
	//    1
	if mark.Col > center.Col {
		loc = 0
	} else if mark.Col < center.Col {
		loc = 2
	} else if mark.Row > center.Row {
		loc = 1
	} else if mark.Row < center.Row {
		loc = 3
	}
	if loc != -1 {
		checked[loc] = true
	}
}

// isExamined checks if given pixel has already been examined
func isExamined(checked []bool) bool {
	return checked[0]
}

// followBorder is a helper function for the contour finding algorithm
func followBorder(img *mat.Dense, row, col int, p2 PointMat, nbp Border) []image.Point {
	nRows, nCols := img.Dims()
	current := PointMat{
		Row: p2.Row,
		Col: p2.Col,
	}
	start := PointMat{
		Row: row,
		Col: col,
	}
	pointSlice := make([]image.Point, 0)
	for ok := true; ok; ok = isPointOutOfBounds(&current, nRows, nCols) || img.At(current.Row, current.Col) == 0. {
		current.stepCW8(&start)
		if current.SamePoint(&p2) {
			img.Set(start.Row, start.Col, float64(-nbp.segNum))
			pointSlice = append(pointSlice, image.Point{start.Col, start.Row})

			return pointSlice
		}
	}
	p1 := current
	p3 := start
	p4 := PointMat{
		Row: 0,
		Col: 0,
	}
	p2.SetTo(p1)
	checked := make([]bool, 8)  // we proceed in 8-connectivity
	for {
		current.SetTo(p2)
		for ok := true; ok; ok = isPointOutOfBounds(&current, nRows, nCols) || img.At(current.Row, current.Col) == 0. {
			markExamined(current, p3, checked)
			current.stepCCW8(&p3)
		}
		p4.SetTo(current)
		if (p3.Col+1 >= nCols || img.At(p3.Row, p3.Col+1) == 0) && isExamined(checked) {
			img.Set(p3.Row, p3.Col, float64(-nbp.segNum))
		} else if p3.Col+1 < nCols && img.At(p3.Row, p3.Col) == 1 {
			img.Set(p3.Row, p3.Col, float64(nbp.segNum))
		}
		pointSlice = append(pointSlice, image.Point{p3.Col, p3.Row}) // adding p3 to contour
		if p4.SamePoint(&start) && p3.SamePoint(&p1) {
			return pointSlice
		}

		p2.SetTo(p3)
		p3.SetTo(p4)
	}

}

// FindContours implements the contour hierarchy finding from Suzuki et al.
func FindContours(img *mat.Dense) ([][]image.Point, []ContourNode) {
	hierarchy := make([]ContourNode, 0)
	nRows, nCols := img.Dims()
	nbd := CreateHoleBorder()
	nbd.segNum = 1
	lnbd := CreateHoleBorder()
	contours := make([][]image.Point, 0)
	for i := range contours {
		contours[i] = make([]image.Point, 0)
	}
	tmpNode := ContourNode{
		Parent:      -1,
		FirstChild:  -1,
		NextSibling: -1,
		Border:      nbd,
	}
	hierarchy = append(hierarchy, tmpNode)
	p2 := PointMat{
		Row: 0,
		Col: 0,
	}
	borderStartFound := false
	for r := 0; r < nRows; r++ {
		lnbd.segNum = 1
		lnbd.borderType = Hole
		for c := 0; c < nCols; c++ {
			borderStartFound = false
			if (img.At(r, c) == 1 && c-1 < 0) || (img.At(r, c) == 1 && img.At(r, c-1) == 0) {
				nbd.borderType = Outer
				nbd.segNum++
				p2.Set(r, c-1)
				borderStartFound = true
			} else if c+1 < nCols && (img.At(r, c) >= 1 && img.At(r, c+1) == 0) {
				nbd.borderType = Hole
				nbd.segNum++
				if img.At(r, c) > 1 {
					lnbd.segNum = int(img.At(r, c))
					lnbd.borderType = hierarchy[lnbd.segNum-1].Border.borderType
				}
				p2.Set(r, c+1)
				borderStartFound = true
			}
			if borderStartFound {
				tmpNode.reset()
				if nbd.borderType == lnbd.borderType {
					tmpNode.Parent = hierarchy[lnbd.segNum-1].Parent
					tmpNode.NextSibling = hierarchy[tmpNode.Parent-1].FirstChild
					hierarchy[tmpNode.Parent-1].FirstChild = nbd.segNum
					tmpNode.Border = nbd
					hierarchy = append(hierarchy, tmpNode)
				} else {
					if hierarchy[lnbd.segNum-1].FirstChild != -1 {
						tmpNode.NextSibling = hierarchy[lnbd.segNum-1].FirstChild
					}
					tmpNode.Parent = lnbd.segNum
					hierarchy[lnbd.segNum-1].FirstChild = nbd.segNum
					tmpNode.Border = nbd
					hierarchy = append(hierarchy, tmpNode)
				}
				contour := followBorder(img, r, c, p2, nbd)
				//fmt.Println(contour)
				contour = SortImagePointCounterClockwise(contour)
				contours = append(contours, contour)
			}
			if math.Abs(img.At(r, c)) > 1 {
				lnbd.segNum = int(math.Abs(img.At(r, c)))
				idx := lnbd.segNum - 1
				if idx < 0 {
					idx = 0
				}
				lnbd.borderType = hierarchy[idx].Border.borderType
			}
		}
	}

	return contours, hierarchy
}

// ToImagePoint convert a local struct PointMap to an image.Point
func ToImagePoint(q PointMat) image.Point {
	return image.Point{q.Col, q.Row}
}

// Douglas-Peucker algorithm to simplify contours

// Line represents a line segment.
type Line struct {
	Start r2.Point
	End   r2.Point
}

// DistanceToPoint returns the perpendicular distance of a point to the line.
func (l Line) DistanceToPoint(pt r2.Point) float64 {
	a, b, c := l.Coefficients()
	return math.Abs(a*pt.X+b*pt.Y+c) / math.Sqrt(a*a+b*b)
}

// Coefficients returns the three coefficients that define a line.
// A line can be represented by the following equation:
// ax + by + c = 0
func (l Line) Coefficients() (a, b, c float64) {
	a = l.Start.Y - l.End.Y
	b = l.End.X - l.Start.X
	c = l.Start.X*l.End.Y - l.End.X*l.Start.Y

	return a, b, c
}

// ApproxContourDP accepts a slice of points and a threshold epsilon, and approximates a contour by
// a succession of segments
func ApproxContourDP(points ContourFloat, ep float64) ContourFloat {
	if len(points) <= 2 {
		return points
	}
	points2 := make(ContourFloat, len(points))
	copy(points2, points)

	if IsContourClosed(points, 5) {
		points2 = ReorderClosedContourWithFarthestPoints(points2)
	}
	l := Line{Start: points2[0], End: points2[len(points)-1]}

	idx, maxDist := seekMostDistantPoint(l, points2)
	if maxDist >= ep {
		left := ApproxContourDP(points2[:idx+1], ep)
		right := ApproxContourDP(points2[idx:], ep)
		return append(left[:len(left)-1], right...)
	}

	// If the most distant point fails to pass the threshold test, then just return the two points
	return ContourFloat{points[0], points[len(points)-1]}
}

// seekMostDistantPoint finds the point that is the most distant from the line defined by 2 points
func seekMostDistantPoint(l Line, points ContourFloat) (idx int, maxDist float64) {
	for i := 0; i < len(points); i++ {
		d := l.DistanceToPoint(points[i])
		if d > maxDist {
			maxDist = d
			idx = i
		}
	}

	return idx, maxDist
}

// SimplifyContours iterates through a slice of contours and performs the contour approximation from ApproxContourDP
func SimplifyContours(contours [][]image.Point) []ContourFloat {
	simplifiedContours := make([]ContourFloat, len(contours))

	for i, c := range contours {
		cf := ConvertContourIntToContourFloat(c)
		sc := ApproxContourDP(cf, 0.03*float64(len(c)))
		simplifiedContours[i] = sc
	}
	return simplifiedContours
}

// SortPointCounterClockwise sorts a slice of image.Point in counterclockwise order, starting from point closest to -pi
func SortPointCounterClockwise(pts []r2.Point) []r2.Point {
	// create new slice of points
	out := make([]r2.Point, len(pts))
	xs, ys := SliceVecsToXsYs(pts)
	xMin := floats.Min(xs)
	xMax := floats.Max(xs)
	yMin := floats.Min(ys)
	yMax := floats.Max(ys)
	centerX := xMin + (xMax-xMin)/2
	centerY := yMin + (yMax-yMin)/2
	floats.AddConst(-centerX, xs)
	floats.AddConst(-centerY, ys)
	angles := make([]float64, len(pts))
	for i := range xs {
		angles[i] = math.Atan2(ys[i], xs[i])
	}
	inds := make([]int, len(pts))
	floats.Argsort(angles, inds)

	for i := 0; i < len(pts); i++ {
		idx := inds[i]
		x := math.Round(xs[idx] + centerX)
		y := math.Round(ys[idx] + centerY)
		out[i] = r2.Point{X: x, Y: y}
	}
	return out
}

// SortImagePointCounterClockwise sorts a slice of image.Point in counterclockwise order, starting from point closest to -pi
func SortImagePointCounterClockwise(pts []image.Point) []image.Point {
	// create new slice of points
	out := make([]image.Point, len(pts))
	xs, ys := SlicePointsToXsYs(pts)
	centerX := floats.Sum(xs) / float64(len(xs))
	centerY := floats.Sum(ys) / float64(len(ys))
	floats.AddConst(-centerX, xs)
	floats.AddConst(-centerY, ys)
	angles := make([]float64, len(pts))
	for i := range xs {
		angles[i] = math.Atan2(ys[i], xs[i])
	}
	inds := make([]int, len(pts))
	floats.Argsort(angles, inds)

	for i := 0; i < len(pts); i++ {
		idx := inds[i]
		x := math.Round(xs[idx] + centerX)
		y := math.Round(ys[idx] + centerY)
		out[i] = image.Point{X: int(x), Y: int(y)}
	}
	return out
}
