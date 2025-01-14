package objectdetection

import (
	"context"
	"image"
)

// validPixelFunc is a function that returns true if a pixel in an image.Image passes a certain criteria.
type validPixelFunc func(image.Image, image.Point) bool

// connectedComponentDetector identifies objects in an image by merging neighbors that share similar properties.
// Based on some valid criteria, it will group the pixel into the current segment.
type connectedComponentDetector struct {
	valid validPixelFunc
	label string
}

// Inference takes in an image frame and returns the Detections found in the image.
func (ccd *connectedComponentDetector) Inference(ctx context.Context, img image.Image) ([]Detection, error) {
	detections := []Detection{}
	_, rectangles := ConnectedComponents(img, ccd.valid)
	for _, rectangle := range rectangles {
		detections = append(detections, NewDetection(rectangle, 1, ccd.label))
	}
	return detections, nil
}

// ConnectedComponents takes in an image frame and a function that determines if a point in the image is valid and returns two slices.
// The first returned slice is a 2D slice of Points, with each 1D slice corresponding to a contiguous set of valid Points within the image.
// The second returned slice contains Rectangles, each of which constitutes the bounding box for the set of valid Points at the same index.
func ConnectedComponents(img image.Image, isValid func(image.Image, image.Point) bool) ([][]image.Point, []image.Rectangle) {
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	seen := make([]bool, width*height)
	rectangles := []image.Rectangle{}
	clusters := [][]image.Point{}

	getNeighbors := func(pt image.Point) []image.Point {
		bounds := img.Bounds()
		neighbors := make([]image.Point, 0, 4)
		fourPoints := []image.Point{{pt.X, pt.Y - 1}, {pt.X, pt.Y + 1}, {pt.X - 1, pt.Y}, {pt.X + 1, pt.Y}}
		for _, p := range fourPoints {
			indx := p.Y*bounds.Dx() + p.X
			if !p.In(bounds) || seen[indx] {
				continue
			}
			if isValid(img, p) {
				neighbors = append(neighbors, p)
			}
			seen[indx] = true
		}
		return neighbors
	}

	for i := 0; i < width; i++ {
		for j := 0; j < height; j++ {
			pt := image.Point{i, j}
			indx := pt.Y*width + pt.X
			if seen[indx] {
				continue
			}
			if !isValid(img, pt) {
				seen[indx] = true
				continue
			}
			cluster := []image.Point{pt}
			x0, y0, x1, y1 := pt.X, pt.Y, pt.X, pt.Y // the bounding box of the segment
			for k := 0; k < len(cluster); k++ {
				newPt := cluster[k]
				newIndx := newPt.Y*width + newPt.X
				seen[newIndx] = true
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
				neighbors := getNeighbors(newPt)
				cluster = append(cluster, neighbors...)
			}
			rectangles = append(rectangles, image.Rect(x0, y0, x1, y1))
			clusters = append(clusters, cluster)
		}
	}
	return clusters, rectangles
}
