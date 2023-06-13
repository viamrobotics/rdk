package tpspace

import (
	"testing"

	"go.viam.com/test"
)

var defaultPTGs = []func(float64, float64, float64) PrecomputePTG{
	NewCirclePTG,
	NewCCPTG,
	NewCCSPTG,
	NewCSPTG,
	NewAlphaPTG,
}

var (
	defaultMps = 1.
	defaultDps = 45.
)

func TestSim(t *testing.T) {
	
	for _, ptg := range defaultPTGs {
		ptgGen := ptg(defaultMps, defaultDps, 1.)
		test.That(t, ptgGen, test.ShouldNotBeNil)
		_, err := NewPTGGridSim(ptgGen, defaultAlphaCnt, 1000.)
		test.That(t, err, test.ShouldBeNil)
	}
}
