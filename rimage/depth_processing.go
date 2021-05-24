package rimage

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"

	"go.viam.com/core/utils"
)

// expandDepthMapForConvolution pads the image on every side with a mirror image of the data. This is so evaluation
// of the convolution at the borders will happen smoothly and will not draw sharp edges.
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

// BoolMapToGrayImage creates an image out of true/false map of points
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

// Gaussian blur applies a blurring with a given sigma over the input depth map
func GaussianBlur(dm *DepthMap, sigma float64) *DepthMap {
	if sigma <= 0. {
		return dm
	}
	blurFilter := GaussianFilter(sigma)
	width, height := dm.Width(), dm.Height()
	outDM := NewEmptyDepthMap(width, height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if dm.GetDepth(x, y) == 0 {
				continue
			}
			point := image.Point{x, y}
			val := blurFilter(point, dm)
			outDM.Set(point.X, point.Y, Depth(val))
		}
	}
	return outDM
}

// SavistkyGolaySmoothing is used to smooth two dimensional data affected by noise. The algorithm is as follows:
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

func getValidNeighbors(pt image.Point, valid *image.Gray, visited map[image.Point]bool) []image.Point {
	neighbors := make([]image.Point, 0, 4)
	directions := []direction{
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
// into different segments.
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

// CleanDepthMap uses a breadth-first graph search to create a map of all the connected components of data.
// Removes the small regions below a certain thershold.
func CleanDepthMap(dm *DepthMap, threshold int) {
	// Create a list of connected non-empty regions in depth map
	// only keeping the largest regions
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

func SmoothNonZeroData(ii *ImageWithDepth) (*ImageWithDepth, error) {
	if !ii.IsAligned() {
		return nil, errors.New("input ImageWithDepth is not aligned.")
	}
	validPoints := MissingDepthData(ii.Depth)
	sgDepth := NewEmptyDepthMap(ii.Depth.Width(), ii.Depth.Height())
	err := SavitskyGolaySmoothing(ii.Depth, sgDepth, validPoints, 3, 3)
	if err != nil {
		return nil, err
	}

	return MakeImageWithDepth(ii.Color, sgDepth, ii.IsAligned(), ii.CameraSystem()), nil
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

// ForwardDepthGradient computes the forward gradients in the X and Y direction of a depth map
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

// Missing data and hole inprinting

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
