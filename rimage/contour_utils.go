package rimage

import (
	"image"
	"math"

	"github.com/golang/geo/r2"
	"gonum.org/v1/gonum/mat"
)

// This package implements the algorithm to detect individual contours in a contour map and creates their hierarchy.
// From: Satoshi Suzuki and others. Topological structural analysis of digitized binary images by border following.
//       Computer Vision, Graphics, and Image Processing, 30(1):32–46, 1985.
// It also implements the Douglas Peucker algorithm (https://en.wikipedia.org/wiki/Ramer–Douglas–Peucker_algorithm)
// to approximate contours with a polygon

var NPixelNeighbors = 8

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

// ArcLength returns the perimeter of the contour
func ArcLength(contour []image.Point) float64 {
	lastIdx := len(contour) - 1
	prev := r2.Point{X: float64(contour[lastIdx].X), Y: float64(contour[lastIdx].Y)}
	perimeter := 0.
	for i := 0; i < len(contour); i++ {
		p1 := r2.Point{X: float64(contour[i].X), Y: float64(contour[i].Y)}
		d := p1.Sub(prev).Norm()
		perimeter += d
		prev = p1
	}
	return perimeter
}

// SimplifyContours iterates through a slice of contours and performs the contour approximation from ApproxContourDP
func SimplifyContours(contours [][]image.Point) [][]r2.Point {
	simplifiedContours := make([][]r2.Point, len(contours))

	for i, c := range contours {
		eps := ArcLength(c)
		cf := ConvertSliceImagePointToSliceVec(c)
		sc := ApproxContourDP(cf, 0.04*eps)
		simplifiedContours[i] = sc
	}
	return simplifiedContours
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

func DeltasToIndex(x, y int) int {
	return 4 - 2*x + x*y - 2*(1-x*x)*(y*y-y)
}

var (
	Index2DeltaX = []int{0, 1, 1, 1, 0, -1, -1, -1}
	Index2DeltaY = []int{-1, -1, 0, 1, 1, 1, 0, -1}
)

const (
	Clockwise = iota
	CounterClockwise
)

type Direction int

// RoundNext returns the next pixel around center from start in the direction dir
func RoundNext(center, start image.Point, dir int) image.Point {
	deltaX := start.X - center.X
	deltaY := start.Y - center.Y
	index := DeltasToIndex(deltaX, deltaY)
	nextIndex := (index + 9 - 2*dir) % 8
	//fmt.Println("index : ", index)
	//fmt.Println("nextIndex : ", nextIndex)
	nextDeltaX := Index2DeltaX[nextIndex]
	nextDeltaY := Index2DeltaY[nextIndex]
	next := image.Point{
		X: center.X + nextDeltaX,
		Y: center.Y + nextDeltaY,
	}
	return next
}
func IsPointWithinBounds(pt image.Point, im *mat.Dense) bool {
	rows, cols := im.Dims()
	return pt.X >= 0 && pt.X < cols && pt.Y >= 0 && pt.Y < rows
}
func RoundNextForeground1(im *mat.Dense, center, start image.Point, dir int) (image.Point, bool) {
	found := false
	current := start
	var nextFG image.Point
	for i := 0; i < 7; i++ {
		next := RoundNext(center, current, dir)
		if IsPointWithinBounds(next, im) {
			v := im.At(next.Y, next.X)
			if v != 0 {
				found = true
				nextFG = next
				break
			}
		}
		current = next
	}
	return nextFG, found
}

func RoundNextForeground2(im *mat.Dense, center, start, through image.Point, dir int) (image.Point, bool, bool) {
	found, passedThrough := false, false
	current := start
	var nextFG image.Point
	for i := 0; i < 7; i++ {
		next := RoundNext(center, current, dir)
		if IsPointWithinBounds(next, im) {
			v := im.At(next.Y, next.X)
			if v != 0 {
				found = true
				nextFG = next
				break
			}
		}
		if next == through {
			passedThrough = true
		}
		current = next
	}
	return nextFG, found, passedThrough
}

// FindContoursSuzuki implements the contour finding algorithm in a binary image from Suzuki et al.
func FindContoursSuzuki(img *mat.Dense) ([][]image.Point, [][]int) {
	nRows, nCols := img.Dims()
	im := mat.NewDense(nRows, nCols, nil)
	im = mat.DenseCopyOf(img)
	contours := make([][]image.Point, 0)
	hierarchy := make([][]int, 0)
	cOuter := make([]bool, 0)
	lastSons := make([]int, 0)
	lastOuterMost := -1
	NBD, LNBD := 1, 1
	found, passedThrough := false, false
	for r := 1; r < nRows-1; r++ {
		LNBD = 1
		for c := 1; c < nCols-1; c++ {
			if im.At(r, c) != 0 {
				p := image.Point{c, r}
				var p1, p2, p3, p4 image.Point
				start := false

				if (im.At(r, c) == 1 && c-1 < 0) || (im.At(r, c) == 1 && im.At(r, c-1) == 0) {
					// outer contour found
					p2 = image.Point{c - 1, r}
					start = true
					cOuter = append(cOuter, true)
				} else if c+1 < nCols-1 && (im.At(r, c) >= 1 && im.At(r, c+1) == 0) {
					//c+1 < nCols && (im.At(r,c) >= 1 && im.At(r,c+1) == 0)
					// Hole border found
					p2 = image.Point{r, c + 1}
					if im.At(r, c) > 1 {
						LNBD = int(im.At(r, c))
					}
					start = true
					cOuter = append(cOuter, false)
				}
				// if border found
				if start {
					NBD++
					cnt := make([]image.Point, 0)
					cnt = append(cnt, image.Point{c, r})
					lastSons = append(lastSons, -1)
					cur := len(contours)
					father, prevSibling, bPrimo := -1, -1, LNBD-2
					// if outer contour
					if cOuter[len(cOuter)-1] {
						if bPrimo >= 0 && cOuter[bPrimo] {
							father = hierarchy[bPrimo][3]
						} else {
							father = bPrimo
						}
					} else {
						if bPrimo == -1 || cOuter[bPrimo] {
							father = bPrimo

						} else {
							father = hierarchy[bPrimo][3]
						}
					}
					// if contour has a parent, set siblings
					if father >= 0 {
						prevSibling = lastSons[father]
						if prevSibling < 0 {
							hierarchy[father][2] = cur
						} else {
							hierarchy[prevSibling][0] = cur
						}
						lastSons[father] = cur
					} else {
						if lastOuterMost >= 0 {
							hierarchy[lastOuterMost][0] = cur
							prevSibling = lastOuterMost
						}
						lastOuterMost = cur
					}
					hierarchy = append(hierarchy, []int{-1, prevSibling, -1, father})
					// follow contour - check for first non-zero neighbor clockwise
					p1, found = RoundNextForeground1(im, p, p2, Clockwise)
					if found {
						p2, p3 = p1, p
						for {
							through := image.Point{p3.X + 1, p3.Y}
							passedThrough = false
							p4, found, passedThrough = RoundNextForeground2(im, p3, p2, through, CounterClockwise)
							if !found {
								p4 = p2
							}
							if passedThrough {
								im.Set(p3.Y, p3.X, float64(-NBD))
							} else if im.At(p3.Y, p3.X) == 1 {
								im.Set(p3.Y, p3.X, float64(NBD))
							}
							if p4 == p && p3 == p1 {
								// we are back to the starting point
								break
							}
							cnt = append(cnt, p4)
							// update points
							p2 = p3
							p3 = p4
						}
					} else {
						im.Set(r, c, float64(-NBD))
					}
					contours = append(contours, cnt)
				}
				if im.At(r, c) != 1 {
					LNBD = int(math.Abs(im.At(r, c)))
				}
			}
		}
	}
	return contours, hierarchy
}
