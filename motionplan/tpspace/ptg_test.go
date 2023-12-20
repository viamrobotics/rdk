package tpspace

import (
	"math"
	"testing"

	"go.viam.com/test"
)

func TestAlphaIdx(t *testing.T) {
	for i := uint(0); i < defaultAlphaCnt; i++ {
		alpha := index2alpha(i, defaultAlphaCnt)
		i2 := alpha2index(alpha, defaultAlphaCnt)
		test.That(t, i, test.ShouldEqual, i2)
	}
}

func alpha2index(alpha float64, numPaths uint) uint {
	alpha = wrapTo2Pi(alpha+math.Pi) - math.Pi
	idx := uint(math.Round(0.5 * (float64(numPaths)*(1.0+alpha/math.Pi) - 1.0)))
	return idx
}
