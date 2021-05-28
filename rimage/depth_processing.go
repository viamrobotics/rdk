package rimage

import (
	"errors"
	"image"
	"image/color"
	"math"

	"go.viam.com/core/utils"
)

// PreprocessDepthMap applies data cleaning and smoothing procedures to an input imagewithdepth
func PreprocessDepthMap(iwd *ImageWithDepth) (*ImageWithDepth, error) {
	if !iwd.IsAligned() {
		return nil, errors.New("image with depth is not aligned. Cannot preprocess the depth map")
	}
	var err error
	// remove noisy data
	CleanDepthMap(iwd.Depth)
	// fill in small holes
	iwd.Depth, err = ClosingMorph(iwd.Depth, 5, 1)
	if err != nil {
		return nil, err
	}
	// fill in large holes using color info
	FillDepthMap(iwd)
	CleanDepthMap(iwd.Depth)
	// smooth the data
	iwd.Depth, err = OpeningMorph(iwd.Depth, 5, 1)
	return iwd, nil
}

// DetectDepthEdges uses a Canny edge detector to find edges in a depth map and returns a grayscale image of edges.
func (cd *CannyEdgeDetector) DetectDepthEdges(dmIn *DepthMap, blur float64) (*image.Gray, error) {
	var err error
	var dm *DepthMap
	if cd.preprocessImage {
		validPoints := MissingDepthData(dmIn)
		dm, err = SavitskyGolaySmoothing(dmIn, validPoints, int(blur), 3)
		if err != nil {
			return nil, err
		}
	} else {
		dm = dmIn
	}

	vectorField := ForwardDepthGradient(dm)
	dmMagnitude, dmDirection := vectorField.MagnitudeField(), vectorField.DirectionField()

	nms, err := GradientNonMaximumSuppressionC8(dmMagnitude, dmDirection)
	if err != nil {
		return nil, err
	}
	low, high, err := GetHysteresisThresholds(dmMagnitude, nms, cd.highRatio, cd.lowRatio)
	if err != nil {
		return nil, err
	}
	// low, high =
	edges, err := EdgeHysteresisFiltering(dmMagnitude, low, high)
	if err != nil {
		return nil, err
	}
	return edges, nil
}

// MissingDepthData outputs a binary map where white represents where data is, and black is where data is missing.
func MissingDepthData(dm *DepthMap) *image.Gray {
	nonHoles := image.NewGray(image.Rect(0, 0, dm.Width(), dm.Height()))
	for y := 0; y < dm.Height(); y++ {
		for x := 0; x < dm.Width(); x++ {
			if dm.GetDepth(x, y) != 0 {
				nonHoles.Set(x, y, color.Gray{255})
			}
		}
	}
	return nonHoles
}

// Smoothing Functions

// GaussianSmoothing smoothes a depth map affect by noise by using a weighted average of the pixel values in a window according
// to a gaussian distribution with a given sigma.
func GaussianSmoothing(dm *DepthMap, sigma float64) (*DepthMap, error) {
	width, height := dm.Width(), dm.Height()
	outDM := NewEmptyDepthMap(width, height)
	if sigma <= 0. {
		*outDM = *dm
		return outDM, nil
	}
	filter := gaussianFilter(sigma)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if dm.GetDepth(x, y) == 0 {
				continue
			}
			point := image.Point{x, y}
			val := filter(point, dm)
			outDM.Set(point.X, point.Y, Depth(val))
		}
	}
	return outDM, nil
}

// SavitskyGolaySmoothing smoothes a depth map affected by noise by using a least-squares fit to a 2D polynomial equation.
// radius determines the window of the smoothing, while polyOrder determines the order of the polynomial fit.
func SavitskyGolaySmoothing(dm *DepthMap, validPoints *image.Gray, radius, polyOrder int) (*DepthMap, error) {
	width, height := dm.Width(), dm.Height()
	outDM := NewEmptyDepthMap(width, height)
	if radius <= 0 || polyOrder <= 0 {
		*outDM = *dm
		return outDM, nil
	}
	filter, err := savitskyGolayFilter(radius, polyOrder)
	if err != nil {
		return nil, err
	}
	dmForConv := expandDepthMapForConvolution(dm, radius)
	zero := color.Gray{0}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if validPoints.At(x, y) == zero {
				continue
			}
			pointForConv := image.Point{x + radius, y + radius}
			val := filter(pointForConv, dmForConv)
			dmForConv.Set(x, y, Depth(val))
			outDM.Set(x, y, Depth(val))
		}
	}
	return outDM, nil
}

// JointBilateralSmoothing smoothes a depth map affected by noise by using the product of two gaussian filters,
// one based on spatial distance, and the other based on depth differences. depthSigma essentially sets a threshold to not
// smooth across large differences in depth.
func JointBilateralSmoothing(dm *DepthMap, spatialSigma, depthSigma float64) (*DepthMap, error) {
	filter := jointBilateralFilter(spatialSigma, depthSigma)
	width, height := dm.Width(), dm.Height()
	outDM := NewEmptyDepthMap(width, height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if dm.GetDepth(x, y) == 0 {
				continue
			}
			point := image.Point{x, y}
			val := filter(point, dm)
			outDM.Set(point.X, point.Y, Depth(val))
		}
	}
	return outDM, nil
}

// expandDepthMapForConvolution pads an input depth map on every side with a mirror image of the data. This is so evaluation
// of the convolution at the borders will happen smoothly and will not created artifacts of sharp edges.
func expandDepthMapForConvolution(dm *DepthMap, radius int) *DepthMap {
	if radius <= 0 {
		return dm
	}
	width, height := dm.Width(), dm.Height()
	outDM := NewEmptyDepthMap(width+2*radius, height+2*radius)
	for y := -radius; y < height+radius; y++ {
		for x := -radius; x < width+radius; x++ {
			outDM.Set(x+radius, y+radius, dm.GetDepth(utils.AbsInt(x%width), utils.AbsInt(y%height)))
		}
	}
	return outDM
}

// Gradient functions

// SobelDepthGradient computes the approximate gradients in the X and Y direction of a depth map and returns a vector field.
func SobelDepthGradient(dm *DepthMap) VectorField2D {
	width, height := dm.Width(), dm.Height()
	maxMag := 0.0
	g := make([]Vec2D, 0, width*height)
	sobel := sobelDepthFilter()
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

// ForwardDepthGradient computes the forward gradients in the X and Y direction of a depth map and returns a vector field.
func ForwardDepthGradient(dm *DepthMap) VectorField2D {
	width, height := dm.Width(), dm.Height()
	maxMag := 0.0
	g := make([]Vec2D, 0, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			sX, sY := 0, 0
			if dm.GetDepth(x, y) != 0 {
				if x+1 >= 0 && x+1 < width && dm.GetDepth(x+1, y) != 0 {
					sX = int(dm.GetDepth(x+1, y)) - int(dm.GetDepth(x, y))
				}
				if y+1 >= 0 && y+1 < height && dm.GetDepth(x, y+1) != 0 {
					sY = int(dm.GetDepth(x, y+1)) - int(dm.GetDepth(x, y))
				}
			}
			mag, dir := getMagnitudeAndDirection(float64(sX), float64(sY))
			g = append(g, Vec2D{mag, dir})
			maxMag = math.Max(math.Abs(mag), maxMag)
		}
	}
	vf := VectorField2D{width, height, g, maxMag}
	return vf
}

// Filling and cleaning depth map functions

// CleanDepthMap removes the connected regions of data below a certain size thershold.
func CleanDepthMap(dm *DepthMap) {
	validData := MissingDepthData(dm)
	regionMap := segmentBinaryImage(validData)
	for _, seg := range regionMap {
		avgDepth := averageDepthInSegment(seg, dm)
		threshold := thresholdFromDepth(avgDepth, dm.Width()*dm.Height())
		if len(seg) < threshold {
			for point := range seg {
				dm.Set(point.X, point.Y, Depth(0))
			}
		}
	}
}

// FillDepthMap finds regions of connected missing data, and for those below a certain size, fills them in with
// an average of the surrounding pixels by using 8-point ray-marching.
func FillDepthMap(iwd *ImageWithDepth) {
	validData := MissingDepthData(iwd.Depth)
	missingData := invertGrayImage(validData)
	holeMap := segmentBinaryImage(missingData)
	for _, seg := range holeMap {
		avgDepth := averageDepthAroundHole(seg, iwd.Depth)
		threshold := thresholdFromDepth(avgDepth, iwd.Width()*iwd.Height())
		if len(seg) < threshold {
			for point := range seg {
				val := depthRayMarching(point.X, point.Y, 16, sixteenPoints, iwd)
				iwd.Depth.Set(point.X, point.Y, val)
			}
		}
	}
}

// limits inpainting/cleaning to holes of a specific size. Farther distance means the same pixel size represents a larger area.
// It might be better to make it a real function of depth, right now just split into regions of close, middle, far based on distance
// in mm and make thresholds based on proportion of the image resolution.
func thresholdFromDepth(depth float64, imgResolution int) int {
	res := float64(imgResolution)
	switch {
	case depth < 500.:
		return int(0.05 * res)
	case depth >= 500. && depth < 4000.:
		return int(0.005 * res)
	default:
		return int(0.0005 * res)
	}
}

// get the average depth within the segment. assumes segment has only valid points, and no points are out of bounds.
func averageDepthInSegment(segment map[image.Point]bool, dm *DepthMap) float64 {
	sum, count := 0.0, 0.0
	for point := range segment {
		d := float64(dm.GetDepth(point.X, point.Y))
		sum += d
		count++
	}
	return sum / count
}

// calculate the average depth inside the hole as the average of the border of the hole
func averageDepthAroundHole(segment map[image.Point]bool, dm *DepthMap) float64 {
	directions := []image.Point{
		{0, 1},  //up
		{0, -1}, //down
		{-1, 0}, //left
		{1, 0},  //right
	}
	sum := 0.0
	count := 0.0
	visited := make(map[image.Point]bool) // don't double-count border pixels
	for hole := range segment {
		for _, dir := range directions {
			point := image.Point{hole.X + dir.X, hole.Y + dir.Y}
			if !dm.Contains(point.X, point.Y) {
				continue
			}
			d := float64(dm.GetDepth(point.X, point.Y))
			if d != 0. && !visited[point] {
				sum += d
				count++
				visited[point] = true
			}
		}
	}
	return sum / count
}

// directions for ray-marching
var (
	eightPoints = []image.Point{
		{0, 1},   //up
		{0, -1},  //down
		{-1, 0},  //left
		{1, 0},   //right
		{-1, 1},  // upper-left
		{1, 1},   // upper-right
		{-1, -1}, // lower-left
		{1, -1},  //lower-right
	}

	sixteenPoints = []image.Point{
		{0, 2}, {0, -2}, {-2, 0}, {2, 0},
		{-2, 2}, {2, 2}, {-2, -2}, {2, -2},
		{-2, 1}, {-1, 2}, {1, 2}, {2, 1},
		{-2, -1}, {-1, -2}, {1, -2}, {2, -1},
	}
)

// depthRayMarching uses multi-point ray-marching to fill in missing data. It marches out in 8 directions from the missing pixel until
// it encounters a pixel with data, and then averages the values of the non-zero pixels it finds to fill the missing value.
// Uses color info to help. If the color changes "too much" between pixels (exponential weighing), the depth will contribute
// less to the average.
func depthRayMarching(x, y, iterations int, directions []image.Point, iwd *ImageWithDepth) Depth {
	centerColor := iwd.Color.GetXY(x, y)
	depthAvg := 0.0
	weightTot := 0.0
	for _, dir := range directions {
		depth := 0.0
		count := 0.0
		var col Color
		i, j := x, y
		for iter := 0; iter < iterations; iter++ { // average in the same direction
			depthIter := 0.0
			for depthIter == 0.0 { // increment in the given direction until you reach a filled pixel
				i += dir.X
				j += dir.Y
				if !iwd.Depth.Contains(i, j) { // skip if out of picture bounds
					break
				}
				depthIter = float64(iwd.Depth.GetDepth(i, j))
				col = iwd.Color.GetXY(i, j)
			}
			if depthIter != 0.0 {
				colorDistance := centerColor.DistanceLab(col) // 0.0 is same color, >=1.0 is extremely different
				weight := math.Exp(-100.0 * colorDistance)
				depthAvg = (depthAvg*weightTot + (depth/count)*weight) / (weightTot + weight)
				weightTot += weight
			}
		}
	}
	depthAvg = math.Max(depthAvg, 0.0) // depth cannot be zero
	return Depth(depthAvg)
}

// getValidNeighbors uses both a B+W image of valid points as well as a map of points already visited to determine which points should
// be added in the queue for a breadth-first search.
func getValidNeighbors(pt image.Point, valid *image.Gray, visited map[image.Point]bool) []image.Point {
	neighbors := make([]image.Point, 0, 4)
	directions := []image.Point{
		{0, 1},  //up
		{0, -1}, //down
		{-1, 0}, //left
		{1, 0},  //right
	}
	zero := color.Gray{0}
	for _, dir := range directions {
		neighbor := image.Point{pt.X + dir.X, pt.Y + dir.Y}
		if neighbor.In(valid.Bounds()) && !visited[neighbor] && valid.At(neighbor.X, neighbor.Y) != zero {
			neighbors = append(neighbors, neighbor)
			visited[neighbor] = true
		}
	}
	return neighbors
}

// segmentBinaryImage does a bredth-first search on a black and white image and splits the non-connected white regions
// into different regions.
func segmentBinaryImage(img *image.Gray) map[int]map[image.Point]bool {
	regionMap := make(map[int]map[image.Point]bool)
	visited := make(map[image.Point]bool)
	region := 0
	queue := make([]image.Point, 0)
	height, width := img.Bounds().Dy(), img.Bounds().Dx()
	zero := color.Gray{0}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			p := image.Point{x, y}
			if visited[p] || img.At(x, y) == zero {
				continue
			}
			queue = append(queue, p)
			segment := make(map[image.Point]bool)
			for len(queue) != 0 {
				// pop off element in queue and add to segment
				point := queue[0]
				segment[point] = true
				queue = queue[1:]
				// get non-visited, valid (i.e. non-zero) neighbors
				neighbors := getValidNeighbors(point, img, visited)
				// add neighbors to queue
				queue = append(queue, neighbors...)
			}
			regionMap[region] = segment
			region++
		}
	}
	return regionMap
}

// invertGrayImage produces a negated version of the input image
func invertGrayImage(img *image.Gray) *image.Gray {
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	dst := image.NewGray(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := img.At(x, y).(color.Gray)
			dst.Set(x, y, color.Gray{255 - val.Y})
		}
	}
	return dst
}
