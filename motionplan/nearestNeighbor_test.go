package motionplan

import (
	"context"
	"math"
	"runtime"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
)

var nCPU = int(math.Max(1.0, float64(runtime.NumCPU()/4)))

func TestNearestNeighbor(t *testing.T) {
	nm := &neighborManager{nCPU: 2, parallelNeighbors: 1000}
	rrtMap := map[node]node{}

	j := &basicNode{q: []referenceframe.Input{{0.0}}}
	// We add ~110 nodes to the set of candidates. This is smaller than the configured
	// `parallelNeighbors` or 1000 meaning the `nearestNeighbor` call will be evaluated in series.
	for i := 1.0; i < 110.0; i++ {
		iSol := &basicNode{q: []referenceframe.Input{{i}}}
		rrtMap[iSol] = j
		j = iSol
	}
	ctx := context.Background()

	seed := []referenceframe.Input{{23.1}}
	opt := newBasicPlannerOptions(referenceframe.NewZeroStaticFrame("test-frame"))
	nn := nm.nearestNeighbor(ctx, opt, &basicNode{q: seed}, rrtMap)
	test.That(t, nn.Q()[0].Value, test.ShouldAlmostEqual, 23.0)

	// We add more nodes to trip the 1000 threshold. The `nearestNeighbor` call will use `nCPU` (2)
	// goroutines for evaluation.
	for i := 120.0; i < 1100.0; i++ {
		iSol := &basicNode{q: []referenceframe.Input{{i}}}
		rrtMap[iSol] = j
		j = iSol
	}
	seed = []referenceframe.Input{{723.6}}
	nn = nm.nearestNeighbor(ctx, opt, &basicNode{q: seed}, rrtMap)
	test.That(t, nn.Q()[0].Value, test.ShouldAlmostEqual, 724.0)
}
