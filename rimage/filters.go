//go:build !notc

package rimage

import (
	"fmt"
	"image"
	"math"

	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/utils"
)

// Helper function for convolving depth maps with kernels. When used with i, dx := range makeRangeArray(n)
// i is the position within the kernel and dx gives the offset within the depth map.
// if length is even, then the origin is to the right of middle i.e. 4 -> {-2, -1, 0, 1} (even lengths rarely used).
func makeRangeArray(length int) []int {
	if length <= 0 {
		return make([]int, 0)
	}
	tailValue := 0
	if length%2 == 0 { // length is even, save value to prepend to beginning
		tailValue = length / 2
		length--
	}
	rangeArray := make([]int, length)
	span := (length - 1) / 2
	for i := 0; i < span; i++ {
		rangeArray[length-1-i] = span - i
		rangeArray[i] = -span + i
	}
	if tailValue != 0 {
		rangeArray = append([]int{-tailValue}, rangeArray...)
	}
	return rangeArray
}

// gaussianFunction1D takes in a sigma and returns a gaussian function useful for weighing averages or blurring.
func gaussianFunction1D(sigma float64) func(p float64) float64 {
	if sigma <= 0. {
		return func(p float64) float64 {
			return 1.
		}
	}
	return func(p float64) float64 {
		return math.Exp(-0.5*math.Pow(p, 2)/math.Pow(sigma, 2)) / (sigma * math.Sqrt(2.*math.Pi))
	}
}

// gaussianFunction2D takes in a sigma and returns an isotropic 2D gaussian useful for weighing averages or blurring.
func gaussianFunction2D(sigma float64) func(p1, p2 float64) float64 {
	if sigma <= 0. {
		return func(p1, p2 float64) float64 {
			return 1.
		}
	}
	return func(p1, p2 float64) float64 {
		return math.Exp(-0.5*(p1*p1+p2*p2)/math.Pow(sigma, 2)) / (sigma * sigma * 2. * math.Pi)
	}
}

// gaussianKernel takes characteristic length (sigma) as input and creates the k x k 2D array used to create the Guassian filter.
func gaussianKernel(sigma float64) [][]float64 {
	gaus2D := gaussianFunction2D(sigma)
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

// Filters for convolutions, used in their corresponding smoothing functions

// using just spatial information to fill the kernel values.
func gaussianFilter(sigma float64) func(p image.Point, dm *DepthMap) float64 {
	kernel := gaussianKernel(sigma)
	k := len(kernel)
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	filter := func(p image.Point, dm *DepthMap) float64 {
		val := 0.0
		weight := 0.0
		for i, dx := range xRange {
			for j, dy := range yRange {
				if !dm.Contains(p.X+dx, p.Y+dy) {
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

// Uses both spatial and depth information to fill the kernel values.
func jointBilateralFilter(spatialSigma, depthSigma float64) func(p image.Point, dm *DepthMap) float64 {
	spatialFilter := gaussianFunction2D(spatialSigma)
	depthFilter := gaussianFunction1D(depthSigma)
	k := utils.MaxInt(3, 1+2*int(3.*spatialSigma)) // 3 sigma worth of area
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	filter := func(p image.Point, dm *DepthMap) float64 {
		newDepth := 0.0
		totalWeight := 0.0
		center := float64(dm.GetDepth(p.X, p.Y))
		for _, dx := range xRange {
			for _, dy := range yRange {
				if !dm.Contains(p.X+dx, p.Y+dy) {
					continue
				}
				d := float64(dm.GetDepth(p.X+dx, p.Y+dy))
				if d == 0.0 {
					continue
				}
				weight := spatialFilter(float64(dx), float64(dy))
				weight *= depthFilter(center - d)
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
func sobelDepthFilter() func(p image.Point, dm *DepthMap) (float64, float64) {
	xRange, yRange := makeRangeArray(3), makeRangeArray(3)
	// apply the Sobel Filter over a 3x3 square around each pixel
	filter := func(p image.Point, dm *DepthMap) (float64, float64) {
		sX, sY := 0.0, 0.0
		if dm.GetDepth(p.X, p.Y) == 0 {
			return sX, sY
		}
		for i, dx := range xRange {
			for j, dy := range yRange {
				if !dm.Contains(p.X+dx, p.Y+dy) {
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

func sobelColorFilter() func(p image.Point, img *Image) (float64, float64) {
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
func vectorBlurFilter(k int) func(p image.Point, vf *VectorField2D) Vec2D {
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	filter := func(p image.Point, vf *VectorField2D) Vec2D {
		sumX, sumY := 0.0, 0.0
		count := 0.0
		for _, dx := range xRange {
			for _, dy := range yRange {
				point := image.Point{p.X + dx, p.Y + dy}
				if !vf.Contains(point.X, point.Y) || vf.Get(point).Magnitude() == 0. {
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

// SavitskyGolayFilter algorithm is as follows:
// 1. for each point of the DepthMap extract a sub-matrix, centered at that point and with a size equal to an
// odd number "windowSize".
// 2. For this sub-matrix compute a least-square fit of a polynomial surface, defined as
// p(x,y) = a0 + a1*x + a2*y + a3*x\^2 + a4*y\^2 + a5*x*y + ... .
// Note that x and y are equal to zero at the central point. The parameters for the fit are gotten from
// the SavitskyGolayKernel.
// 3. The output value is computed with the calculated fit parameters multiplied times the input data.
func savitskyGolayFilter(radius, polyOrder int) (func(p image.Point, dm *DepthMap) float64, error) {
	kernel, err := savitskyGolayKernel(radius, polyOrder)
	if err != nil {
		return nil, err
	}
	k := len(kernel)
	xRange, yRange := makeRangeArray(k), makeRangeArray(k)
	filter := func(p image.Point, dm *DepthMap) float64 {
		val := 0.0
		for i, dx := range xRange {
			for j, dy := range yRange {
				if !dm.Contains(p.X+dx, p.Y+dy) {
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

// To calculate a least squares fit to a polynomial equation, one is trying to calculate the coefficients "a" in
// p(x,y) = a0 + a1*x + a2*y + a3*x^2 + a4*y^2 + a5*x*y + ... such that the square difference sum_over_x,y |f(x,y) - p(x,y)|^2
// is a minimum. f(x,y) is the actual data, in this case the depth info from the input image. We represent the data f(x,y) as a vector
// f, and the equation p(x,y) as a product of the matrix A and vector of coefs a. Therefore, we want to solve the equation that gets
// as close as possible to Aa - f = 0 or equivalently a = (A^-1)f. We can pre-compute the pseudo-inverse of A to apply to f later,
// and since we only need to know p(0,0), the point at the center of the filter, we only need to use the first row of A^-1
// which gives a0. If you wanted to get the gradient of the fit as well, you could use the 2nd and 3rd row of A^-1
// which represent a1 and a2 respectively.

// creates a slice of the exponents on the x and y in each term of the polynomial.
// e.g. image.Point{0,2} -> a4*y^2, image.Point{1,1} -> a5*x*y.
func polyExponents(order int) []image.Point {
	exps := make([]image.Point, 0, (order+1)*(order+2)/2)
	for k := 0; k < order+1; k++ {
		for n := 0; n < k+1; n++ {
			exps = append(exps, image.Point{k - n, n})
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

func savitskyGolayKernel(radius, order int) ([][]float64, error) {
	windowSize := 1 + 2*radius
	nElements := windowSize * windowSize
	// we are going to create a least-squares equation fit to a 2D polynomial of order "order"
	exps := polyExponents(order)
	nTerms := len(exps)
	if nElements < nTerms {
		return nil, fmt.Errorf("n elements in window (%d) is less than terms to solve (%d)", nElements, nTerms)
	}
	xRange, yRange := makeRangeArray(windowSize), makeRangeArray(windowSize)
	A := mat.NewDense(nElements, nTerms, nil)
	for i, y := range yRange {
		for j, x := range xRange {
			for k, exp := range exps {
				A.Set(i*(windowSize)+j, k, math.Pow(float64(x), float64(exp.X))*math.Pow(float64(y), float64(exp.Y)))
			}
		}
	}
	// calculate pseudo-inverse of A
	var solution mat.Dense
	I := eye(nElements)
	if err := solution.Solve(A, I); err != nil {
		return nil, err
	}
	// Get the row used to calculate the a0 coefficients and form it back into a square
	coefs := solution.RowView(0).(*mat.VecDense).RawVector().Data
	kernel := [][]float64{}
	for y := range yRange {
		row := make([]float64, windowSize)
		for x := range xRange {
			row[x] = coefs[y*windowSize+x]
		}
		kernel = append(kernel, row)
	}
	return kernel, nil
}
