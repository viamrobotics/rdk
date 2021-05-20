package rimage

import (
	"fmt"
	"image"
	"image/color"
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

// GaussianFunction1D takes in a sigma and returns a gaussian function useful for weighing averages or blurring.
func GaussianFunction1D(sigma float64) func(p float64) float64 {
	return func(p float64) float64 {
		return math.Exp(-0.5*math.Pow(p, 2)/math.Pow(sigma, 2)) / (sigma * math.Sqrt(2.*math.Pi))
	}
}

// GaussianFunction2D takes in a sigma and returns an isotropic 2D gaussian
func GaussianFunction2D(sigma float64) func(p1, p2 float64) float64 {
	return func(p1, p2 float64) float64 {
		return math.Exp(-0.5*(p1*p1+p2*p2)/math.Pow(sigma, 2)) / (sigma * sigma * 2. * math.Pi)
	}
}

func GaussianKernel(sigma float64) [][]float64 {
	gaus2D := GaussianFunction2D(sigma)
	// size of the kernel is determined by size of sigma. want to get 3 sigma worth of gaussian function
	k := utils.MaxInt(3, 1+2*int(3.*sigma))
	xRange := makeRangeArray(k)
	kernel := [][]float64{}
	for y := 0; y < k; y++ {
		row := make([]float64, k)
		for i, x := range xRange {
			row[i] = gaus2D(float64(x), float64(y))
		}
		kernel = append(kernel, row)
	}
	return kernel
}

func GaussianBlur(dm *DepthMap, sigma float64) *DepthMap {
	if sigma <= 0. {
		return dm
	}
	width, height := dm.Width(), dm.Height()
	outDM := NewEmptyDepthMap(width, height)
	kernel := GaussianKernel(sigma)
	k := len(kernel)
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// apply the blur around the depth map pixel (x,y)
			val := 0.0
			weight := 0.0
			for i, dx := range xRange {
				for j, dy := range yRange {
					if !dm.In(x+dx, y+dy) {
						continue
					}
					d := float64(dm.GetDepth(x+dx, y+dy))
					if d == 0.0 {
						continue
					}
					// rows are height j, columns are width i
					val += kernel[j][i] * d
					weight += kernel[j][i]
				}
			}
			val = math.Max(0, val/weight)
			outDM.Set(x, y, Depth(val))
		}
	}
	return outDM
}

// JointTrilateralFilter smooths the depth map using information from the color image.
func JointTrilateralFilter(ii *ImageWithDepth, k int, spatialVar, colorVar, depthVar float64) (*ImageWithDepth, error) {
	if !ii.IsAligned() {
		return nil, fmt.Errorf("input ImageWithDepth is not aligned.")
	}
	if k%2 == 0 {
		return nil, fmt.Errorf("kernel size k must be odd, has size %d", k)
	}
	width, height := ii.Width(), ii.Height()
	outDM := NewEmptyDepthMap(width, height)
	spatialFilter := GaussianFunction1D(spatialVar)
	//depthFilter := GaussianFunction1D(depthVar)
	colorFilter := GaussianFunction1D(colorVar)
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// apply the Joint Trilateral Filter over a k x k square around each pixel
			//pDepth := float64(ii.Depth.GetDepth(x, y))
			pColor := ii.Color.GetXY(x, y)
			newDepth := 0.0
			totalWeight := 0.0
			for _, dx := range xRange {
				for _, dy := range yRange {
					if !ii.Color.In(x+dx, y+dy) {
						continue
					}
					d := float64(ii.Depth.GetDepth(x+dx, y+dy))
					if d == 0.0 {
						continue
					}
					weight := spatialFilter(float64(dx)) * spatialFilter(float64(dy))
					weight *= colorFilter(pColor.DistanceLab(ii.Color.GetXY(x+dx, y+dy)))
					//weight *= depthFilter(pDepth - d)
					//fmt.Printf("colorDist: %v, depthDist: %v\n", pColor.DistanceLab(ii.Color.GetXY(x+dx, y+dy)), pDepth-d)
					newDepth += d * weight
					totalWeight += weight
				}
			}
			outDM.Set(x, y, Depth(newDepth/totalWeight))
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

// Sobel filters are used to approximate the gradient of the image intensity. One filter for each direction.
var (
	sobelX = [3][3]int{{-1, 0, 1}, {-2, 0, 2}, {-1, 0, 1}}
	sobelY = [3][3]int{{-1, -2, -1}, {0, 0, 0}, {1, 2, 1}}
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
					if !dm.In(x+dx, y+dy) {
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

// Missing data and hole inprinting

// PixlesToBeFilled categorizes the pixels to be left blank or filled with data. Depends on the majority within the window
func PixelsToBeFilled(dm *image.Gray, k int) *image.Gray {
	holes := image.NewGray(dm.Bounds())
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	zero := color.Gray{0}
	for y := 0; y < dm.Bounds().Dy(); y++ {
		for x := 0; x < dm.Bounds().Dx(); x++ {
			hole, nonHole := 0, 0
			for _, dx := range xRange {
				for _, dy := range yRange {
					point := image.Point{x + dx, y + dy}
					if !point.In(dm.Bounds()) {
						continue
					}
					if dm.At(point.X, point.Y) == zero {
						nonHole += 1
					} else {
						hole += 1
					}
				}
			}
			if hole > nonHole {
				holes.Set(x, y, color.Gray{255})
			}
		}
	}
	return holes
}

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
