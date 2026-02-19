package matrix

import (
	"math"

	"gonum.org/v1/gonum/stat/distuv"
)

// SampleNIntegersNormal samples n integers from normal distribution centered around (vMax+vMin) / 2
// and in range [vMin, vMax].
func SampleNIntegersNormal(n int, vMin, vMax float64) []int {
	z := make([]int, n)
	// get normal distribution centered on (vMax+vMin) / 2 and whose sampled are mostly in [vMin, vMax] (var=0.1)
	mean := (vMax + vMin) / 2
	dist := distuv.Normal{
		Mu:    mean,
		Sigma: (vMax - vMin) * 0.4472,
	}
	for i := range z {
		val := math.Round(dist.Rand())
		for val < vMin || val > vMax {
			val = math.Round(dist.Rand())
		}
		z[i] = int(val)
	}

	return z
}

// SampleNIntegersUniform samples n integers uniformly in [vMin, vMax].
func SampleNIntegersUniform(n int, vMin, vMax float64) []int {
	z := make([]int, n)
	// get uniform distribution on [vMin, vMax]
	dist := distuv.Uniform{
		Min: vMin,
		Max: vMax,
	}
	for i := range z {
		val := math.Round(dist.Rand())
		for val < vMin || val > vMax {
			val = math.Round(dist.Rand())
		}
		z[i] = int(val)
	}

	return z
}
