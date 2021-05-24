package rimage

import (
	"fmt"
	"image"
	"math"

	"go.viam.com/core/utils"

	"gonum.org/v1/gonum/mat"
)

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
	k := utils.MaxInt(3, 1+2*int(math.Ceil(4.*sigma)))
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
		if dm.GetDepth(p.X, p.Y) == 0 {
			return sX, sY
		}
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

// VectorBlurFilter sets the vector at point p to be the average of vectors in a k x k square around it.
func VectorBlurFilter(k int) func(p image.Point, vf *VectorField2D) Vec2D {
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	filter := func(p image.Point, vf *VectorField2D) Vec2D {
		sumX, sumY := 0.0, 0.0
		count := 0.0
		for _, dx := range xRange {
			for _, dy := range yRange {
				point := image.Point{p.X + dx, p.Y + dy}
				if !vf.Contains(point) || vf.Get(point).Magnitude() == 0. {
					continue
				}
				x, y := vf.Get(point).Cartesian()
				sumX += x
				sumY += y
				count++
			}
		}
		if count == 0 {
			return NewVec2D(0, 0)
		}
		mag, dir := getMagnitudeAndDirection(sumX/count, sumY/count)
		return NewVec2D(mag, dir)
	}
	return filter
}

type Exponents image.Point

func polyExponents(order int) []Exponents {
	exps := make([]Exponents, 0, (order+1)*(order+2)/2)
	for k := 0; k < order+1; k++ {
		for n := 0; n < k+1; n++ {
			exps = append(exps, Exponents{k - n, n})
		}
	}
	return exps
}

func eye(n int) *mat.Dense {
	m := mat.NewDense(n, n, nil)
	for i := 0; i < n; i++ {
		m.Set(i, i, 1)
	}
	return m
}

func SavitskyGolayKernel(radius, order int) ([][]float64, error) {
	windowSize := 1 + 2*radius
	nElements := windowSize * windowSize
	exps := polyExponents(order)
	nTerms := len(exps)
	if nElements < nTerms {
		return nil, fmt.Errorf("n elements in window (%d) is less than terms to solve (%d)", nElements, nTerms)
	}
	xRange, yRange := makeRangeArray(windowSize), makeRangeArray(windowSize)
	// we are going to create a least-squares equation to solve
	A := mat.NewDense(nElements, nTerms, nil)
	for i, y := range yRange {
		for j, x := range xRange {
			for k, exp := range exps {
				A.Set(i*(windowSize)+j, k, math.Pow(float64(x), float64(exp.X))*math.Pow(float64(y), float64(exp.Y)))
			}
		}
	}
	// calculate psuedo-inverse
	var solution mat.Dense
	I := eye(nElements)
	err := solution.Solve(A, I)
	if err != nil {
		return nil, err
	}
	coefs := solution.RowView(0).(*mat.VecDense).RawVector().Data
	kernel := [][]float64{}
	for y, _ := range yRange {
		row := make([]float64, windowSize)
		for x, _ := range xRange {
			row[x] = coefs[y*windowSize+x]
		}
		kernel = append(kernel, row)
	}
	return kernel, nil
}

// SavitskyGolayFilter algorithm is as follows:
// 1. for each point of the DepthMap extract a sub-matrix, centered at that point and with a size equal to an odd number "windowSize".
// 2. For this sub-matrix compute a least-square fit of a polynomial surface, defined as p(x,y) = a0 + a1*x + a2*y + a3*x\^2 + a4*y\^2 + a5*x*y + ... . Note that x and y are equal to zero at the central point. The parameters for the fit are gotten from the SavitskyGolayKernel.
// 3. The output value is computed with the fit.
func SavitskyGolayFilter(radius, polyOrder int) (func(p image.Point, dm *DepthMap) float64, error) {
	kernel, err := SavitskyGolayKernel(radius, polyOrder)
	if err != nil {
		return nil, err
	}
	k := len(kernel)
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	filter := func(p image.Point, dm *DepthMap) float64 {
		val := 0.0
		for i, dx := range xRange {
			for j, dy := range yRange {
				if !dm.In(p.X+dx, p.Y+dy) {
					continue
				}
				// rows are height j, columns are width i
				val += kernel[j][i] * float64(dm.GetDepth(p.X+dx, p.Y+dy))
			}
		}
		return math.Max(0, val)
	}
	return filter, nil
}
