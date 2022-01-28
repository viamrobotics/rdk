package objectdetection

import (
	"image"

	"github.com/pkg/errors"

	"go.viam.com/rdk/rimage"
)

// validPixelFunc is a function that returns true if a pixel in an rimage.ImageWithDepth passes a certain criteria.
type validPixelFunc func(*rimage.ImageWithDepth, image.Point) bool

// connectedComponentDetector identifies objects in an image by merging neighbors that share similar properties.
// Based on some valid criteria, it will group the pixel into the current segment.
type connectedComponentDetector struct {
	valid validPixelFunc
}

// Inference takes in an image frame and returns the Detections found in the image.
func (ccd *connectedComponentDetector) Inference(img image.Image) ([]Detection, error) {
	// can use depth info if it is available
	iwd := rimage.ConvertToImageWithDepth(img)
	seen := make([]bool, iwd.Width()*iwd.Height())
	queue := []image.Point{}
	detections := []Detection{}
	for i := 0; i < iwd.Width(); i++ {
		for j := 0; j < iwd.Height(); j++ {
			pt := image.Point{i, j}
			indx := pt.Y*iwd.Width() + pt.X
			if seen[indx] {
				continue
			}
			if !ccd.valid(iwd, pt) {
				seen[indx] = true
				continue
			}
			queue = append(queue, pt)
			x0, y0, x1, y1 := pt.X, pt.Y, pt.X, pt.Y // the bounding box of the segment
			for len(queue) != 0 {
				newPt := queue[0]
				newIndx := newPt.Y*iwd.Width() + newPt.X
				seen[newIndx] = true
				queue = queue[1:]
				if newPt.X < x0 {
					x0 = newPt.X
				}
				if newPt.X > x1 {
					x1 = newPt.X
				}
				if newPt.Y < y0 {
					y0 = newPt.Y
				}
				if newPt.Y > y1 {
					y1 = newPt.Y
				}
				neighbors := ccd.getNeighbors(newPt, iwd, seen)
				queue = append(queue, neighbors...)
			}
			d := &detection2D{image.Rect(x0, y0, x1, y1), 1.0}
			detections = append(detections, d)
		}
	}
	return detections, nil
}

func (ccd *connectedComponentDetector) getNeighbors(pt image.Point, img *rimage.ImageWithDepth, seen []bool) []image.Point {
	bounds := img.Bounds()
	neighbors := make([]image.Point, 0, 4)
	fourPoints := []image.Point{{pt.X, pt.Y - 1}, {pt.X, pt.Y + 1}, {pt.X - 1, pt.Y}, {pt.X + 1, pt.Y}}
	for _, p := range fourPoints {
		indx := p.Y*bounds.Dx() + p.X
		if !p.In(bounds) || seen[indx] {
			continue
		}
		if ccd.valid(img, p) {
			neighbors = append(neighbors, p)
		}
		seen[indx] = true
	}
	return neighbors
}

///// Detectors /////

// NewColorDetector a detector that identifies objects based on color.
// It takes in a hue value between 0 and 360, and then defines a valid range around the hue of that color
// based on the tolerance. The color is considered valid if the pixel is between hue - tol < color < hue + tol.
func NewColorDetector(tol, hue float64) (Detector, error) {
	if tol > 1.0 || tol < 0.0 {
		return nil, errors.Errorf("tolerance must be between 0.0 and 1.0. Got %.5f", tol)
	}

	var valid validPixelFunc
	if tol == 1.0 {
		valid = makeValidColorFunction(0, 360)
	} else {
		tol = (tol / 2.) * 360.0 // change from percent to degrees
		hiValid := hue + tol
		if hiValid >= 360. {
			hiValid -= 360.
		}
		loValid := hue - tol
		if loValid < 0. {
			loValid += 360.
		}
		valid = makeValidColorFunction(loValid, hiValid)
	}
	cd := connectedComponentDetector{valid}
	return cd.Inference, nil
}

func makeValidColorFunction(loValid, hiValid float64) validPixelFunc {
	valid := func(v float64) bool { return v == loValid }
	if hiValid > loValid {
		valid = func(v float64) bool { return v <= hiValid && v >= loValid }
	} else if loValid > hiValid {
		valid = func(v float64) bool { return v <= hiValid || v >= loValid }
	}
	// create the ValidPixel function
	return func(img *rimage.ImageWithDepth, pt image.Point) bool {
		c := img.Color.Get(pt)
		h, s, v := c.HsvNormal()
		if s < .2 {
			return false
		}
		if v < 0.5 {
			return false
		}
		return valid(h)
	}
}
