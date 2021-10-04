package chessboard

import (
	"math"

	"github.com/golang/geo/r2"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/transform"
)

// ChessGreedyConfiguration stores the parameters for the iterative refinement of the grid
type ChessGreedyConfiguration struct {
	HomographyAcceptableScaleRatio float64 `json:"scale-ratio"`       // acceptable ratio for scale part in estimated homography
	MinPointsNeeded                int     `json:"min_points_needed"` // minimum number of points to deem Grid estimation valid
	MaxPointsNeeded                int     `json:"max_points_needed"` // if number of valid points above this, greedy iterations can be stopped
}

var (
	quadI = []r2.Point{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
)

// Single generates an n-dimensional Grid using a single set of values.
// dim specifies the number of dimensions, the entries in x specify the gridded values.
func Single(dim int, x []float64) [][]float64 {
	dims := make([]int, dim)
	for i := range dims {
		dims[i] = len(x)
	}
	sz := size(dims)
	pts := make([][]float64, sz)
	sub := make([]int, dim)
	for i := 0; i < sz; i++ {
		SubFor(sub, i, dims)
		pt := make([]float64, dim)
		for j := range pt {
			pt[j] = x[sub[j]]
		}
		pts[i] = pt
	}
	return pts
}

func size(dims []int) int {
	n := 1
	for _, v := range dims {
		n *= v
	}
	return n
}

// SubFor constructs the multi-dimensional subscript for the input linear index.
// dims specifies the maximum size in each dimension. SubFor is the converse of
// IdxFor.
//
// If sub is non-nil the result is stored in-place into sub. If it is nil a new
// slice of the appropriate length is allocated.
func SubFor(sub []int, idx int, dims []int) []int {
	for _, v := range dims {
		if v <= 0 {
			panic("bad dims")
		}
	}
	if sub == nil {
		sub = make([]int, len(dims))
	}
	if len(sub) != len(dims) {
		panic("size mismatch")
	}
	if idx < 0 {
		panic("bad index")
	}
	stride := 1
	for i := len(dims) - 1; i >= 1; i-- {
		stride *= dims[i]
	}
	for i := 0; i < len(dims)-1; i++ {
		v := idx / stride
		if v >= dims[i] {
			panic("bad index")
		}
		sub[i] = v
		idx -= v * stride
		stride /= dims[i+1]
	}
	if idx > dims[len(sub)-1] {
		panic("bad dims")
	}
	sub[len(sub)-1] = idx
	return sub
}

// getIdentityGrid returns a n x n 2D Grid with coordinates offset,..., offset+n-1,
func getIdentityGrid(n, offset int) []r2.Point {
	// create n x n 2D Grid 0...n-1
	x := make([]float64, n)
	floats.Span(x, 0, float64(n-1))
	pts := Single(2, x)

	// add offset
	for _, points := range pts {
		floats.AddConst(float64(offset), points)

	}
	// output slice of r2.Point
	outPoints := make([]r2.Point, 0)
	for i := 0; i < len(pts); i++ {
		pt := r2.Point{X: pts[i][1], Y: pts[i][0]}
		outPoints = append(outPoints, pt)
	}
	return outPoints
}

// makeChessGrid returns an identity Grid and its transformation with homography H
func makeChessGrid(H rimage.Matrix, n int) ([]r2.Point, []r2.Point) {
	grid := transform.ApplyHomography(H, idealGrid)
func getInitialChessGrid(quad []r2.Point) ([]r2.Point, []r2.Point, *mat.Dense, error) {
	// order points ccw
	quad = rimage.SortPointCounterClockwise(quad)
	// estimate exact homography
	H, err := transform.EstimateExactHomographyFrom8Points(quadI, quad)
	if err != nil {
		return nil, nil, nil, err
	}
	// make chess Grid
	if H != nil {
		idealGrid, grid := makeChessGrid(H, 1)
		return idealGrid, grid, H, nil
	}
	return nil, nil, nil, nil
}

// GenerateNewBestFit gets the homography that gets the most inliers in current Grid
func GenerateNewBestFit(gridIdeal, grid []r2.Point, gridGood []int) (*mat.Dense, error) {
	// select valid chessboard corner points in ideal Grid
	ptsA := make([]r2.Point, 0)
	for i, pt := range gridIdeal {
		if gridGood[i] == 1 {
			ptsA = append(ptsA, pt)
		}
	}
	// select valid chessboard corner points in detected Grid
	ptsB := make([]r2.Point, 0)
	for i, pt := range grid {
		if gridGood[i] == 1 {
			ptsB = append(ptsB, pt)
		}
	}
	// estimate homography from these points
	H, _, err := transform.EstimateHomographyRANSAC(ptsA, ptsB, 0.5, 200)
	if err != nil {
		return nil, err
	}
	return H, nil
}

// findGoodPoints returns points from a detected Grid that are close to a saddle point and replace these points
// with the saddle points coordinates
func findGoodPoints(grid, saddlePoints []r2.Point, maxPointDist float64) ([]r2.Point, []int) {
	// for each Grid point, get the closest saddle point within range
	newGrid := make([]r2.Point, len(grid))
	copy(newGrid, grid)
	chosenSaddlePoints := make(map[r2.Point]bool)
	nPoints := len(grid)
	gridGood := make([]int, nPoints)

	for i, ptI := range grid {
		pt2, d := getMinSaddleDistance(saddlePoints, ptI)
		if _, ok := chosenSaddlePoints[pt2]; ok {
			d = maxPointDist
		} else {
			chosenSaddlePoints[pt2] = true
		}
		if d < maxPointDist {
			// replace Grid point with saddle point
			newGrid[i] = pt2
			gridGood[i] = 1
		}
	}

	return newGrid, gridGood
}

// ChessGrid stores the data necessary to get the chess grid points in an image
type ChessGrid struct {
	M            *mat.Dense // homography from ideal Grid to estimated Grid
	IdealGrid    []r2.Point // ideal Grid
	Grid         []r2.Point // detected chessboard Grid
	GoodPoints   []int      // if point in Grid is valid, GoodPoints[point] = 1, otherwise 0
	SaddlePoints []r2.Point // slice of saddle points detected in the first step of the chessboard detection algorithm
}

// sumGoodPoints returns the number of good points in the grid
func sumGoodPoints(goodPoints []int) int {
	sum := 0
	for _, pt := range goodPoints {
		sum += pt
	}
	return sum
}

// GreedyIterations performs greedy iterations to find the best fitted grid for chess board
func GreedyIterations(contours [][]r2.Point, saddlePoints []r2.Point, cfg ChessGreedyConfiguration) (*ChessGrid, error) {
	currentNGood := 0
	currentGridNext := make([]r2.Point, 0)
	currentGridGood := make([]int, 0)
	currentM := mat.NewDense(3, 3, nil)
	var currentGrid, idealGrid []r2.Point
	var M *mat.Dense
	var err error
	// iterate through contours
	for _, cnt := range contours {
		_, _, M, err = getInitialChessGrid(cnt)
		if err != nil {
			return nil, err
		}
		nGood := 0
		nextGrid := make([]r2.Point, 0)
		goodGrid := make([]int, 0)
		if M != nil {
			nGood = 0
			// iterate through possible positions of the Grid
			for gridI := 0; gridI < 7; gridI++ {
				idealGrid, currentGrid = makeChessGrid(M, gridI+1)
				nextGrid, goodGrid = findGoodPoints(currentGrid, saddlePoints, 15.0)

				nGood = sumGoodPoints(goodGrid)
				fmt.Println("nGood = ", nGood)
				if nGood < 4 {
				}
				if M == nil || math.Abs(M.At(0, 0)/M.At(1, 1)) > cfg.HomographyAcceptableScaleRatio || math.Abs(M.At(1, 1)/M.At(0, 0)) > cfg.HomographyAcceptableScaleRatio {
					M = nil
					break
				}
			}
		}

		// if M is nil, go directly to next contour
		if M == nil {
			continue
		} else if nGood > currentNGood {
			// current fit is better than previous best, store it instead
			currentNGood = nGood
			currentGridNext, currentGridGood = nextGrid, goodGrid
			currentM = M
		}
		// if current fit has more that MaxPointsNeeded, estimation is good enough, we can stop the iterations here
		if nGood > cfg.MaxPointsNeeded {
			break
		}
	}
	// if we found a relevant Grid estimation, return it
	if currentNGood > cfg.MinPointsNeeded {
		finalIdealGrid := getIdentityGrid(2+2*7, -7)
		return &ChessGrid{
			M:            currentM,
			IdealGrid:    finalIdealGrid,
			Grid:         currentGridNext,
			GoodPoints:   currentGridGood,
			SaddlePoints: saddlePoints,
		}, nil
	}
	return nil, nil
}
