package calibrate

import (
	"testing"

	"go.viam.com/test"
	"gonum.org/v1/gonum/mat"
)

func TestCornersToMatrix(t *testing.T) {
	C := []Corner{
		{X: 300, Y: 50},
		{X: 220, Y: 60},
		{X: 100, Y: 100},
		{X: 200, Y: 151},
	}
	got := CornersToMatrix(C)
	r, c := got.Dims()
	test.That(t, r, test.ShouldEqual, 3)
	test.That(t, c, test.ShouldEqual, 4)
}

func TestDoLM(t *testing.T) {
	I := CornersToMatrix([]Corner{{X: 300, Y: 50}, {X: 220, Y: 60}})
	W := CornersToMatrix([]Corner{{X: 100, Y: 100}, {X: 200, Y: 151}})
	Hbad := mat.NewVecDense(6, []float64{1, 2.3, 4, 5.6, 7, 8.9})
	Hgood := mat.NewVecDense(9, []float64{1, 2.3, 4, 13, 8.7, 6.03, 5.6, 7, 8.9})

	got1, err := DoLM(Hbad, I, W)
	got2, err2 := DoLM(Hgood, I, W)
	r, c := got2.Dims()

	test.That(t, got1, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeError)
	test.That(t, r, test.ShouldEqual, 3)
	test.That(t, c, test.ShouldEqual, 3)
	test.That(t, err2, test.ShouldBeNil)
}

func TestBuildH(t *testing.T) {
	I := []Corner{{X: 300, Y: 50}, {X: 220, Y: 60}, {X: 100, Y: 100}, {X: 200, Y: 151}}
	W := []Corner{{X: 230, Y: 90}, {X: 20, Y: 160}, {X: 10, Y: 260}, {X: 50, Y: 150}}
	got := BuildH(I, W)
	test.That(t, got.Len(), test.ShouldEqual, 9)
}

func TestGetV(t *testing.T) {
	H1 := mat.NewVecDense(9, []float64{1, 2.3, 4, 13, 8.7, 6.03, 5.6, 87, 8.9})
	H2 := mat.NewVecDense(9, []float64{1, 0.3, 4, 23, 8.7, 1.23, 5.6, 7, 393.9})
	H3 := mat.NewVecDense(9, []float64{0, 3, 4, 1.3, 8.7, 6.3, 525600, 7, 14.9})

	got := GetV(H1, H2, H3)
	r, c := got.Dims()

	test.That(t, r, test.ShouldEqual, 6)
	test.That(t, c, test.ShouldEqual, 6)
}

func TestBuildBFromV(t *testing.T) {
	V := mat.NewDense(6, 6, []float64{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22,
		23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36,
	})

	got, _ := BuildBFromV(V)
	test.That(t, got.Len(), test.ShouldEqual, 6)
}

func TestGetIntrinsicsFromB(t *testing.T) {
	B := mat.NewVecDense(6, []float64{1, 2, 3, 4, 5, 6})

	got := GetIntrinsicsFromB(B)
	test.That(t, len(got), test.ShouldEqual, 6)
	test.That(t, got, test.ShouldResemble, []float64{-3, -7, 2.6457513110645907, 2.6457513110645907, 5.291502622129182, -2})
}
