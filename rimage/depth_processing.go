package rimage

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"go.viam.com/core/utils"
)

// PreprocessDepthMap applies data cleaning and smoothing procedures to an input depth map. Copies the data in order to leave
// the original input depth map unmodified.
func PreprocessDepthMap(dm *DepthMap) (*DepthMap, error) {
	var err error
	// copy data into a new depthmap
	dst := make([]Depth, dm.Width()*dm.Height())
	copy(dst, dm.data)
	outDM := &DepthMap{dm.width, dm.height, dst}
	// remove noisy data
	CleanDepthMap(outDM, 500)
	// fill in small holes
	outDM, err = ClosingMorph(outDM, 5, 1)
	if err != nil {
		return nil, err
	}
	// fill in large holes
	FillDepthMap(outDM)
	// smooth the data
	outDM = GaussianSmoothing(outDM, 2)
	/*
		validPoints := MissingDepthData(outDM)
		err = SavitskyGolaySmoothing(outDM, outDM, validPoints, 3, 3)
		if err != nil {
			return nil, err
		}
	*/
	return outDM, nil
}

// DetectDepthEdges uses a Canny edge detector to find edges in a depth map and returns a grayscale image of edges.
func (cd *CannyEdgeDetector) DetectDepthEdges(dmIn *DepthMap, blur float64) (*image.Gray, error) {
	var err error
	var dm *DepthMap
	if cd.preprocessImage {
		dm = GaussianSmoothing(dmIn, blur)
	} else {
		dm = dmIn
	}

	vectorField := ForwardDepthGradient(dm)
	//vectorField := SobelDepthGradient(dm)
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

// Smoothing Functions

func SmoothWithFilter(dm *DepthMap, filter func(p image.Point, d *DepthMap) float64) *DepthMap {
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
	return outDM
}

// GaussianSmoothing smoothes a depth map affect by noise by using a weighted average of the pixel values in a window according
// to a gaussian distribution with a given sigma.
func GaussianSmoothing(dm *DepthMap, sigma float64) *DepthMap {
	if sigma <= 0. {
		return dm
	}
	filter := GaussianFilter(sigma)
	return SmoothWithFilter(dm, filter)
}

// SavistkyGolaySmoothing smoothes a depth map affected by noise by using a least-squares fit to a 2D polynomial equation.
// radius determines the window of the smoothing, while polyOrder determines the order of the polynomial fit.
func SavitskyGolaySmoothing(dm, outDM *DepthMap, validPoints *image.Gray, radius, polyOrder int) error {
	if radius <= 0 || polyOrder <= 0 {
		outDM = dm
		return nil
	}
	filter, err := SavitskyGolayFilter(radius, polyOrder)
	if err != nil {
		return err
	}
	width, height := dm.Width(), dm.Height()
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
	return nil
}

// JointBilateralSmoothing smoothes a depth map affected by noise by using the product of two gaussian filters,
// one based on spatial distance, and the other based on depth differences. depthSigma essentially sets a threshold to not
// smooth across large differences in depth.
func JointBilateralSmoothing(dm *DepthMap, spatialSigma, depthSigma float64) *DepthMap {
	filter := JointBilateralFilter(spatialSigma, depthSigma)
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
	return outDM
}

// JointTrilateralSmoothing smoothes a depth map affected by noise by using the product of three gaussian filters,
// one based on spatial distance, one based on color differences of the associated RGB image, and one based on depth differences.
func JointTrilateralSmoothing(ii *ImageWithDepth, spatialSigma, colSigma, depSigma float64) *ImageWithDepth {
	filter := JointTrilateralFilter(spatialSigma, colSigma, depSigma)
	width, height := ii.Depth.Width(), ii.Depth.Height()
	outDM := NewEmptyDepthMap(width, height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if ii.Depth.GetDepth(x, y) == 0 {
				continue
			}
			point := image.Point{x, y}
			val := filter(point, ii)
			outDM.Set(point.X, point.Y, Depth(val))
		}
	}
	return MakeImageWithDepth(ii.Color, outDM, ii.IsAligned(), ii.CameraSystem())
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
func CleanDepthMap(dm *DepthMap, threshold int) {
	validData := MissingDepthData(dm)
	regionMap := SegmentBinaryImage(validData)
	for _, seg := range regionMap {
		if len(seg) < threshold {
			for point, _ := range seg {
				dm.Set(point.X, point.Y, Depth(0))
			}
		}
	}
}

// FillDepthMap finds regions of connected missing data, and for those below a certain size, fills them in with
// an average of the surrounding pixels by using 8-point ray-marching.
func FillDepthMap(dm *DepthMap) {
	validData := MissingDepthData(dm)
	missingData := InvertGrayImage(validData)
	holeMap := SegmentBinaryImage(missingData)
	for _, seg := range holeMap {
		avgDepth := averageDepthAroundHole(seg, dm)
		threshold := thresholdFromDepth(avgDepth, dm.Width()*dm.Height())
		if len(seg) < threshold {
			for point, _ := range seg {
				val := depthRayMarching(point.X, point.Y, dm)
				dm.Set(point.X, point.Y, val)
			}
		}
	}
}

// limits inpainting to holes of a specific size. Farther distance means the same pixel size represents a larger area.
// It might be better to make it a real function of depth, right now just split into regions of close, middle, far and thresholds based on proportion of the image resolution.
func thresholdFromDepth(d float64, imgResolution int) int {
	res := float64(imgResolution)
	switch {
	case d < 500.:
		return int(0.05 * res)
	case d >= 500. && d < 4000.:
		return int(0.005 * res)
	default:
		return int(0.0005 * res)
	}
}

// take the average of the border of the hole as the average inside the hole
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
	for hole, _ := range segment {
		for _, dir := range directions {
			point := image.Point{hole.X + dir.X, hole.Y + dir.Y}
			if !dm.In(point.X, point.Y) {
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

// depthRayMarching uses 8-point ray-marching to fill in missing data. It marches out in 8 directions from the missing pixel until
// it encounters a pixel with data, and then averages the values of the non-zero pixels it finds to fill the missing value.
func depthRayMarching(x, y int, dm *DepthMap) Depth {
	directions := []image.Point{
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
	for _, dir := range directions {
		val := 0
		i, j := x, y
		for val == 0 { // increment in the given direction until you reach a filled pixel
			i += dir.X
			j += dir.Y
			if !dm.In(i, j) { // skip if out of picture bounds
				break
			}
			val = int(dm.GetDepth(i, j))
		}
		if val != 0 {
			valAvg = (valAvg*count + float64(val)) / (count + 1.)
			count += 1.
		}
	}
	valAvg = math.Max(valAvg, 0.0) // depth cannot be zero
	return Depth(valAvg)
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

// SegmentBinaryImage does a bredth-first search on a black and white image and splits the non-connected white regions
// into different regions.
func SegmentBinaryImage(img *image.Gray) map[int]map[image.Point]bool {
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

// InvertGrayImage produces a negated version of the input image
func InvertGrayImage(img *image.Gray) *image.Gray {
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

// BoolMapToGrayImage creates a black and white image out of a true/false map of points. True points map to white.
func BoolMapToGrayImage(boolMap map[image.Point]bool, width, height int) (*image.Gray, error) {
	dst := image.NewGray(image.Rect(0, 0, width, height))
	for point, _ := range boolMap {
		if !point.In(dst.Bounds()) {
			return nil, fmt.Errorf("point (%d,%d) not in bounds of image with dim (%d,%d)", point.X, point.Y, width, height)
		}
		dst.Set(point.X, point.Y, color.Gray{255})
	}
	return dst, nil
}

// DrawAverageHoleDepth
func DrawAverageHoleDepth(dm *DepthMap) *Image {
	red, green, blue := NewColor(255, 0, 0), NewColor(0, 255, 0), NewColor(0, 0, 255)
	img := NewImage(dm.Width(), dm.Height())
	validData := MissingDepthData(dm)
	missingData := InvertGrayImage(validData)
	holeMap := SegmentBinaryImage(missingData)
	for _, seg := range holeMap {
		avgDepth := averageDepthAroundHole(seg, dm)
		var c Color
		switch {
		case avgDepth < 500.0:
			c = red
		case avgDepth >= 500.0 && avgDepth < 4000.0:
			c = green
		default:
			c = blue
		}
		for pt, _ := range seg {
			img.Set(pt, c)
		}
	}
	return img
}
