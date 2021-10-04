package rimage

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"github.com/fogleman/gg"
	"github.com/golang/geo/r2"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/mat"
)

// This package implements the algorithm to detect individual contours in a contour map and creates their hierarchy.
// From: Satoshi Suzuki and others. Topological structural analysis of digitized binary images by Border following.
//       Computer Vision, Graphics, and Image Processing, 30(1):32–46, 1985.
// It also implements the Douglas Peucker algorithm (https://en.wikipedia.org/wiki/Ramer–Douglas–Peucker_algorithm)
// to approximate contours with a polygon

// NPixelNeighbors stores the number of neighbors for each pixel (should be 4 or 8)
var NPixelNeighbors = 8

// Border stores the ID of a Border and its type (Hole or Outer)
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

// SetTo sets a current point to new coordinates (r,c)
func (p *PointMat) SetTo(p2 PointMat) {
	p.Col = p2.Col
	p.Row = p2.Row
}

// SamePoint returns true if a point q is equal to the point p
func (p *PointMat) SamePoint(q *PointMat) bool {
	return p.Row == q.Row && p.Col == q.Col
}

// Node is a structure storing data from each contour to form a tree
type Node struct {
	Parent      int
	FirstChild  int
	NextSibling int
	Border      Border
}

// reset resets a Node to default values
func (n *Node) reset() {
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
	checked := make([]bool, NPixelNeighbors)
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
	points2 := make([]r2.Point, len(points))
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

// SimplifyContours iterates through a slice of contours and performs the contour approximation from ApproxContourDP
func SimplifyContours(contours [][]image.Point) [][]r2.Point {
	simplifiedContours := make([][]r2.Point, len(contours))

	for i, c := range contours {
		cf := ConvertSliceImagePointToSliceVec(c)
		sc := ApproxContourDP(cf, 0.03*float64(len(c)))
		simplifiedContours[i] = sc
	}
	return simplifiedContours
}

// ContourPoint is a simple structure for readability
type ContourPoint struct {
	Point r2.Point
	Idx   int
}

// GetPairOfFarthestPointsContour take an ordered contour and returns the 2 farthest points and their respective
// indices in that contour
func GetPairOfFarthestPointsContour(points []r2.Point) (ContourPoint, ContourPoint) {
	start := ContourPoint{points[0], 0}
	end := ContourPoint{points[1], 1}
	distMax := 0.0
	for i, p := range points {
		for j, q := range points {
			d := p.Sub(q).Norm()
			if d > distMax {
				start = ContourPoint{p, i}
				end = ContourPoint{q, j}
				distMax = d
			}
		}
	}
	return start, end
}

// IsContourClosed takes a sorted contour, and returns true if the first and last points of it are spatially epsilon-close
func IsContourClosed(points []r2.Point, eps float64) bool {
	start := points[0]
	end := points[len(points)-1]
	d := end.Sub(start).Norm()

	return d < eps
}

// ReorderClosedContourWithFarthestPoints takes a closed ordered contour and reorders the points in it so that all the
// points from p1 to p2 are in the first part of the contour and points from p2 to p1 in the second part
func ReorderClosedContourWithFarthestPoints(points []r2.Point) []r2.Point {
	start, end := GetPairOfFarthestPointsContour(points)
	if start.Idx > end.Idx {
		start, end = end, start
	}
	c1 := points[start.Idx:end.Idx]
	c2 := points[end.Idx:]
	c2 = append(c2, points[:start.Idx]...)
	reorderedContour := append(c1, c2...)
	return reorderedContour
}

// helpers
// convertImagePointToVec converts an image.Point to a r2.Point
func convertImagePointToVec(p image.Point) r2.Point {
	return r2.Point{X: float64(p.X), Y: float64(p.Y)}
}

// ConvertSliceImagePointToSliceVec converts a slice of image.Point to a slice of r2.Point
func ConvertSliceImagePointToSliceVec(pts []image.Point) []r2.Point {
	out := make([]r2.Point, len(pts))
	for i, pt := range pts {
		out[i] = convertImagePointToVec(pt)
	}
	return out
}

// savePNG takes an image.Image and saves it to a png file fn
func savePNG(fn string, m image.Image) error {
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	utils.UncheckedErrorFunc(f.Close)
	return png.Encode(f, m)
}

// Viasualization functions

// DrawContours draws the contours in a black image and saves it in outFile
func DrawContours(img *mat.Dense, contours [][]image.Point, outFile string) *image.Gray16 {
	h, w := img.Dims()
	g := image.NewGray16(image.Rect(0, 0, w, h))

	for i, cnt := range contours {
		for _, pt := range cnt {
			g.SetGray16(pt.X, pt.Y, color.Gray16{uint16(i)})
		}
	}

	if err := savePNG(outFile, g); err != nil {
		panic(err)
	}
	return g
}

// DrawContoursSimplified draws the simplified polygonal contours in a black image and saves it in outFile
func DrawContoursSimplified(img *mat.Dense, contours [][]r2.Point, outFile string) error {
	h, w := img.Dims()
	dc := gg.NewContext(w, h)
	dc.SetRGB(0, 0, 0)
	dc.Clear()
	for _, cnt := range contours {
		nPoints := len(cnt)
		for i, pt := range cnt {
			x1, y1 := pt.X, pt.Y
			x2, y2 := cnt[(i+1)%nPoints].X, cnt[(i+1)%nPoints].Y
			r := 1.0
			g := 1.0
			b := 1.0
			a := 1.0
			dc.SetRGBA(r, g, b, a)
			dc.SetLineWidth(1)
			dc.DrawLine(x1, y1, x2, y2)
			dc.Stroke()
		}
	}
	err := dc.SavePNG(outFile)
	return err
}
