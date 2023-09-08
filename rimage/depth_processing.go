//go:build cgo
package rimage

import (
	"image"
	"image/color"
	"math"

	"github.com/golang/geo/r2"

	"go.viam.com/rdk/utils"
)

// PreprocessDepthMap applies data cleaning and smoothing procedures to an input depth map, and optional rgb image.
// It is assumed the depth map and rgb image are aligned.
func PreprocessDepthMap(dm *DepthMap, img *Image) (*DepthMap, error) {
	var err error
	// remove noisy data
	CleanDepthMap(dm)
	// fill in small holes
	dm, err = ClosingMorph(dm, 5, 1)
	if err != nil {
		return nil, err
	}
	// fill in large holes using color info
	if img != nil {
		dm, err = FillDepthMap(dm, img)
		if err != nil {
			return nil, err
		}
	}
	// smooth the sharp edges out
	dm, err = OpeningMorph(dm, 5, 1)
	if err != nil {
		return nil, err
	}
	return dm, nil
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

// BilinearInterpolationDepth approximates the Depth value between pixels according to a bilinear
// interpolation. A nil return value means the interpolation is out of bounds.
func BilinearInterpolationDepth(pt r2.Point, dm *DepthMap) *Depth {
	width, height := float64(dm.Width()), float64(dm.Height())
	if pt.X < 0 || pt.Y < 0 || pt.X > width-1 || pt.Y > height-1 { // point out of bounds - skip it
		return nil
	}
	xmin := int(math.Floor(pt.X))
	xmax := int(math.Ceil(pt.X))
	ymin := int(math.Floor(pt.Y))
	ymax := int(math.Ceil(pt.Y))
	// get depth values
	d00 := float64(dm.GetDepth(xmin, ymin))
	d10 := float64(dm.GetDepth(xmax, ymin))
	d01 := float64(dm.GetDepth(xmin, ymax))
	d11 := float64(dm.GetDepth(xmax, ymax))
	// calculate weights
	area := float64((xmax - xmin) * (ymax - ymin))
	if area == 0.0 { // exactly on a pixel
		result := dm.GetDepth(int(pt.X), int(pt.Y))
		return &result
	}
	w00 := ((float64(xmax) - pt.X) * (float64(ymax) - pt.Y)) / area
	w10 := ((pt.X - float64(xmin)) * (float64(ymax) - pt.Y)) / area
	w01 := ((float64(xmax) - pt.X) * (pt.Y - float64(ymin))) / area
	w11 := ((pt.X - float64(xmin)) * (pt.Y - float64(ymin))) / area

	result := Depth(math.Round(d00*w00 + d01*w01 + d10*w10 + d11*w11))
	return &result
}

// NearestNeighborDepth takes the value of the closest point to the intermediate pixel.
func NearestNeighborDepth(pt r2.Point, dm *DepthMap) *Depth {
	width, height := float64(dm.Width()), float64(dm.Height())
	if pt.X < 0 || pt.Y < 0 || pt.X > width-1 || pt.Y > height-1 { // point out of bounds - skip it
		return nil
	}
	x := int(math.Round(pt.X))
	y := int(math.Round(pt.Y))
	// get depth value
	result := dm.GetDepth(x, y)
	return &result
}

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

// Cleaning depth map functions

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

// get the average depth within the segment. Assumes segment has only valid points. Checks if points are out of bounds.
func averageDepthInSegment(segment map[image.Point]bool, dm *DepthMap) float64 {
	sum, count := 0.0, 0.0
	for point := range segment {
		if !point.In(dm.Bounds()) {
			continue
		}
		d := float64(dm.GetDepth(point.X, point.Y))
		sum += d
		count++
	}
	return sum / count
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

// getValidNeighbors uses both a B+W image of valid points as well as a map of points already visited to determine which points should
// be added in the queue for a breadth-first search.
func getValidNeighbors(pt image.Point, valid *image.Gray, visited map[image.Point]bool) []image.Point {
	neighbors := make([]image.Point, 0, 4)
	directions := []image.Point{
		{0, 1},  // up
		{0, -1}, // down
		{-1, 0}, // left
		{1, 0},  // right
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

// invertGrayImage produces a negated version of the input image.
func invertGrayImage(img *image.Gray) *image.Gray {
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	dst := image.NewGray(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pix := img.At(x, y)
			val, err := utils.AssertType[color.Gray](pix)
			if err != nil {
				panic(err)
			}
			dst.Set(x, y, color.Gray{255 - val.Y})
		}
	}
	return dst
}
