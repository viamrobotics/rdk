package chessboard

import (
	"math/rand"
	"testing"

	"github.com/golang/geo/r2"
	"go.viam.com/test"
	"gonum.org/v1/gonum/mat"
)

var NTESTRANDOM = 5

func Create3IdentityMatrix() *mat.Dense {
	I := mat.NewDense(3, 3, nil)
	I.Set(0, 0, 1)
	I.Set(1, 1, 1)
	I.Set(2, 2, 1)
	return I
}

func TestGetIdentityGrid(t *testing.T) {
	grid1 := getIdentityGrid(2, 0)
	test.That(t, len(grid1), test.ShouldEqual, 4)
	test.That(t, grid1[0], test.ShouldResemble, r2.Point{0, 0})
	test.That(t, grid1[1], test.ShouldResemble, r2.Point{1, 0})
	test.That(t, grid1[2], test.ShouldResemble, r2.Point{0, 1})
	test.That(t, grid1[3], test.ShouldResemble, r2.Point{1, 1})

	grid2 := getIdentityGrid(4, -1)
	test.That(t, len(grid2), test.ShouldEqual, 16)
	test.That(t, grid2[0], test.ShouldResemble, r2.Point{-1, -1})
	test.That(t, grid2[1], test.ShouldResemble, r2.Point{0, -1})
	test.That(t, grid2[14], test.ShouldResemble, r2.Point{1, 2})
	test.That(t, grid2[15], test.ShouldResemble, r2.Point{2, 2})
}

func TestMakeChessGrid(t *testing.T) {
	// test that both grids are equal if homography is the identity matrix
	H := mat.NewDense(3, 3, nil)
	H.Set(0, 0, 1)
	H.Set(1, 1, 1)
	H.Set(2, 2, 1)
	gridUnit, gridTransformed := makeChessGrid(H, 4)
	test.That(t, len(gridTransformed), test.ShouldEqual, 100)
	test.That(t, gridTransformed[0], test.ShouldResemble, gridUnit[0])
	// test for random indices
	for i := 0; i < NTESTRANDOM; i++ {
		randomIndex := rand.Intn(len(gridUnit))
		test.That(t, gridTransformed[randomIndex], test.ShouldResemble, gridUnit[randomIndex])
	}

	// test for a 90 degrees ration counter-clockwise : R(x, y) = (-y, x)
	R := mat.NewDense(3, 3, nil)
	R.Set(0, 1, -1)
	R.Set(1, 0, 1)
	R.Set(2, 2, 1)
	_, gridRotated := makeChessGrid(R, 4)
	test.That(t, len(gridRotated), test.ShouldEqual, 100)
	test.That(t, gridRotated[0], test.ShouldResemble, r2.Point{4, -4})
	// test for random indices
	for i := 0; i < NTESTRANDOM; i++ {
		randomIndex := rand.Intn(len(gridUnit))
		ptUnit := gridUnit[randomIndex]
		test.That(t, gridRotated[randomIndex], test.ShouldResemble, r2.Point{-ptUnit.Y, ptUnit.X})
	}
}

func TestGetInitialChessGrid(t *testing.T) {
	// Should return same grid twice and identity matrix
	idealGrid, grid, H, err := getInitialChessGrid(quadI)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(grid), test.ShouldEqual, 16)
	test.That(t, len(idealGrid), test.ShouldEqual, 16)
	// test for random indices
	for i := 0; i < NTESTRANDOM; i++ {
		randomIndex := rand.Intn(len(grid))
		test.That(t, idealGrid[randomIndex].X, test.ShouldAlmostEqual, grid[randomIndex].X)
		test.That(t, idealGrid[randomIndex].Y, test.ShouldAlmostEqual, grid[randomIndex].Y)
	}
	// test H = I
	I := Create3IdentityMatrix()
	test.That(t, mat.EqualApprox(H, I, 0.0000001), test.ShouldBeTrue)

	// translated quad
	quadTranslated := []r2.Point{{2, 2}, {3, 2}, {3, 3}, {2, 3}}
	_, gridT, T, err := getInitialChessGrid(quadTranslated)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(gridT), test.ShouldEqual, 16)
	// create GT translation by (2,2) matrix
	T2 := Create3IdentityMatrix()
	T2.Set(0, 2, 2)
	T2.Set(1, 2, 2)
	test.That(t, mat.EqualApprox(T, T2, 0.0000001), test.ShouldBeTrue)
}

func TestSumGoodPoints(t *testing.T) {
	goodPoints1 := []int{0, 0, 0, 0, 0, 0}
	sum1 := sumGoodPoints(goodPoints1)
	test.That(t, sum1, test.ShouldEqual, 0)

	goodPoints2 := []int{0, 0, 1, 0, 0, 0}
	sum2 := sumGoodPoints(goodPoints2)
	test.That(t, sum2, test.ShouldEqual, 1)

	goodPoints3 := []int{1, 1, 1, 1, 1, 1}
	sum3 := sumGoodPoints(goodPoints3)
	test.That(t, sum3, test.ShouldEqual, 6)
}
