package armplanning

import (
	"math"
	"slices"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

// searchHeadroom is the multiplier applied to raw joint sensitivity ratios when computing
// search ranges. A value of 5 means we search 5x the estimated joint range needed to reach
// the goal, providing headroom for the non-linearity of the distance function.
const searchHeadroom = 5.0

// computeJointSensitivities returns a per-joint sensitivity ratio indicating what fraction of each
// joint's total range is estimated to be needed to reach the goal. Non-moving joints are indicated
// with a value of -1. Use clampSensitivities to convert raw ratios into usable search bounds.
//
// For each moving joint, a small perturbation (1% of range) is applied to estimate sensitivity.
func computeJointSensitivities(
	mc *motionChains,
	startNotMine *referenceframe.LinearInputs,
	frameSystem *referenceframe.FrameSystem,
	distanceFunc motionplan.StateFSMetric,
	logger logging.Logger,
) ([]float64, error) {
	inputsSchema, err := startNotMine.GetSchema(frameSystem)
	if err != nil {
		return nil, err
	}

	rawRatios := []float64{}

	// Sorry for the hacky copy.
	start := startNotMine.ToFrameSystemInputs().ToLinearInputs()
	_, nonmoving := mc.framesFilteredByMovingAndNonmoving()
	startDistance := distanceFunc(&motionplan.StateFS{Configuration: startNotMine, FS: mc.fs})
	logger.Debugf("startDistance: %0.2f", startDistance)

	const percentJog = 0.01

	for _, frameName := range inputsSchema.FrameNamesInOrder() {
		frame := frameSystem.Frame(frameName)
		if len(frame.DoF()) == 0 {
			// Frames without degrees of freedom can't move.
			continue
		}

		if slices.Contains(nonmoving, frame.Name()) {
			// Frames that can move, but we are not moving them to solve this problem.
			for range frame.DoF() {
				rawRatios = append(rawRatios, -1)
			}
			continue
		}

		// For each degree of freedom, we want to determine how much impact a small change
		// makes. For cases where a small movement results in a big change in distance, we want to
		// walk in smaller steps. For cases where a small change has a small effect, we want to
		// allow the walking algorithm to take bigger steps.
		for idx := range frame.DoF() {
			linearIdx := len(rawRatios)
			orig := start.Get(frame.Name())[idx]

			// Compute the new input for a specific joint that's one "jog" away. E.g: ~5 degrees for
			// a rotational joint.
			y := inputsSchema.Jog(linearIdx, orig, percentJog)

			// Update the copied joint set in place. This is undone at the end of the loop.
			start.Get(frame.Name())[idx] = y

			myDistance := distanceFunc(&motionplan.StateFS{Configuration: start, FS: mc.fs})
			// Compute how much effect the small change made. The bigger the difference, the smaller
			// the ratio.
			//
			// Note that Go deals with the potential divide by 0. Representing `thisRatio` as
			// infinite. The following comparisons continue to work as expected.
			thisRatio := startDistance / math.Abs(myDistance-startDistance)
			myJogRatio := percentJog * thisRatio

			logger.Debugf("idx: %d myDistance: %0.2f thisRatio: %0.3f myJogRatio: %0.3f",
				linearIdx, myDistance, thisRatio, myJogRatio)

			rawRatios = append(rawRatios, myJogRatio)

			// Undo the above modification. Returning `start` back to its original state.
			start.Get(frame.Name())[idx] = orig
		}
	}

	logger.Debugf("computeJointSensitivities result: %v", rawRatios)
	return rawRatios, nil
}

// clampSensitivities converts raw per-joint sensitivity ratios (from computeJointSensitivities)
// into bounded search range ratios. Non-moving joints (indicated by -1) are set to 0.
// Moving joints are scaled by searchHeadroom and clamped to [minJog, 1.0].
func clampSensitivities(rawRatios []float64, minJog float64) []float64 {
	ratios := make([]float64, len(rawRatios))
	for i, raw := range rawRatios {
		if raw < 0 {
			// Non-moving joint sentinel.
			continue
		}
		ratios[i] = min(1, max(minJog, raw*searchHeadroom))
		if math.IsNaN(ratios[i]) {
			ratios[i] = 1
		}
	}
	return ratios
}
