package objectdetection

import (
	"image"
	"image/color"
)

// simpleDetector converts an image to gray and then finds the connected components above a certain size according to
// pixels below a certain threshold value
type simpleDetector struct {
	threshold int
	size      int
}

// NewSimpleDetector creates a detector useful for local testing purposes on the robot. Looks for dark objects in the image.
// It finds pixels below the set threshold, and only returns the connected components above the specified size.
func NewSimpleDetector(threshold, size int) Detector {
	return &simpleDetector{threshold, size}
}

// Inference takes in an image frame and returns the detection bounding boxes found in the image.
func (sd *simpleDetector) Inference(img image.Image) ([]*Detection, error) {
	seen := make(map[image.Point]bool)
	queue := []image.Point{}
	detections := []*Detection{}
	bounds := img.Bounds()
	for i := 0; i < bounds.Dx(); i++ {
		for j := 0; j < bounds.Dy(); j++ {
			pt := image.Point{i, j}
			if seen[pt] || !sd.pass(img.At(pt.X, pt.Y)) {
				seen[pt] = true
				continue
			}
			queue = append(queue, pt)
			x0, y0, x1, y1 := pt.X, pt.Y, pt.X, pt.Y // the bounding box of the segment
			for len(queue) != 0 {
				newPt := queue[0]
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
				seen[newPt] = true
				neighbors := sd.getNeighbors(newPt, img, seen)
				queue = append(queue, neighbors...)
			}
			d := &Detection{image.Rect(x0, y0, x1, y1), 1.0}
			if d.Area() >= sd.size {
				detections = append(detections, d)
			}
		}
	}
	return detections, nil
}

func (sd *simpleDetector) pass(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	return int(lum/256) < sd.threshold
}

func (sd *simpleDetector) getNeighbors(pt image.Point, img image.Image, seen map[image.Point]bool) []image.Point {
	bounds := img.Bounds()
	neighbors := make([]image.Point, 0, 4)
	fourPoints := []image.Point{{pt.X, pt.Y - 1}, {pt.X, pt.Y + 1}, {pt.X - 1, pt.Y}, {pt.X + 1, pt.Y}}
	for _, p := range fourPoints {
		if p.In(bounds) && !seen[p] && sd.pass(img.At(p.X, p.Y)) {
			neighbors = append(neighbors, p)
		}
		seen[p] = true
	}
	return neighbors
}
