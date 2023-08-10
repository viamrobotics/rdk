package tpspace

//~ import (
	//~ "testing"

	//~ "go.viam.com/test"
//~ )


//~ type ptgFactory func(float64, float64) tpspace.PrecomputePTG

//~ var defaultPTGs = []ptgFactory{
	//~ NewCirclePTG,
	//~ NewCCPTG,
	//~ NewCCSPTG,
	//~ NewCSPTG,
//~ }

//~ var (
	//~ defaultMps    = 0.3
	//~ turnRadMeters = 0.3
//~ )

//~ func TestSim(t *testing.T) {
	//~ for _, ptg := range defaultPTGs {
		//~ radPS := defaultMps / turnRadMeters

		//~ ptgGen := ptg(defaultMps, radPS, 1.)
		//~ test.That(t, ptgGen, test.ShouldNotBeNil)
		//~ _, err := NewPTGGridSim(ptgGen, defaultAlphaCnt, 1000.)
		//~ test.That(t, err, test.ShouldBeNil)
	//~ }
//~ }
