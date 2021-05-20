package rimage

import (
	"image"
	"math"

	"go.viam.com/core/utils"
)

// GaussianFunction1D takes in a sigma and returns a gaussian function useful for weighing averages or blurring.
func GaussianFunction1D(sigma float64) func(p float64) float64 {
	if sigma <= 0. {
		return func(p float64) float64 {
			return 1.
		}
	}
	return func(p float64) float64 {
		return math.Exp(-0.5*math.Pow(p, 2)/math.Pow(sigma, 2)) / (sigma * math.Sqrt(2.*math.Pi))
	}
}

// GaussianFunction2D takes in a sigma and returns an isotropic 2D gaussian
func GaussianFunction2D(sigma float64) func(p1, p2 float64) float64 {
	if sigma <= 0. {
		return func(p1, p2 float64) float64 {
			return 1.
		}
	}
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

// Filters for convolutions

func GaussianFilter(sigma float64) func(p image.Point, dm *DepthMap) float64 {
	kernel := GaussianKernel(sigma)
	k := len(kernel)
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	filter := func(p image.Point, dm *DepthMap) float64 {
		val := 0.0
		weight := 0.0
		for i, dx := range xRange {
			for j, dy := range yRange {
				if !dm.In(p.X+dx, p.Y+dy) {
					continue
				}
				d := float64(dm.GetDepth(p.X+dx, p.Y+dy))
				if d == 0.0 {
					continue
				}
				// rows are height j, columns are width i
				val += kernel[j][i] * d
				weight += kernel[j][i]
			}
		}
		return math.Max(0, val/weight)
	}
	return filter
}

func DirectionalJointBilateralFilter(spatialXSigma, spatialYSigma, colorSigma float64) func(p image.Point, direction float64, ii *ImageWithDepth) float64 {
	spatialXFilter := GaussianFunction1D(spatialXSigma)
	spatialYFilter := GaussianFunction1D(spatialYSigma)
	colorFilter := GaussianFunction1D(colorSigma)
	// k is determined by spatial sigma
	k := utils.MaxInt(3, 1+2*int(3.*spatialXSigma))
	k = utils.MaxInt(k, 1+2*int(3.*spatialYSigma))
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	filter := func(p image.Point, direction float64, ii *ImageWithDepth) float64 {
		pColor := ii.Color.GetXY(p.X, p.Y)
		newDepth := 0.0
		totalWeight := 0.0
		for _, dx := range xRange {
			for _, dy := range yRange {
				if !ii.Color.In(p.X+dx, p.Y+dy) {
					continue
				}
				d := float64(ii.Depth.GetDepth(p.X+dx, p.Y+dy))
				if d == 0.0 {
					continue
				}
				weight := spatialXFilter(float64(dx)) * spatialYFilter(float64(dy))
				weight *= colorFilter(pColor.DistanceLab(ii.Color.GetXY(p.X+dx, p.Y+dy)))
				newDepth += d * weight
				totalWeight += weight
			}
		}
		return newDepth / totalWeight
	}
	return filter
}

func JointBilateralFilter(spatialSigma, colorSigma float64) func(p image.Point, ii *ImageWithDepth) float64 {
	spatialFilter := GaussianFunction1D(spatialSigma)
	colorFilter := GaussianFunction1D(colorSigma)
	k := utils.MaxInt(3, 1+2*int(3.*spatialSigma))
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	filter := func(p image.Point, ii *ImageWithDepth) float64 {
		pColor := ii.Color.GetXY(p.X, p.Y)
		newDepth := 0.0
		totalWeight := 0.0
		for _, dx := range xRange {
			for _, dy := range yRange {
				if !ii.Color.In(p.X+dx, p.Y+dy) {
					continue
				}
				d := float64(ii.Depth.GetDepth(p.X+dx, p.Y+dy))
				if d == 0.0 {
					continue
				}
				weight := spatialFilter(float64(dx)) * spatialFilter(float64(dy))
				weight *= colorFilter(pColor.DistanceLab(ii.Color.GetXY(p.X+dx, p.Y+dy)))
				newDepth += d * weight
				totalWeight += weight
			}
		}
		return newDepth / totalWeight
	}
	return filter
}

func JointTrilateralFilter(spatialSigma, colorSigma, depthSigma float64) func(p image.Point, ii *ImageWithDepth) float64 {
	spatialFilter := GaussianFunction1D(spatialSigma)
	depthFilter := GaussianFunction1D(depthSigma)
	colorFilter := GaussianFunction1D(colorSigma)
	k := utils.MaxInt(3, 1+2*int(3.*spatialSigma))
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	filter := func(p image.Point, ii *ImageWithDepth) float64 {
		pColor := ii.Color.GetXY(p.X, p.Y)
		pDepth := float64(ii.Depth.GetDepth(p.X, p.Y))
		newDepth := 0.0
		totalWeight := 0.0
		for _, dx := range xRange {
			for _, dy := range yRange {
				if !ii.Color.In(p.X+dx, p.Y+dy) {
					continue
				}
				d := float64(ii.Depth.GetDepth(p.X+dx, p.Y+dy))
				if d == 0.0 {
					continue
				}
				weight := spatialFilter(float64(dx)) * spatialFilter(float64(dy))
				weight *= colorFilter(pColor.DistanceLab(ii.Color.GetXY(p.X+dx, p.Y+dy)))
				weight *= depthFilter(pDepth - d)
				//fmt.Printf("colorDist: %v, depthDist: %v\n", pColor.DistanceLab(ii.Color.GetXY(x+dx, y+dy)), pDepth-d)
				newDepth += d * weight
				totalWeight += weight
			}
		}
		return newDepth / totalWeight
	}
	return filter
}

// Sobel filters are used to approximate the gradient of the image intensity. One filter for each direction.

var (
	sobelX = [3][3]float64{{-1, 0, 1}, {-2, 0, 2}, {-1, 0, 1}}
	sobelY = [3][3]float64{{-1, -2, -1}, {0, 0, 0}, {1, 2, 1}}
)

// SobelFilter takes in a DepthMap, approximates the gradient in the X and Y direction at every pixel
// creates a  vector in polar form, and returns a vector field.
func SobelDepthFilter() func(p image.Point, dm *DepthMap) (float64, float64) {
	xRange, yRange := makeRangeArray(3), makeRangeArray(3)
	// apply the Sobel Filter over a 3x3 square around each pixel
	filter := func(p image.Point, dm *DepthMap) (float64, float64) {
		sX, sY := 0.0, 0.0
		for i, dx := range xRange {
			for j, dy := range yRange {
				if !dm.In(p.X+dx, p.Y+dy) {
					continue
				}
				d := float64(dm.GetDepth(p.X+dx, p.Y+dy))
				// rows are height j, columns are width i
				sX += sobelX[j][i] * d
				sY += sobelY[j][i] * d
			}
		}
		return sX, sY
	}
	return filter
}

func SobelColorFilter() func(p image.Point, img *Image) (float64, float64) {
	xRange, yRange := makeRangeArray(3), makeRangeArray(3)
	// apply the Sobel Filter over a 3x3 square around each pixel
	filter := func(p image.Point, img *Image) (float64, float64) {
		sX, sY := 0.0, 0.0
		for i, dx := range xRange {
			for j, dy := range yRange {
				if !img.In(p.X+dx, p.Y+dy) {
					continue
				}
				c := Luminance(img.GetXY(p.X+dy, p.Y+dy))
				// rows are height j, columns are width i
				sX += sobelX[j][i] * c
				sY += sobelY[j][i] * c
			}
		}
		return sX, sY
	}
	return filter
}
