package objectdetection

import (
	"image"
)

// validPixelFunc is a function that returns true if a pixel in an rimage.ImageWithDepth passes a certain criteria.
type validPixelFunc func(image.Image, image.Point) bool

// connectedComponentDetector identifies objects in an image by merging neighbors that share similar properties.
// Based on some valid criteria, it will group the pixel into the current segment.
type connectedComponentDetector struct {
	valid validPixelFunc
	label string
}

// Inference takes in an image frame and returns the Detections found in the image.
func (ccd *connectedComponentDetector) Inference(img image.Image) ([]Detection, error) {
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	seen := make([]bool, width*height)
	queue := []image.Point{}
	detections := []Detection{}
	for i := 0; i < width; i++ {
		for j := 0; j < height; j++ {
			pt := image.Point{i, j}
			indx := pt.Y*width + pt.X
			if seen[indx] {
				continue
			}
			if !ccd.valid(img, pt) {
				seen[indx] = true
				continue
			}
			queue = append(queue, pt)
			x0, y0, x1, y1 := pt.X, pt.Y, pt.X, pt.Y // the bounding box of the segment
			for len(queue) != 0 {
				newPt := queue[0]
				newIndx := newPt.Y*width + newPt.X
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
				neighbors := ccd.getNeighbors(newPt, img, seen)
				queue = append(queue, neighbors...)
			}
			d := &detection2D{image.Rect(x0, y0, x1, y1), 1.0, ccd.label}
			detections = append(detections, d)
		}
	}
	return detections, nil
}

func (ccd *connectedComponentDetector) getNeighbors(pt image.Point, img image.Image, seen []bool) []image.Point {
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
