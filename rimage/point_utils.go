package rimage

import (
	"image"
	"math"

	"go.viam.com/robotcore/utils"
)

var (
	NoPoints = image.Point{-1, -1}
)

func Center(contour []image.Point, maxDiff int) image.Point {
	if len(contour) == 0 {
		return NoPoints
	}

	x := 0
	y := 0

	for _, p := range contour {
		x += p.X
		y += p.Y
	}

	weightedMiddle := image.Point{x / len(contour), y / len(contour)}

	numPoints := 0
	box := image.Rectangle{image.Point{1000000, 100000}, image.Point{0, 0}}
	for _, p := range contour {
		if utils.AbsInt(p.X-weightedMiddle.X) > maxDiff || utils.AbsInt(p.Y-weightedMiddle.Y) > maxDiff {
			continue
		}

		numPoints = numPoints + 1

		if p.X < box.Min.X {
			box.Min.X = p.X
		}
		if p.Y < box.Min.Y {
			box.Min.Y = p.Y
		}

		if p.X > box.Max.X {
			box.Max.X = p.X
		}
		if p.Y > box.Max.Y {
			box.Max.Y = p.Y
		}

	}

	if numPoints == 0 {
		return NoPoints
	}

	avgMiddle := image.Point{(box.Min.X + box.Max.X) / 2, (box.Min.Y + box.Max.Y) / 2}
	//fmt.Printf("%v -> %v  box: %v\n", weightedMiddle, avgMiddle, box)
	return avgMiddle
}

func PointDistance(a, b image.Point) float64 {
	x := utils.SquareInt(b.X - a.X)
	x += utils.SquareInt(b.Y - a.Y)
	return math.Sqrt(float64(x))
}

func PointAngle(a, b image.Point) float64 {
	x := b.X - a.X
	y := b.Y - a.Y
	return math.Atan2(float64(y), float64(x))
}

func BoundingBox(pts []image.Point) image.Rectangle {
	min := image.Point{math.MaxInt32, math.MaxInt32}
	max := image.Point{0, 0}

	for _, p := range pts {
		if p.X < min.X {
			min.X = p.X
		}
		if p.Y < min.Y {
			min.Y = p.Y
		}

		if p.X > max.X {
			max.X = p.X
		}
		if p.Y > max.Y {
			max.Y = p.Y
		}

	}

	return image.Rectangle{min, max}
}
