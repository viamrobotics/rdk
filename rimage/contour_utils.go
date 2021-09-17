package rimage

import (
	"image"
	"math"
	"sort"

	"github.com/golang/geo/r2"
	"github.com/gonum/floats"
	"go.viam.com/core/utils"
	"gonum.org/v1/gonum/mat"
)

// This package implements the algorithm to detect individual contours in a contour map and creates their hierarchy.
// From: Satoshi Suzuki and others. Topological structural analysis of digitized binary images by border following.
//       Computer Vision, Graphics, and Image Processing, 30(1):32–46, 1985.
// It also implements the Douglas Peucker algorithm (https://en.wikipedia.org/wiki/Ramer–Douglas–Peucker_algorithm)
// to approximate contours with a polygon

var NPixelNeighbors = 8

// GetAngle computes the angle given a 3 square side lengths, in degrees
func GetAngle(a, b, c float64) float64 {
	k := (a*a + b*b - c*c) / (2 * a * b)
	// handle floating point issues
	k = utils.ClampF64(k, -1, 1)
	return math.Acos(k) * 180 / math.Pi
}

// Border stores the ID of a border and its type (Hole or Outer)
type Border struct {
	segNum, borderType int
}

// enumeration for Border types
const (
	Hole = iota + 1
	Outer
)

// CreateHoleBorder creates a Hole Border
func CreateHoleBorder() Border {
	return Border{
		segNum:     0,
		borderType: Hole,
	}
}

// CreateOuterBorder creates an Outer Border
func CreateOuterBorder() Border {
	return Border{
		segNum:     0,
		borderType: Outer,
	}
}

// PointMat stores a point in matrix coordinates convention
type PointMat struct {
	Row, Col int
}

// Set sets a current point to new coordinates (r,c)
func (p *PointMat) Set(r, c int) {
	p.Col = c
	p.Row = r
}

// SamePoint returns true if a point q is equal to the point p
func (p *PointMat) SamePoint(q *PointMat) bool {
	return p.Row == q.Row && p.Col == q.Col
}

// ToImagePoint convert a local struct PointMap to an image.Point
func ToImagePoint(q PointMat) image.Point {
	return image.Point{q.Col, q.Row}
}

// Node is a structure storing data from each contour to form a tree
type Node struct {
	parent      int
	firstChild  int
	nextSibling int
	border      Border
}

// reset resets a Node to default values
func (n *Node) reset() {
	n.parent = -1
	n.firstChild = -1
	n.nextSibling = -1
}

// stepCCW4 set p to the next counter-clockwise neighbor of pivot from p in connectivity 4
func (p *PointMat) stepCCW4(pivot *PointMat) {
	if p.Col > pivot.Col {
		p.Row = pivot.Row - 1
		p.Col = pivot.Col
	} else if p.Col < pivot.Col {
		p.Row = pivot.Row + 1
		p.Col = pivot.Col
	} else if p.Row > pivot.Row {
		p.Row = pivot.Row
		p.Col = pivot.Col + 1
	} else if p.Row < pivot.Row {
		p.Row = pivot.Row
		p.Col = pivot.Col - 1
	}
}

// stepCW4 goes to the next clockwise neighbor of pivot from p in connectivity 4
func (p *PointMat) stepCW4(pivot *PointMat) {
	if p.Col > pivot.Col {
		p.Row = pivot.Row + 1
		p.Col = pivot.Col
	} else if p.Col < pivot.Col {
		p.Row = pivot.Row - 1
		p.Col = pivot.Col
	} else if p.Row > pivot.Row {
		p.Row = pivot.Row
		p.Col = pivot.Col - 1
	} else if p.Row < pivot.Row {
		p.Row = pivot.Row
		p.Col = pivot.Col + 1
	}
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
	return
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
			pointSlice = append(pointSlice, image.Point{
				X: start.Col,
				Y: start.Row,
			})

			return pointSlice
		}
	}
	p1 := current
	p3 := start
	p4 := PointMat{
		Row: 0,
		Col: 0,
	}
	p2 = p1
	checked := make([]bool, NPixelNeighbors, NPixelNeighbors)
	for {
		current = p2
		for ok := true; ok; ok = isPointOutOfBounds(&current, nRows, nCols) || img.At(current.Row, current.Col) == 0. {
			markExamined(current, p3, checked)
			current.stepCCW8(&p3)
		}
		p4 = current
		if (p3.Col+1 >= nCols || img.At(p3.Row, p3.Col+1) == 0) && isExamined(checked) {
			img.Set(p3.Row, p3.Col, float64(-nbp.segNum))
		} else if p3.Col+1 < nCols && img.At(p3.Row, p3.Col) == 1 {
			img.Set(p3.Row, p3.Col, float64(nbp.segNum))
		}
		pointSlice = append(pointSlice, ToImagePoint(p3))
		if p4.SamePoint(&start) && p3.SamePoint(&p1) {
			return pointSlice
		}
		p2 = p3
		p3 = p4
	}
}

// FindContours implements the contour hierarchy finding from Suzuki et al.
func FindContours(img *mat.Dense) ([][]image.Point, []Node) {
	hierarchy := make([]Node, 0)
	nRows, nCols := img.Dims()
	nbd := CreateHoleBorder()
	nbd.segNum = 1
	lnbd := CreateHoleBorder()
	contours := make([][]image.Point, 0)
	for i := range contours {
		contours[i] = make([]image.Point, 0)
	}
	tmpNode := Node{
		parent:      -1,
		firstChild:  -1,
		nextSibling: -1,
		border:      nbd,
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
				nbd.segNum += 1
				p2.Set(r, c-1)
				borderStartFound = true
			} else if c+1 < nCols && (img.At(r, c) >= 1 && img.At(r, c+1) == 0) {
				nbd.borderType = Hole
				nbd.segNum += 1
				if img.At(r, c) > 1 {
					lnbd.segNum = int(img.At(r, c))
					lnbd.borderType = hierarchy[lnbd.segNum-1].border.borderType
				}
				p2.Set(r, c+1)
				borderStartFound = true
			}
			if borderStartFound {
				tmpNode.reset()
				if nbd.borderType == lnbd.borderType {
					tmpNode.parent = hierarchy[lnbd.segNum-1].parent
					tmpNode.nextSibling = hierarchy[tmpNode.parent-1].firstChild
					hierarchy[tmpNode.parent-1].firstChild = nbd.segNum
					tmpNode.border = nbd
					hierarchy = append(hierarchy, tmpNode)
				} else {
					if hierarchy[lnbd.segNum-1].firstChild != -1 {
						tmpNode.nextSibling = hierarchy[lnbd.segNum-1].firstChild
					}
					tmpNode.parent = lnbd.segNum
					hierarchy[lnbd.segNum-1].firstChild = nbd.segNum
					tmpNode.border = nbd
					hierarchy = append(hierarchy, tmpNode)
				}
				contour := followBorder(img, r, c, p2, nbd)
				contour = SortPointCounterClockwise(contour)
				contours = append(contours, contour)
			}
			if math.Abs(img.At(r, c)) > 1 {
				lnbd.segNum = int(math.Abs(img.At(r, c)))
				//fmt.Println(len(hierarchy))
				//fmt.Println(lnbd.segNum)
				idx := lnbd.segNum - 1
				if idx < 0 {
					idx = 0
				}
				lnbd.borderType = hierarchy[idx].border.borderType
			}
		}
	}

	return contours, hierarchy
}

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
func ApproxContourDP(points []r2.Point, ep float64) []r2.Point {
	if len(points) <= 2 {
		return points
	}

	l := Line{Start: points[0], End: points[len(points)-1]}

	idx, maxDist := seekMostDistantPoint(l, points)
	if maxDist >= ep {
		left := ApproxContourDP(points[:idx+1], ep)
		right := ApproxContourDP(points[idx:], ep)
		return append(left[:len(left)-1], right...)
	}

	// If the most distant point fails to pass the threshold test, then just return the two points
	return []r2.Point{points[0], points[len(points)-1]}
}

// seekMostDistantPoint finds the point that is the most distant from the line defined by 2 points
func seekMostDistantPoint(l Line, points []r2.Point) (idx int, maxDist float64) {
	for i := 0; i < len(points); i++ {
		d := l.DistanceToPoint(points[i])
		if d > maxDist {
			maxDist = d
			idx = i
		}
	}

	return idx, maxDist
}

// GetAreaCoveredByConvexContour computes the area covered by the envelope of a convex contour
// The formula works for all polygons
func GetAreaCoveredByConvexContour(contour []PointMat) float64 {
	if len(contour) < 2 {
		return float64(len(contour))
	}
	sum := 0.
	for i := 0; i < len(contour); i++ {
		// get points i and i+1
		p0 := contour[i]
		p1 := contour[(i+1)%len(contour)]
		// update sum
		sum += float64(p0.Col*p1.Row - p0.Row*p1.Col)
	}
	// take half of absolute value of sum to obtain area
	return math.Abs(sum) / 2.
}

func IsContourSquare(contour []PointMat) bool {
	isSquare := false
	if len(contour) != 4 {
		return isSquare
	}
	p0 := convertPointMatToVec(contour[0])
	p1 := convertPointMatToVec(contour[1])
	p2 := convertPointMatToVec(contour[2])
	p3 := convertPointMatToVec(contour[3])
	// side lengths
	dd0 := p0.Sub(p1).Norm()
	dd1 := p1.Sub(p2).Norm()
	dd2 := p2.Sub(p3).Norm()
	dd3 := p3.Sub(p0).Norm()
	// diagonal lengths
	xa := p0.Sub(p2).Norm()
	xb := p1.Sub(p3).Norm()
	// check that points in contour are part of a convex hull
	ta := GetAngle(dd3, dd0, xb)
	tb := GetAngle(dd0, dd1, xa)
	tc := GetAngle(dd1, dd2, xb)
	td := GetAngle(dd2, dd3, xa)
	angles := []float64{ta, tb, tc, td}
	angleSum := floats.Sum(angles)
	isConvex := math.Abs(angleSum-360.) < 5.
	nGoodAngles := uint8(0)
	for _, angle := range angles {
		if angle > 40. && angle < 140. {
			nGoodAngles += 1
		}
	}
	isSquare = nGoodAngles == 4 && isConvex
	return isSquare
}

// ArcLength returns the perimeter of the contour
func ArcLength(contour []PointMat) float64 {
	lastIdx := len(contour) - 1
	prev := r2.Point{X: float64(contour[lastIdx].Col), Y: float64(contour[lastIdx].Row)}
	perimeter := 0.
	for i := 0; i < len(contour); i++ {
		p1 := r2.Point{X: float64(contour[i].Col), Y: float64(contour[i].Row)}
		d := p1.Sub(prev).Norm()
		perimeter += d
		prev = p1
	}
	return perimeter
}

func SimplifyContours(contours [][]PointMat) [][]r2.Point {
	simplifiedContours := make([][]r2.Point, len(contours))

	for i, c := range contours {
		eps := ArcLength(c)
		cf := convertSlicePointMatToSliceVec(c)
		sc := ApproxContourDP(cf, 0.04*eps)
		simplifiedContours[i] = sc
	}
	return simplifiedContours
}

type ByX []r2.Point

func (a ByX) Len() int           { return len(a) }
func (a ByX) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByX) Less(i, j int) bool { return a[i].X < a[j].X }

func SortPointsQuad(points []r2.Point) []r2.Point {
	sorted := make([]r2.Point, 4)
	out := make([]r2.Point, 4)
	copy(sorted, points)
	sort.Sort(ByX(sorted))
	// get top left and bottom left points
	if sorted[0].Y < sorted[1].Y {
		out[0] = sorted[0]
		out[3] = sorted[1]
	} else {
		out[0] = sorted[1]
		out[3] = sorted[0]
	}
	if sorted[2].Y < sorted[3].Y {
		out[1] = sorted[2]
		out[2] = sorted[3]
	} else {
		out[1] = sorted[3]
		out[2] = sorted[2]
	}
	return out
}

// helpers
// convertPointMatToVec converts a pointMat to a r2.Point
func convertPointMatToVec(p PointMat) r2.Point {
	return r2.Point{X: float64(p.Col), Y: float64(p.Row)}
}

func convertSlicePointMatToSliceVec(pts []PointMat) []r2.Point {
	out := make([]r2.Point, len(pts))
	for i, pt := range pts {
		out[i] = convertPointMatToVec(pt)
	}
	return out
}
