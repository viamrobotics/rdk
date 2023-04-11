package motionplan

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
)

func TestNearestNeighbor(t *testing.T) {
	nm := &neighborManager{nCPU: 2}
	rrtMap := map[node]node{}

	j := &basicNode{q: []referenceframe.Input{{0.0}}}
	for i := 1.0; i < 110.0; i++ {
		iSol := &basicNode{q: []referenceframe.Input{{i}}}
		rrtMap[iSol] = j
		j = iSol
	}
	ctx := context.Background()
	m1chan := make(chan node, 1)
	defer close(m1chan)

	seed := []referenceframe.Input{{23.1}}
	// test serial NN
	opt := newBasicPlannerOptions()
	utils.PanicCapturingGo(func() {
		nm.nearestNeighbor(ctx, opt, seed, rrtMap, m1chan)
	})
	nn := <-m1chan
	test.That(t, nn.Q()[0].Value, test.ShouldAlmostEqual, 23.0)

	for i := 120.0; i < 1100.0; i++ {
		iSol := &basicNode{q: []referenceframe.Input{{i}}}
		rrtMap[iSol] = j
		j = iSol
	}
	seed = []referenceframe.Input{{723.6}}
	// test parallel NN
	utils.PanicCapturingGo(func() {
		nm.nearestNeighbor(ctx, opt, seed, rrtMap, m1chan)
	})
	nn = <-m1chan
	test.That(t, nn.Q()[0].Value, test.ShouldAlmostEqual, 724.0)
}
