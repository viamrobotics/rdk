package rimage

import (
	"fmt"
	"image"
	"math"

	"github.com/edaniels/golog"
)

// returns points suitable for calling warp on
func ImageAlign(img1Size image.Point, img1Points []image.Point,
	img2Size image.Point, img2Points []image.Point) ([]image.Point, []image.Point, error) {

	debug := true

	if len(img1Points) != 2 || len(img2Points) != 2 {
		return nil, nil, fmt.Errorf("need exactly 2 matching points")
	}

	expand := func(pts []image.Point, x bool) []image.Point {
		center := Center(pts, 100000)

		n := []image.Point{}
		for _, p := range pts {
			if x {
				dis := center.X - p.X
				newDis := int(float64(dis) * 1.1)
				if dis == newDis {
					newDis = dis * 2
				}

				n = append(n, image.Point{center.X - newDis, p.Y})
			} else {
				dis := center.Y - p.Y
				newDis := int(float64(dis) * 1.1)
				if dis == newDis {
					newDis = dis * 2
				}
				n = append(n, image.Point{p.X, center.Y - newDis})
			}

		}
		return n
	}

	fixPoints := func(pts []image.Point) []image.Point {
		r := BoundingBox(pts)
		return arrayToPoints([]image.Point{r.Min, r.Max})
	}

	// this only works for things on a multiple of 90 degrees apart, not arbitrary

	// firse we figure out if we are rotated 90 degrees or not to know which direction to expand
	colorAngle := PointAngle(img1Points[0], img1Points[1])
	depthAngle := PointAngle(img2Points[0], img2Points[1])

	if colorAngle < 0 {
		colorAngle += math.Pi
	}
	if depthAngle < 0 {
		depthAngle += math.Pi
	}

	colorAngle /= (math.Pi / 2)
	depthAngle /= (math.Pi / 2)

	rotated := false
	if colorAngle < 1 && depthAngle > 1 || colorAngle > 1 && depthAngle < 1 {
		rotated = true
	}

	if debug {
		golog.Global.Debugf("colorAngle: %v depthAngle: %v rotated: %v", colorAngle, depthAngle, rotated)
	}

	// now we expand in one direction
	for {
		c2 := expand(img1Points, true)
		d2 := expand(img2Points, !rotated)

		if !AllPointsIn(img1Size, c2) || !AllPointsIn(img2Size, d2) {
			break
		}
		img1Points = c2
		img2Points = d2
		if debug {
			golog.Global.Debugf("A: %v %v", img1Points, img2Points)
		}
	}

	// now we expand in the other direction
	for {
		c2 := expand(img1Points, false)
		d2 := expand(img2Points, rotated)

		if !AllPointsIn(img1Size, c2) || !AllPointsIn(img2Size, d2) {
			break
		}
		img1Points = c2
		img2Points = d2
		if debug {
			golog.Global.Debugf("B: %v %v", img1Points, img2Points)
		}
	}

	img1Points = fixPoints(img1Points)
	img2Points = fixPoints(img2Points)

	if rotated {
		// TODO(erh): handle flipped
		img2Points = append(img2Points[1:], img2Points[0])
	}

	return img1Points, img2Points, nil
}
