package objectdetection

import (
	"image"

	"go.viam.com/rdk/rimage"
)

// colorDetector identifies objects of a certain color in the scene. Based on hue, currently cannot do pure black/white objects.
type colorDetector struct {
	valid func(float64) bool
}

// NewColorDetector a detector that identifies objects based on color.
// It takes in a color, converts it to HSV, and then defines a valid range around the hue of that color
// based on the tolerance. The color is considered valid if the pixel is between hue - tol < color < hue + tol.
func NewColorDetector(tol float64, c rimage.Color) Detector {
	h, _, _ := c.HsvNormal()
	hiValid := h + tol
	if hiValid >= 360 {
		hiValid -= 360
	}
	loValid := h - tol
	if loValid < 0 {
		loValid += 360
	}
	valid := func(v float64) bool { return v == h }
	if hiValid > loValid {
		valid = func(v float64) bool { return v < hiValid && v > loValid }
	} else if loValid > hiValid {
		valid = func(v float64) bool { return v < hiValid || v > loValid }
	}
	cd := colorDetector{valid}
	return cd.Inference
}

// Inference takes in an image frame and returns the detection bounding boxes found in the image.
func (cd *colorDetector) Inference(img image.Image) ([]Detection, error) {
	rimg := rimage.ConvertImage(img)
	seen := make([]bool, rimg.Width()*rimg.Height())
	queue := []image.Point{}
	detections := []Detection{}
	for i := 0; i < rimg.Width(); i++ {
		for j := 0; j < rimg.Height(); j++ {
			pt := image.Point{i, j}
			indx := pt.Y*rimg.Width() + pt.X
			if seen[indx] {
				continue
			}
			if !cd.pass(rimg.Get(pt)) {
				seen[indx] = true
				continue
			}
			queue = append(queue, pt)
			x0, y0, x1, y1 := pt.X, pt.Y, pt.X, pt.Y // the bounding box of the segment
			for len(queue) != 0 {
				newPt := queue[0]
				newIndx := newPt.Y*rimg.Width() + newPt.X
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
				neighbors := cd.getNeighbors(newPt, rimg, seen)
				queue = append(queue, neighbors...)
			}
			d := &detection2D{image.Rect(x0, y0, x1, y1), 1.0}
			detections = append(detections, d)
		}
	}
	return detections, nil
}

func (cd *colorDetector) pass(c rimage.Color) bool {
	h, s, v := c.HsvNormal()
	if s < .2 {
		return false
	}
	if v < 0.5 {
		return false
	}
	return cd.valid(h)
}

func (cd *colorDetector) getNeighbors(pt image.Point, img *rimage.Image, seen []bool) []image.Point {
	bounds := img.Bounds()
	neighbors := make([]image.Point, 0, 4)
	fourPoints := []image.Point{{pt.X, pt.Y - 1}, {pt.X, pt.Y + 1}, {pt.X - 1, pt.Y}, {pt.X + 1, pt.Y}}
	for _, p := range fourPoints {
		indx := p.Y*bounds.Dx() + p.X
		if !p.In(bounds) || seen[indx] {
			continue
		}
		if cd.pass(img.Get(p)) {
			neighbors = append(neighbors, p)
		}
		seen[indx] = true
	}
	return neighbors
}
