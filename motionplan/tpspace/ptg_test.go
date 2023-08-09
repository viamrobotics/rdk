package tpspace

import (
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
