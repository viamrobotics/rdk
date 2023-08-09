package tpspace

import (
	"testing"

	"go.viam.com/test"
)

var (
	defaultMps    = 0.3
	turnRadMeters = 0.3
)

func TestSim(t *testing.T) {
	for _, ptg := range defaultPTGs {
		radPS := defaultMps / turnRadMeters

		ptgGen := ptg(defaultMps, radPS, 1.)
		test.That(t, ptgGen, test.ShouldNotBeNil)
		_, err := NewPTGGridSim(ptgGen, defaultAlphaCnt, 1000.)
		test.That(t, err, test.ShouldBeNil)
	}
}
