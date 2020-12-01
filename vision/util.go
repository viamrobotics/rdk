package vision

import (
	"image"

	"github.com/echolabsinc/robotcore/rcutil"
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

	// TODO: this should be about coniguous, not distance

	numPoints := 0
	box := image.Rectangle{image.Point{1000000, 100000}, image.Point{0, 0}}
	for _, p := range contour {
		if rcutil.AbsInt(p.X-weightedMiddle.X) > maxDiff || rcutil.AbsInt(p.Y-weightedMiddle.Y) > maxDiff {
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
