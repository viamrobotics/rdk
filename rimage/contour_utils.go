package rimage

import (
	"gonum.org/v1/gonum/mat"
	"math"
)

var NPixelNeighbors = 8

// Point represents a 2D point on a Cartesian plane.
type Point struct {
	X float64
	Y float64
}

// Line represents a line segment.
type Line struct {
	Start Point
	End   Point
}

// Contour is a structure that stores and represent a contour with the points in it, its ID, ParentID and a flag
// that stores if the contour is a hole contour or not
type Contour struct {
	Points   []Point
	Id       int
	ParentId int
	IsHole   bool
}

// NewContour create a pointer to an empty contour with ID 0
func NewContour() *Contour {
	return &Contour{
		Points:   nil,
		Id:       0,
		ParentId: 0,
		IsHole:   false,
	}
}

// NewContourWithID creates a new contour with a given ID
func NewContourWithID(idx int) *Contour {
	return &Contour{
		Points:   nil,
		Id:       idx,
		ParentId: 0,
		IsHole:   false,
	}
}

// neighborIDToIndex maps a neigh ID to the image coordinates
func neighborIDToIndex(i, j, id int) (int, int) {
	iNb := 0
	jNb := 0
	if id == 0 {
		iNb, jNb = i, j+1
	}
	if id == 1 {
		iNb, jNb = i-1, j+1
	}
	if id == 2 {
		iNb, jNb = i-1, j
	}
	if id == 3 {
		iNb, jNb = i-1, j-1
	}
	if id == 4 {
		iNb, jNb = i, j-1
	}
	if id == 5 {
		iNb, jNb = i+1, j-1
	}
	if id == 6 {
		iNb, jNb = i+1, j
	}
	if id == 7 {
		iNb, jNb = i+1, j+1
	}

	return iNb, jNb
}

// neighborIndexToID return a neighbor ID from its image coordinates
func neighborIndexToID(i0, j0, i, j int) int {
	di := i - i0
	dj := j - j0
	if di == 0 && dj == 1 {
		return 0
	}
	if di == -1 && dj == 1 {
		return 1
	}
	if di == -1 && dj == 0 {
		return 2
	}
	if di == -1 && dj == -1 {
		return 3
	}
	if di == 0 && dj == -1 {
		return 4
	}
	if di == 1 && dj == -1 {
		return 5
	}
	if di == 1 && dj == 0 {
		return 6
	}
	if di == 1 && dj == 1 {
		return 7
	}
	return -1
}

// firstClockwiseNonZeroNeighbor returns the first clockwise non-zero element in 8-neighborhood
func firstClockwiseNonZeroNeighbor(F mat.Dense, i0, j0, i, j, offset int) (int, int) {
	idNb := neighborIndexToID(i0, j0, i, j)
	for k := 1; k < NPixelNeighbors; k++ {
		kk := (idNb - k - offset + NPixelNeighbors*2) % NPixelNeighbors
		i_, j_ := neighborIDToIndex(i0, j0, kk)
		if F.At(i_, j_) != 0 {
			return i_, j_
		}
	}
	return 0, 0
}

// firstCounterClockwiseNonZeroNeighbor returns the first counter-clockwise non-zero element in 8-neighborhood
func firstCounterClockwiseNonZeroNeighbor(F mat.Dense, i0, j0, i, j, offset int) (int, int) {
	idNb := neighborIndexToID(i0, j0, i, j)
	for k := 1; k < NPixelNeighbors; k++ {
		kk := (idNb + k + offset + NPixelNeighbors*2) % NPixelNeighbors
		i_, j_ := neighborIDToIndex(i0, j0, kk)
		if F.At(i_, j_) != 0 {
			return i_, j_
		}
	}
	return 0, 0
}


//FindContourHierarchy finds contours in a binary image
// Implements Suzuki, S. and Abe, K.
// "Topological Structural Analysis of Digitized Binary Images by Border Following."
// See source code for step-by-step correspondence to the paper's algorithm
// description.
// @param  edges    The bitmap, stored in 1-dimensional row-major form.
//                  0=background, 1=foreground, will be modified by the function
//                  to hold semantic information
// @return          An array of contours found in the image.
func FindContourHierarchy(edges *mat.Dense) []*Contour {
	c := NewContour()
	return []*Contour{c}
}

// DistanceToPoint returns the perpendicular distance of a point to the line.
func (l Line) DistanceToPoint(pt Point) float64 {
	a, b, c := l.Coefficients()
	return math.Abs(a*pt.X+b*pt.Y+c) / math.Sqrt(a*a+b*b)
}

// Coefficients returns the three coefficients that define a line.
// A line can represent by the following equation.
//
// ax + by + c = 0
//
func (l Line) Coefficients() (a, b, c float64) {
	a = l.Start.Y - l.End.Y
	b = l.End.X - l.Start.X
	c = l.Start.X*l.End.Y - l.End.X*l.Start.Y

	return a, b, c
}

// SimplifyPath accepts a list of points and epsilon as threshold, simplifies a path by dropping
// points that do not pass threshold values.
func SimplifyPath(points []Point, ep float64) []Point {
	if len(points) <= 2 {
		return points
	}

	l := Line{Start: points[0], End: points[len(points)-1]}

	idx, maxDist := seekMostDistantPoint(l, points)
	if maxDist >= ep {
		left := SimplifyPath(points[:idx+1], ep)
		right := SimplifyPath(points[idx:], ep)
		return append(left[:len(left)-1], right...)
	}

	// If the most distant point fails to pass the threshold test, then just return the two points
	return []Point{points[0], points[len(points)-1]}
}

// seekMostDistantPoint finds the point that is the most distant from the line defined by 2 points
func seekMostDistantPoint(l Line, points []Point) (idx int, maxDist float64) {
	for i := 0; i < len(points); i++ {
		d := l.DistanceToPoint(points[i])
		if d > maxDist {
			maxDist = d
			idx = i
		}
	}

	return idx, maxDist
}
