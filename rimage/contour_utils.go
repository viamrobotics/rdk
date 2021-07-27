package rimage

import "math"

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
