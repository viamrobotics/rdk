package rimage

import (
	"errors"
	"image"
	"image/color"
	"math"
)

// Input a vector in cartesian coordinates and return the vector in polar coordinates.
func getMagnitudeAndDirection(x, y float64) (float64, float64) {
	mag := math.Sqrt(x*x + y*y)
	dir := math.Atan2(y, x)
	return mag, dir
}

// Helper function for convolving matrices together, When used with i, dx := range makeRangeArray(n)
// i is the position within the kernel and dx gives the offset within the depth map.
// if length is even, then the origin is to the right of middle i.e. 4 -> {-2, -1, 0, 1}
func makeRangeArray(length int) []int {
	if length <= 0 {
		return make([]int, 0)
	}
	rangeArray := make([]int, length)
	var span int
	if length%2 == 0 {
		oddArr := makeRangeArray(length - 1)
		span = length / 2
		rangeArray = append([]int{-span}, oddArr...)
	} else {
		span = (length - 1) / 2
		for i := 0; i < span; i++ {
			rangeArray[length-1-i] = span - i
			rangeArray[i] = -span + i
		}
	}
	return rangeArray
}

func GaussianBlur(dm *DepthMap, sigma float64) *DepthMap {
	if sigma <= 0. {
		return dm
	}
	blurFilter := GaussianFilter(sigma)
	width, height := dm.Width(), dm.Height()
	outDM := NewEmptyDepthMap(width, height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			point := image.Point{x, y}
			val := blurFilter(point, dm)
			outDM.Set(point.X, point.Y, Depth(val))
		}
	}
	return outDM
}

func SmoothNonZeroData(ii *ImageWithDepth, spatialXVar, spatialYVar, colorVar, depthVar float64) (*ImageWithDepth, error) {
	if !ii.IsAligned() {
		return nil, errors.New("input ImageWithDepth is not aligned.")
	}
	// Use canny edges from
	width, height := ii.Width(), ii.Height()
	outDM := NewEmptyDepthMap(width, height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			outDM.Set(x, y, Depth(0))
		}
	}
	return MakeImageWithDepth(ii.Color, outDM, ii.IsAligned(), ii.CameraSystem()), nil
}

// DetectDepthEdges uses a Canny edge detector to find edges in a depth map and returns a
// grayscale image.
func (cd *CannyEdgeDetector) DetectDepthEdges(dmIn *DepthMap, blur float64) (*image.Gray, error) {
	var err error
	var dm *DepthMap
	if cd.preprocessImage {
		dm = GaussianBlur(dmIn, blur)
	} else {
		dm = dmIn
	}

	vectorField := ForwardGradientDepth(dm)
	dmMagnitude, dmDirection := vectorField.MagnitudeField(), vectorField.DirectionField()

	nms, err := GradientNonMaximumSuppressionC8(dmMagnitude, dmDirection)
	if err != nil {
		return nil, err
	}
	low, high, err := GetHysteresisThresholds(dmMagnitude, nms, cd.highRatio, cd.lowRatio)
	if err != nil {
		return nil, err
	}
	edges, err := EdgeHysteresisFiltering(dmMagnitude, low, high)
	if err != nil {
		return nil, err
	}
	return edges, nil
}

// Gradient functions

// SobelGradient takes in a DepthMap, approximates the gradient in the X and Y direction at every pixel
// creates a  vector in polar form, and returns a vector field.
func SobelDepthGradient(dm *DepthMap) VectorField2D {
	width, height := dm.Width(), dm.Height()
	maxMag := 0.0
	g := make([]Vec2D, 0, width*height)
	sobel := SobelDepthFilter()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// apply the Sobel Filter over a 3x3 square around each pixel
			point := image.Point{x, y}
			sX, sY := sobel(point, dm)
			mag, dir := getMagnitudeAndDirection(sX, sY)
			g = append(g, Vec2D{mag, dir})
			maxMag = math.Max(math.Abs(mag), maxMag)
		}
	}
	vf := VectorField2D{width, height, g, maxMag}
	return vf

}

// ForwardGradientDepth computes the forward gradients in the X and Y direction of a depth map
func ForwardGradientDepth(dm *DepthMap) VectorField2D {
	width, height := dm.Width(), dm.Height()
	maxMag := 0.0
	g := make([]Vec2D, 0, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			sX, sY := 0, 0
			if x+1 >= 0 && x+1 < width {
				sX = int(dm.GetDepth(x+1, y)) - int(dm.GetDepth(x, y))
			}
			if y+1 >= 0 && y+1 < height {
				sY = int(dm.GetDepth(x, y+1)) - int(dm.GetDepth(x, y))
			}
			mag, dir := getMagnitudeAndDirection(float64(sX), float64(sY))
			g = append(g, Vec2D{mag, dir})
			maxMag = math.Max(math.Abs(mag), maxMag)
		}
	}
	vf := VectorField2D{width, height, g, maxMag}
	return vf
}

// Missing data and hole inprinting

// MissingDepthData outputs a binary map where white represents where the holes in the depth image are
func MissingDepthData(dm *DepthMap) *image.Gray {
	holes := image.NewGray(image.Rect(0, 0, dm.Width(), dm.Height()))
	for y := 0; y < dm.Height(); y++ {
		for x := 0; x < dm.Width(); x++ {
			if dm.GetDepth(x, y) == 0 {
				holes.Set(x, y, color.Gray{255})
			}
		}
	}
	return holes
}

type direction image.Point

func eightPointInprint(x, y int, dm *DepthMap, edges *image.Gray) Depth {
	directions := []direction{
		{0, 1},   //up
		{0, -1},  //down
		{-1, 0},  //left
		{1, 0},   //right
		{-1, 1},  // upper-left
		{1, 1},   // upper-right
		{-1, -1}, // lower-left
		{1, -1},  //lower-right
	}
	valAvg := 0.0
	count := 0.0
	zero := color.Gray{0}
	for _, dir := range directions {
		val := 0
		i, j := x, y
		for val == 0 { // increment in the given direction until you reach a filled pixel
			i += dir.X
			j += dir.Y
			if !dm.In(i, j) { // skip if out of picture
				break
			}
			val = int(dm.GetDepth(i, j))
			if edges.At(i, j) != zero { // stop if we've reached an edge
				break
			}
		}
		if val != 0 {
			valAvg = (valAvg*count + float64(val)) / (count + 1.)
			count += 1.
		}
	}
	valAvg = math.Max(valAvg, 0.0) // depth cannot be zero
	return Depth(valAvg)
}

func DepthRayMarching(dm *DepthMap, edges *image.Gray) (*DepthMap, error) {
	// loop over pixels until finding an empty one
	// fill empty pixel with 8 direction average
	// // if the pixel is an edge, or you reach the border without finding a pixel, ignore it
	filledDM := NewEmptyDepthMap(dm.Width(), dm.Height())
	*filledDM = *dm
	for y := 0; y < dm.Height(); y++ {
		for x := 0; x < dm.Width(); x++ {
			d := filledDM.GetDepth(x, y)
			if d != 0 {
				filledDM.Set(x, y, d)
			} else {
				inprint := eightPointInprint(x, y, filledDM, edges)
				filledDM.Set(x, y, inprint)
			}
		}
	}
	return filledDM, nil
}

func getValidNeighbors(pt image.Point, dm *DepthMap, visited map[image.Point]bool) []image.Point {
	neighbors := make([]image.Point, 0, 4)
	directions := []direction{
		{0, 1},  //up
		{0, -1}, //down
		{-1, 0}, //left
		{1, 0},  //right
	}
	for _, dir := range directions {
		neighbor := image.Point{pt.X + dir.X, pt.Y + dir.Y}
		if dm.In(neighbor.X, neighbor.Y) && !visited[neighbor] && dm.Get(neighbor) != 0 {
			neighbors = append(neighbors, neighbor)
			visited[neighbor] = true
		}
	}
	return neighbors
}

func CleanDepthMap(dm *DepthMap, threshold int) {
	// Create a list of connected non-empty regions in depth map
	// only keeping the largest regions
	regionMap := make(map[int][]image.Point)
	visited := make(map[image.Point]bool)
	region := 0
	queue := make([]image.Point, 0)
	for y := 0; y < dm.Height(); y++ {
		for x := 0; x < dm.Width(); x++ {
			p := image.Point{x, y}
			if visited[p] || dm.Get(p) == 0 {
				continue
			}
			queue = append(queue, p)
			segment := []image.Point{}
			for len(queue) != 0 {
				// pop off element in queue and add to segment
				point := queue[0]
				segment = append(segment, point)
				queue = queue[1:]
				// get non-zero depth, non-visited, valid neighbors
				neighbors := getValidNeighbors(point, dm, visited)
				// add neighbors to queue
				queue = append(queue, neighbors...)
				//fmt.Printf("size: %d, points visited: %d\n", dm.Width()*dm.Height(), len(visited))
			}
			regionMap[region] = segment
			region++
		}
	}
	for _, seg := range regionMap {
		if len(seg) < threshold {
			for _, point := range seg {
				dm.Set(point.X, point.Y, Depth(0))
			}
		}
	}
	// Then do a morphological closing
	// closedDM, err := rimage.ClosingMorph(fixed.Depth, 5, 1)

	// smooth the data, first the non-edge regions, and then the edges
	// now find the segments of the hole regions, ignoring the largest holes, and classifying the remaining into edge/non-edge
	// Then fill the holes, the non-edges first, and then the edges
}
