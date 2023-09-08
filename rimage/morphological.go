//go:build cgo
package rimage

import (
	"fmt"
	"image"

	"go.viam.com/rdk/utils"
)

// resource and tutorial on mathematical morphology:
// https://clouard.users.greyc.fr/Pantheon/experiments/morphology/index-en.html

// makeStructuringElement returns a simple circular Structuring Element used to smooth the image.
// Structuring elements are like kernels, but only have positive value entries.
// 0 values represent skipping the pixel, rather than a zero value for the pixel.
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

// Erode takes in a point, a depth map and an struct element and applies the operation erode(u,v) = min_(i,j){inDM[u+j,v+i]-kernel[j,i]}
// on the rectangle in the depth map centered at the point (u,v) operated on by the element
// https://clouard.users.greyc.fr/Pantheon/experiments/morphology/index-en.html#ch2-A
func erode(center image.Point, dm, kernel *DepthMap) Depth {
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

// Dilate takes in a point, a depth map and a struct element and applies the operation dilate(u,v) = max_(i,j){inDM[u+j,v+i]+kernel[j,i]}
// on the rectangle in the depth map centered at the point (u,v) operated on by the element
// https://clouard.users.greyc.fr/Pantheon/experiments/morphology/index-en.html#ch2-A
func dilate(center image.Point, dm, kernel *DepthMap) Depth {
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
// https://clouard.users.greyc.fr/Pantheon/experiments/morphology/index-en.html#ch3-A
func ClosingMorph(dm *DepthMap, kernelSize, iterations int) (*DepthMap, error) {
	outDM := NewEmptyDepthMap(dm.Width(), dm.Height())
	tempDM := NewEmptyDepthMap(dm.Width(), dm.Height())
	err := MorphFilter(dm, tempDM, kernelSize, iterations, dilate)
	if err != nil {
		return nil, err
	}
	err = MorphFilter(tempDM, outDM, kernelSize, iterations, erode)
	if err != nil {
		return nil, err
	}
	return outDM, nil
}

// OpeningMorph applies an opening morphological transform, which is a Erosion followed by a Dilation.
// Opening smooths an image by eliminating thin protrusions and narrow outcroppings.
// https://clouard.users.greyc.fr/Pantheon/experiments/morphology/index-en.html#ch3-A
func OpeningMorph(dm *DepthMap, kernelSize, iterations int) (*DepthMap, error) {
	outDM := NewEmptyDepthMap(dm.Width(), dm.Height())
	tempDM := NewEmptyDepthMap(dm.Width(), dm.Height())
	err := MorphFilter(dm, tempDM, kernelSize, iterations, erode)
	if err != nil {
		return nil, err
	}
	err = MorphFilter(tempDM, outDM, kernelSize, iterations, dilate)
	if err != nil {
		return nil, err
	}
	return outDM, nil
}
