package armplanning

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
)

func TestNearestNeighbor(t *testing.T) {
	rrtMap := map[node]node{}

	j := &basicNode{q: referenceframe.FrameSystemInputs{"": {{0.0}}}}
	// We add ~110 nodes to the set of candidates. This is smaller than the configured
	// `parallelNeighbors` or 1000 meaning the `nearestNeighbor` call will be evaluated in series.
	for i := 1.0; i < 110.0; i++ {
		iSol := &basicNode{q: referenceframe.FrameSystemInputs{"": {{i}}}}
		rrtMap[iSol] = j
		j = iSol
	}

	seed := referenceframe.FrameSystemInputs{"": {{23.1}}}
	nn := nearestNeighbor(&basicNode{q: seed}, rrtMap, nodeConfigurationDistanceFunc)
	test.That(t, nn.Q()[""][0].Value, test.ShouldAlmostEqual, 23.0)

	// We add more nodes to trip the 1000 threshold. The `nearestNeighbor` call will use `nCPU` (2)
	// goroutines for evaluation.
	for i := 120.0; i < 1100.0; i++ {
		iSol := &basicNode{q: referenceframe.FrameSystemInputs{"": {{i}}}}
		rrtMap[iSol] = j
		j = iSol
	}
	seed = referenceframe.FrameSystemInputs{"": {{723.6}}}
	nn = nearestNeighbor(&basicNode{q: seed}, rrtMap, nodeConfigurationDistanceFunc)
	test.That(t, nn.Q()[""][0].Value, test.ShouldAlmostEqual, 724.0)
}

func BenchmarkNearestNeighbor(t *testing.B) {
	rrtMap := map[node]node{}

	j := &basicNode{q: referenceframe.FrameSystemInputs{"": {{0.0}}}}
	for i := 120.0; i < 11000.0; i++ {
		iSol := &basicNode{q: referenceframe.FrameSystemInputs{"": {{i}}}}
		rrtMap[iSol] = j
		j = iSol
	}
	seed := referenceframe.FrameSystemInputs{"": {{723.6}}}

	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		nn := nearestNeighbor(&basicNode{q: seed}, rrtMap, nodeConfigurationDistanceFunc)
		test.That(t, nn.Q()[""][0].Value, test.ShouldAlmostEqual, 724.0)
	}
}
