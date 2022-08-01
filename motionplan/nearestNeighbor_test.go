package motionplan

import (
	"context"
	"testing"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/test"
)

func TestNearestNeighbor(t *testing.T) {
	nm := &neighborManager{nCPU: 2}
	rrtMap := map[*configuration]*configuration{}

	j := &configuration{[]referenceframe.Input{{0.0}}}
	for i := 1.0; i < 110.0; i++ {
		iSol := &configuration{[]referenceframe.Input{{i}}}
		rrtMap[iSol] = j
		j = iSol
	}
	ctx := context.Background()

	seed := &configuration{[]referenceframe.Input{{23.1}}}
	// test serial NN
	nn := nm.nearestNeighbor(ctx, seed, rrtMap)
	test.That(t, nn.inputs[0].Value, test.ShouldAlmostEqual, 23.0)

	for i := 120.0; i < 1100.0; i++ {
		iSol := &configuration{[]referenceframe.Input{{i}}}
		rrtMap[iSol] = j
		j = iSol
	}
	seed = &configuration{[]referenceframe.Input{{723.6}}}
	// test parallel NN
	nn = nm.nearestNeighbor(ctx, seed, rrtMap)
	test.That(t, nn.inputs[0].Value, test.ShouldAlmostEqual, 724.0)
}
