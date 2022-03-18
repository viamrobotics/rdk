package utils

import (
	"math"
	"testing"

	"go.viam.com/test"
)

func TestComputeDistance(t *testing.T) {
	v1 := []float64{1, 0, 1}
	d1Euclidean, err := ComputeDistance(v1, v1, Euclidean)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, d1Euclidean, test.ShouldEqual, 0)

	d1Hamming, err := ComputeDistance(v1, v1, Hamming)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, d1Hamming, test.ShouldEqual, 0)

	v2 := []float64{1, 1, 1}

	d2Euclidean, err := ComputeDistance(v1, v2, Euclidean)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, d2Euclidean, test.ShouldEqual, 1)

	d2Hamming, err := ComputeDistance(v1, v2, Hamming)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, d2Hamming, test.ShouldEqual, 1)

	v3 := []float64{1, 0, 0}
	d3Euclidean, err := ComputeDistance(v2, v3, Euclidean)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, d3Euclidean, test.ShouldEqual, math.Sqrt(2))

	d3Hamming, err := ComputeDistance(v2, v3, Hamming)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, d3Hamming, test.ShouldEqual, 2)
}

func TestPairwiseDistance(t *testing.T) {
	p1 := [][]float64{
		{1, 0, 1},
		{1, 1, 1},
		{1, 0, 0},
	}

	p2 := [][]float64{
		{1, 0, 0},
		{1, 1, 0},
		{1, 1, 0},
		{0, 0, 0},
	}

	distancesEuclidean, err := PairwiseDistance(p1, p2, Euclidean)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, distancesEuclidean, test.ShouldNotBeNil)
	m, n := distancesEuclidean.Dims()
	test.That(t, m, test.ShouldEqual, 3)
	test.That(t, n, test.ShouldEqual, 4)
	test.That(t, distancesEuclidean.At(0, 0), test.ShouldEqual, 1)

	distancesHamming, err := PairwiseDistance(p1, p2, Hamming)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, distancesHamming, test.ShouldNotBeNil)
	m2, n2 := distancesHamming.Dims()
	test.That(t, m2, test.ShouldEqual, 3)
	test.That(t, n2, test.ShouldEqual, 4)
	test.That(t, distancesHamming.At(0, 0), test.ShouldEqual, 1)

	minIdx := GetArgMinDistancesPerRow(distancesHamming)
	test.That(t, len(minIdx), test.ShouldEqual, 3)
	test.That(t, minIdx[0], test.ShouldEqual, 0)
	test.That(t, minIdx[1], test.ShouldEqual, 1)
	test.That(t, minIdx[2], test.ShouldEqual, 0)
}
