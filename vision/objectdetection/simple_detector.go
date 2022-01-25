package objectdetection

import (
	"image"
	"image/color"

	"go.viam.com/rdk/rimage"
)

// simpleDetector converts an image to gray and then finds the connected components with values below a certain
// luminance threshold. threshold is between 0.0 and 256.0, with 256.0 being white, and 0.0 being black.
type simpleDetector struct {
	threshold float64
}

// NewSimpleDetector creates a detector useful for local testing purposes on the robot. Looks for dark objects in the image.
// It finds pixels below the set threshold, and returns bounding box around the connected components.
func NewSimpleDetector(threshold float64) Detector {
	sd := simpleDetector{threshold}
	return sd.Inference
}

// Inference takes in an image frame and returns the detection bounding boxes found in the image.
func (sd *simpleDetector) Inference(img image.Image) ([]Detection, error) {
	seen := make(map[image.Point]bool)
	queue := []image.Point{}
	detections := []Detection{}
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
			d := &detection2D{image.Rect(x0, y0, x1, y1), 1.0}
			detections = append(detections, d)
		}
	}
	return detections, nil
}

func (sd *simpleDetector) pass(c color.Color) bool {
	lum := rimage.Luminance(rimage.NewColorFromColor(c))
	return lum < sd.threshold
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
