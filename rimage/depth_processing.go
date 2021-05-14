package rimage

import (
	"fmt"
	"image"
	"math"

	"go.viam.com/core/utils"
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

// DetectDepthEdges uses a Canny edge detector to find edges in a depth map and returns a
// grayscale image.
func (cd *CannyEdgeDetector) DetectDepthEdges(dm *DepthMap) (*image.Gray, error) {

	vectorField := SobelFilter(dm)
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

// Sobel filters are used to approximate the gradient of the image intensity. One filter for each direction.
var (
	sobelX = [3][3]int{{1, 0, -1}, {2, 0, -2}, {1, 0, -1}}
	sobelY = [3][3]int{{1, 2, 1}, {0, 0, 0}, {-1, -2, -1}}
)

// SobelFilter takes in a DepthMap, approximates the gradient in the X and Y direction at every pixel
// creates a  vector in polar form, and returns a vector field.
func SobelFilter(dm *DepthMap) VectorField2D {
	width, height := dm.Width(), dm.Height()
	maxMag := 0.0
	g := make([]Vec2D, 0, width*height)
	xRange, yRange := makeRangeArray(3), makeRangeArray(3)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// apply the Sobel Filter over a 3x3 square around each pixel
			sX, sY := 0, 0
			for i, dx := range xRange {
				for j, dy := range yRange {
					if x+dx < 0 || y+dy < 0 || x+dx >= width || y+dy >= height {
						continue
					}
					d := int(dm.GetDepth(x+dx, y+dy))
					// rows are height j, columns are width i
					sX += sobelX[j][i] * d
					sY += sobelY[j][i] * d
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

// Morphological transformations

// makeStructuringElement returns a simple square Structuring Element used to smooth the image.
// Structuring elements are like kernels, but only have positive value entries.
// 0 values represent skipping the pixel, rather than a zero value for the pixel
func makeStructuringElement(k int) *DepthMap {
	structEle := NewEmptyDepthMap(k, k)
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	center := (k + 1) / 2
	for y, dy := range yRange {
		for x, dx := range xRange {
			if utils.AbsInt(dy)+utils.AbsInt(dx) < center {
				structEle.Set(x, y, Depth(1))
			} else {
				structEle.Set(x, y, Depth(0))
			}
		}
	}
	return structEle
}

// Erode takes in a point, a depth map and a kernel and applies the operation erode(u,v) = min_(i,j){inDM[u+j,v+i]-kernel[j,i]}
// on the rectangle in the depth map centered at the point (u,v) operated on by the kernel
func Erode(center image.Point, dm, kernel *DepthMap) Depth {
	xRange, yRange := makeRangeArray(kernel.Width()), makeRangeArray(kernel.Height())
	depth := int(MaxDepth)
	for y, dy := range yRange {
		for x, dx := range xRange {
			if center.X+dx < 0 || center.Y+dy < 0 || center.X+dx >= dm.Width() || center.Y+dy >= dm.Height() {
				continue
			}
			kernelVal := kernel.GetDepth(x, y)
			dmVal := dm.GetDepth(center.X+dx, center.Y+dy)
			if kernelVal == 0 || dmVal == 0 {
				continue
			}
			depth = utils.MinInt(int(dmVal-kernelVal), depth)

		}
	}
	depth = utils.MaxInt(0, depth) // can't have depth less than 0
	return Depth(depth)
}

// Dilate takes in a point, a depth map and a kernel and applies the operation dilate(u,v) = max_(i,j){inDM[u+j,v+i]+kernel[j,i]}
// on the rectangle in the depth map centered at the point (u,v) operated on by the kernel
func Dilate(center image.Point, dm, kernel *DepthMap) Depth {
	xRange, yRange := makeRangeArray(kernel.Width()), makeRangeArray(kernel.Height())
	depth := 0
	for y, dy := range yRange {
		for x, dx := range xRange {
			if center.X+dx < 0 || center.Y+dy < 0 || center.X+dx >= dm.Width() || center.Y+dy >= dm.Height() || kernel.GetDepth(x, y) == 0 {
				continue
			}
			depth = utils.MaxInt(int(dm.GetDepth(center.X+dx, center.Y+dy)+kernel.GetDepth(x, y)), depth)

		}
	}
	depth = utils.MaxInt(0, depth) // can't have depth less than 0
	return Depth(depth)
}

// MorphFilter takes in a pointer of the input depth map, the output depth map, the size of the kernel,
// the number of times to apply the filter, and the filter to apply. Morphological filters are used in
// image preprocessing to smooth, prune, and fill in noise in the image.
func MorphFilter(inDM, outDM *DepthMap, kernelSize, iterations int, process func(center image.Point, dm, kernel *DepthMap) Depth) error {
	if kernelSize%2 == 0 {
		return fmt.Errorf("kernelSize must be an odd number, input was %d", kernelSize)
	}
	width, height := inDM.Width(), inDM.Height()
	widthOut, heightOut := outDM.Width(), outDM.Height()
	if widthOut != width || heightOut != height {
		return fmt.Errorf("dimensions of inDM and outDM must match. in(%d,%d) != out(%d,%d)", width, height, widthOut, heightOut)
	}
	kernel := makeStructuringElement(kernelSize)
	*outDM = *inDM
	for n := 0; n < iterations; n++ {
		tempDM := NewEmptyDepthMap(width, height)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				val := process(image.Point{x, y}, outDM, kernel)
				tempDM.Set(x, y, val)
			}
		}
		*outDM = *tempDM
	}
	return nil
}

// ClosingMorph applies a closing morphological transform, which is a Dilation followed by an Erosion.
// Closing smooths an image by fusing narrow breaks and filling small holes and gaps.
func ClosingMorph(dm *DepthMap, kernelSize, iterations int) (*DepthMap, error) {
	outDM := NewEmptyDepthMap(dm.Width(), dm.Height())
	tempDM := NewEmptyDepthMap(dm.Width(), dm.Height())
	err := MorphFilter(dm, tempDM, kernelSize, iterations, Dilate)
	if err != nil {
		return nil, err
	}
	err = MorphFilter(tempDM, outDM, kernelSize, iterations, Erode)
	if err != nil {
		return nil, err
	}
	return outDM, nil
}

// OpeningMorph applies an opening morphological transform, which is a Erosion followed by a Dilation.
// Opening smooths an image by eliminating thin protrusions and narrow outcroppings.
func OpeningMorph(dm *DepthMap, kernelSize, iterations int) (*DepthMap, error) {
	outDM := NewEmptyDepthMap(dm.Width(), dm.Height())
	tempDM := NewEmptyDepthMap(dm.Width(), dm.Height())
	err := MorphFilter(dm, tempDM, kernelSize, iterations, Erode)
	if err != nil {
		return nil, err
	}
	err = MorphFilter(tempDM, outDM, kernelSize, iterations, Dilate)
	if err != nil {
		return nil, err
	}
	return outDM, nil
}
