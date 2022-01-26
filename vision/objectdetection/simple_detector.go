package objectdetection

import (
	"image"

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
			if !sd.pass(rimg.Get(pt)) {
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
				neighbors := sd.getNeighbors(newPt, rimg, seen)
				queue = append(queue, neighbors...)
			}
			d := &detection2D{image.Rect(x0, y0, x1, y1), 1.0}
			detections = append(detections, d)
		}
	}
	return detections, nil
}

func (sd *simpleDetector) pass(c rimage.Color) bool {
	lum := rimage.Luminance(c)
	return lum < sd.threshold
}

func (sd *simpleDetector) getNeighbors(pt image.Point, img *rimage.Image, seen []bool) []image.Point {
	bounds := img.Bounds()
	neighbors := make([]image.Point, 0, 4)
	fourPoints := []image.Point{{pt.X, pt.Y - 1}, {pt.X, pt.Y + 1}, {pt.X - 1, pt.Y}, {pt.X + 1, pt.Y}}
	for _, p := range fourPoints {
		indx := p.Y*bounds.Dx() + p.X
		if !p.In(bounds) || seen[indx] {
			continue
		}
		if sd.pass(img.Get(p)) {
			neighbors = append(neighbors, p)
		}
		seen[indx] = true
	}
	return neighbors
}
