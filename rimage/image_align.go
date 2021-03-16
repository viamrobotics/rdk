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
	// crop the four sides of the images so they enclose the same area
	// if one image is rotated, it's assumed it's the second image.
	// dist A/1 must be longer than dist B/2

	var distA, distB, dist1, dist2 int
	var err error
	var trimTop, trimBot, trimRight, trimLeft int
	var trimFirstTop, trimFirstBot, trimFirstRight, trimFirstLeft int
	// trim top (rotated 90: trim from right)
	distA, distB = img1Points[0].Y, img1Points[1].Y
	if rotated {
		dist1, dist2 = img2Size.X-img2Points[0].X, img2Size.X-img2Points[1].X
	} else {
		dist1, dist2 = img2Points[0].Y, img2Points[1].Y
	}
	trimTop, trimFirstTop, err = trim(distA, distB, dist1, dist2)
	if err != nil {
		golog.Global.Debugf("image_align error: %s", err)
	}
	// trim bottom (rotated 90: trim from left)
	distA, distB = img1Size.Y-img1Points[1].Y, img1Size.Y-img1Points[0].Y
	if rotated {
		dist1, dist2 = img2Points[1].X, img2Points[0].X
	} else {
		dist1, dist2 = img2Size.Y-img2Points[1].Y, img2Size.Y-img2Points[0].Y
	}
	trimBot, trimFirstBot, err = trim(distA, distB, dist1, dist2)
	if err != nil {
		golog.Global.Debugf("image_align error: %s", err)
	}
	// trim left (rotated 90: trim from top)
	distA, distB = img1Points[1].X, img1Points[0].X
	if rotated {
		dist1, dist2 = img2Points[1].Y, img2Points[0].Y
	} else {
		dist1, dist2 = img2Points[1].X, img2Points[0].X
	}
	trimLeft, trimFirstLeft, err = trim(distA, distB, dist1, dist2)
	if err != nil {
		golog.Global.Debugf("image_align error: %s", err)
	}
	// trim right (rotated 90: trim from bottom)
	distA, distB = img1Size.X-img1Points[0].X, img1Size.X-img1Points[1].X
	if rotated {
		dist1, dist2 = img2Size.Y-img2Points[0].Y, img2Size.Y-img2Points[1].Y
	} else {
		dist1, dist2 = img2Size.X-img2Points[0].X, img2Size.X-img2Points[1].X
	}
	trimRight, trimFirstRight, err = trim(distA, distB, dist1, dist2)
	if err != nil {
		golog.Global.Debugf("error: %s", err)
	}
	// Set the crop coorindates for the images
	img1Points[0].X, img1Points[0].Y = trimLeft*trimFirstLeft+1, trimTop*trimFirstTop+1
	img1Points[1].X, img1Points[1].Y = img1Size.X-trimRight*trimFirstRight-1, img1Size.Y-trimBot*trimFirstBot-1
	if rotated {
		img2Points[0].X, img2Points[0].Y = trimBot*(1-trimFirstBot)+1, trimLeft*(1-trimFirstLeft)+1
		img2Points[1].X, img2Points[1].Y = img2Size.X-trimTop*(1-trimFirstTop)-1, img2Size.Y-trimRight*(1-trimFirstRight)-1
	} else {
		img2Points[0].X, img2Points[0].Y = trimLeft*(1-trimFirstLeft)+1, trimTop*(1-trimFirstTop)+1
		img2Points[1].X, img2Points[1].Y = img2Size.X-trimRight*(1-trimFirstRight)-1, img2Size.Y-trimBot*(1-trimFirstBot)-1
	}

	if debug {
		golog.Global.Debugf("img1 size: %v img1 points: %v", img1Size, img1Points)
		golog.Global.Debugf("img2 size: %v img2 points: %v", img2Size, img2Points)
		if !AllPointsIn(img1Size, img1Points) || !AllPointsIn(img2Size, img2Points) {
			golog.Global.Debugf("Points are not contained in the images: %v %v", AllPointsIn(img1Size, img1Points), AllPointsIn(img2Size, img2Points))
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

func trim(dA, dB, d1, d2 int) (int, int, error) {
	// required: distA > distB, and dist1 > dist2
	if (dA < dB) || (d2 > d1) {
		return -1, -1, fmt.Errorf("%v must be less than %v, and %v must be less than %v", dB, dA, d2, d1)
	}
	distA, distB := float64(dA), float64(dB)
	dist1, dist2 := float64(d1), float64(d2)
	// returns whether to trim the first or second image, and by how much.
	var trimFirst int
	var trimAmount float64
	// are the ratios equal already?
	const EqualityThreshold = 1e-5
	ratioA := distA / distB
	ratio1 := dist1 / dist2
	if math.Abs(ratioA-ratio1) <= EqualityThreshold {
		return int(trimAmount), trimFirst, nil
	}
	// the ratio that is bigger is the one to match to.
	if ratioA > ratio1 {
		trimFirst = 0
		trimAmount = (distA*dist2 - distB*dist1) / (distA - distB)
		return int(trimAmount), trimFirst, nil
	} else {
		trimFirst = 1
		trimAmount = (dist1*distB - dist2*distA) / (dist1 - dist2)
		return int(trimAmount), trimFirst, nil
	}

}

func expand(pts []image.Point, x bool) []image.Point {
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

func getScale(img1Points, img2Points []image.Point, is_rotated bool, get_x_scale bool) float64 {
	var s float64
	if get_x_scale {
		if is_rotated {
			s = math.Abs(float64(img1Points[0].X-img1Points[1].X)) / math.Abs(float64(img2Points[0].Y-img2Points[1].Y))
		} else {
			s = math.Abs(float64(img1Points[0].X-img1Points[1].X)) / math.Abs(float64(img2Points[0].X-img2Points[1].X))
		}
	} else {
		if is_rotated {
			s = math.Abs(float64(img1Points[0].Y-img1Points[1].Y)) / math.Abs(float64(img2Points[0].X-img2Points[1].X))
		} else {
			s = math.Abs(float64(img1Points[0].Y-img1Points[1].Y)) / math.Abs(float64(img2Points[0].Y-img2Points[1].Y))
		}
	}
	return s
}
