package utils

import (
	"math"
	"sort"

	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

type Vec2Matrix mat.Dense

func (v2m *Vec2Matrix) RotateMatrixAbout(x, y, theta float64) *Vec2Matrix {
	if (*mat.Dense)(v2m).IsEmpty() {
		return v2m
	}
	thetaRad := DegToRad(AntiCWDeg(theta))
	rot := vec2RotationMatrixAbout(x, y, thetaRad)
	var rotated mat.Dense
	rotated.Mul(rot, (*mat.Dense)(v2m))
	return (*Vec2Matrix)(&rotated)
}

func vec2RotationMatrixAbout(x, y, theta float64) mat.Matrix {
	tNeg1 := mat.NewDense(3, 3, []float64{
		1, 0, x,
		0, 1, y,
		0, 0, 1,
	})
	rot := mat.NewDense(3, 3, []float64{
		math.Cos(theta), -math.Sin(theta), 0,
		math.Sin(theta), math.Cos(theta), 0,
		0, 0, 1,
	})
	t := mat.NewDense(3, 3, []float64{
		1, 0, -x,
		0, 1, -y,
		0, 0, 1,
	})
	var rotFinal mat.Dense
	rotFinal.Product(tNeg1, rot, t)
	return &rotFinal
}

func (v2m *Vec2Matrix) DistanceMSETo(to *Vec2Matrix) float64 {
	_, fromLen := (*mat.Dense)(v2m).Dims()
	_, toLen := (*mat.Dense)(to).Dims()
	compareFrom := (*mat.Dense)(v2m)
	compareTo := (*mat.Dense)(to)

	min := math.MaxFloat64
	for i := 0; i < fromLen; i++ {
		v := compareFrom.At(0, i)
		if v < min {
			min = v
		}
	}

	if fromLen < toLen {
		compareFrom = mat.DenseCopyOf(compareFrom).Grow(0, toLen-fromLen).(*mat.Dense)
	} else if fromLen > toLen {
		compareTo = mat.DenseCopyOf(compareTo).Grow(0, fromLen-toLen).(*mat.Dense)
	}

	compareFrom = sortMat(compareFrom)
	compareTo = sortMat(compareTo)

	var subbed mat.Dense
	subbed.Sub(compareFrom, compareTo)

	var powwed mat.Dense
	powwed.MulElem(&subbed, &subbed)

	var plussed mat.Dense
	plussed.Add(powwed.RowView(0), powwed.RowView(1))

	var rooted mat.Dense
	rooted.Apply(func(i, j int, v float64) float64 { return math.Sqrt(v) }, &plussed)

	return stat.Mean(rooted.RawRowView(0), nil)
}

type Vec2Fs [][]float64

func (vs Vec2Fs) Len() int {
	return len(vs)
}

func (vs Vec2Fs) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

func (vs Vec2Fs) Less(i, j int) bool {
	if vs[i][0] < vs[j][0] {
		return true
	}
	if vs[i][0] > vs[j][0] {
		return false
	}
	return vs[i][1] < vs[j][1]
}

func sortMat(target *mat.Dense) *mat.Dense {
	numCols := target.RowView(0).Len()
	cols := make([][]float64, 0, target.RowView(0).Len())
	targetT := mat.DenseCopyOf(target.T())
	for i := 0; i < numCols; i++ {
		cols = append(cols, targetT.RawRowView(i))
	}
	sort.Sort(Vec2Fs(cols))
	r, c := target.Dims()
	sortedMat := mat.NewDense(r, c, nil)
	for i := 0; i < numCols; i++ {
		sortedMat.SetCol(i, cols[i])
	}
	return sortedMat
}
