package utils

import (
	"errors"
	"math"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

// DistanceType defines the type of distance used in a function.
type DistanceType int

const (
	// Euclidean is DistanceType 0.
	Euclidean DistanceType = iota
	// Hamming is DistanceType 1.
	Hamming
)

// ComputeDistance computes the distance between two vectors stored in a slice of floats.
func ComputeDistance(p1, p2 []float64, distType DistanceType) (float64, error) {
	switch distType {
	case Euclidean:
		return EuclideanDistance(p1, p2)
	case Hamming:
		return HammingDistance(p1, p2)
	default:
		return EuclideanDistance(p1, p2)
	}
}

// PairwiseDistance computes the pairwise distances between 2 sets of points.
func PairwiseDistance(pts1, pts2 [][]float64, distType DistanceType) (*mat.Dense, error) {
	m := len(pts1)
	n := len(pts2)
	distances := mat.NewDense(m, n, nil)

	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			d, err := ComputeDistance(pts1[i], pts2[j], distType)
			if err != nil {
				return nil, err
			}
			distances.Set(i, j, d)
		}
	}
	return distances, nil
}

// GetArgMinDistancesPerRow returns in a slice of int the index of the point with minimum distance for each row.
func GetArgMinDistancesPerRow(distances *mat.Dense) []int {
	nRows, _ := distances.Dims()
	indices := make([]int, nRows)
	for i := 0; i < nRows; i++ {
		row := mat.Row(nil, i, distances)
		indices[i] = floats.MinIdx(row)
	}
	return indices
}

// Transpose transposes the slice of slice of ints.
func Transpose(slice [][]int) [][]int {
	xl := len(slice[0])
	yl := len(slice)
	result := make([][]int, xl)
	for i := range result {
		result[i] = make([]int, yl)
	}
	for i := 0; i < xl; i++ {
		for j := 0; j < yl; j++ {
			result[i][j] = slice[j][i]
		}
	}
	return result
}

// GetArgMinDistancesPerRowInt returns in a slice of int the index of the point with minimum distance for each row.
func GetArgMinDistancesPerRowInt(distances [][]int) []int {
	nRows := len(distances)
	indices := make([]int, nRows)
	for j := 0; j < nRows; j++ {
		m := 0
		mIndex := 0
		for i, e := range distances[j] {
			if i == 0 || e < m {
				m = e
				mIndex = i
			}
		}
		indices[j] = mIndex
	}
	return indices
}

// HammingDistance computes the hamming distance between two vectors that only contain zeros and ones.
func HammingDistance(p1, p2 []float64) (float64, error) {
	distance := 0
	if len(p1) != len(p2) {
		return -1, errors.New("must have same length")
	}

	for i := range p1 {
		if p1[i] != p2[i] {
			distance++
		}
	}
	return float64(distance), nil
}

// EuclideanDistance computes the euclidean distance between 2 vectors.
func EuclideanDistance(p1, p2 []float64) (float64, error) {
	if len(p1) != len(p2) {
		return -1, errors.New("must have same length")
	}
	diff := make([]float64, len(p1))
	floats.SubTo(diff, p1, p2)
	// squared diff vector
	floats.Mul(diff, diff)
	// sum squared components
	distSquared := floats.Sum(diff)

	return math.Sqrt(distSquared), nil
}
