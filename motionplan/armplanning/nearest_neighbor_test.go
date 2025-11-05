package armplanning

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
)

func TestNearestNeighbor(t *testing.T) {
	rrtMap := rrtMap{}

	j := &node{inputs: referenceframe.FrameSystemInputs{"": {0.0}}.ToLinearInputs()}
	// We add ~110 nodes to the set of candidates. This is smaller than the configured
	// `parallelNeighbors` or 1000 meaning the `nearestNeighbor` call will be evaluated in series.
	for i := 1.0; i < 110.0; i++ {
		iSol := &node{inputs: referenceframe.FrameSystemInputs{"": {i}}.ToLinearInputs()}
		rrtMap[iSol] = j
		j = iSol
	}

	seed := referenceframe.FrameSystemInputs{"": {23.1}}.ToLinearInputs()
	nn := nearestNeighbor(&node{inputs: seed}, rrtMap, nodeConfigurationDistanceFunc)
	test.That(t, nn.inputs.Get("")[0], test.ShouldAlmostEqual, 23.0)

	// We add more nodes to trip the 1000 threshold. The `nearestNeighbor` call will use `nCPU` (2)
	// goroutines for evaluation.
	for i := 120.0; i < 1100.0; i++ {
		iSol := &node{inputs: referenceframe.FrameSystemInputs{"": {i}}.ToLinearInputs()}
		rrtMap[iSol] = j
		j = iSol
	}
	seed = referenceframe.FrameSystemInputs{"": {723.6}}.ToLinearInputs()
	nn = nearestNeighbor(&node{inputs: seed}, rrtMap, nodeConfigurationDistanceFunc)
	test.That(t, nn.inputs.Get("")[0], test.ShouldAlmostEqual, 724.0)
}

func BenchmarkNearestNeighbor(t *testing.B) {
	rrtMap := rrtMap{}

	j := &node{inputs: referenceframe.FrameSystemInputs{"": {0.0}}.ToLinearInputs()}
	for i := 120.0; i < 11000.0; i++ {
		iSol := &node{inputs: referenceframe.FrameSystemInputs{"": {i}}.ToLinearInputs()}
		rrtMap[iSol] = j
		j = iSol
	}
	seed := referenceframe.FrameSystemInputs{"": {723.6}}.ToLinearInputs()

	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		nn := nearestNeighbor(&node{inputs: seed}, rrtMap, nodeConfigurationDistanceFunc)
		test.That(t, nn.inputs.Get("")[0], test.ShouldAlmostEqual, 724.0)
	}
}
