package armplanning

import (
	"math"
	"slices"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

// return is floats from [0-1] given a percentage of their input range that should be searched
// for example, if the frame system has 2 arms, and only is moving, the inputs for the non-moving arm will all be 0
// the other arm will be scaled 0-1 based on the expected joint distance
// there is a chacne it's not enough and will need be moved more.
func inputChangeRatio(
	mc *motionChains,
	startNotMine *referenceframe.LinearInputs,
	frameSystem *referenceframe.FrameSystem,
	distanceFunc motionplan.StateFSMetric,
	minJog float64,
	logger logging.Logger,
) ([]float64, error) {
	inputsSchema, err := startNotMine.GetSchema(frameSystem)
	if err != nil {
		return nil, err
	}

	ratios := []float64{}

	// Sorry for the hacky copy.
	start := startNotMine.ToFrameSystemInputs().ToLinearInputs()
	_, nonmoving := mc.framesFilteredByMovingAndNonmoving()
	startDistance := distanceFunc(&motionplan.StateFS{Configuration: startNotMine, FS: mc.fs})
	logger.Debugf("startDistance: %0.2f", startDistance)

	for _, frameName := range inputsSchema.FrameNamesInOrder() {
		frame := frameSystem.Frame(frameName)
		if len(frame.DoF()) == 0 {
			// Frames without degrees of freedom can't move.
			continue
		}

		if slices.Contains(nonmoving, frame.Name()) {
			// Frames that can move, but we are not moving them to solve this problem.
			for range frame.DoF() {
				ratios = append(ratios, 0)
			}
			continue
		}
		const percentJog = 0.01

		// For each degree of freedom, we want to determine how much impact a small change
		// makes. For cases where a small movement results in a big change in distance, we want to
		// walk in smaller steps. For cases where a small change has a small effect, we want to
		// allow the walking algorithm to take bigger steps.
		for idx := range frame.DoF() {
			orig := start.Get(frame.Name())[idx]

			// Compute the new input for a specific joint that's one "jog" away. E.g: ~5 degrees for
			// a rotational joint.
			y := inputsSchema.Jog(len(ratios), orig, percentJog)

			// Update the copied joint set in place. This is undone at the end of the loop.
			start.Get(frame.Name())[idx] = y

			myDistance := distanceFunc(&motionplan.StateFS{Configuration: start, FS: mc.fs})
			// Compute how much effect the small change made. The bigger the difference, the smaller
			// the ratio.
			//
			// Note that Go deals with the potential divide by 0. Representing `thisRatio` as
			// infinite. The following comparisons continue to work as expected. Resulting in an
			// adjusted jog ratio of 1.
			thisRatio := startDistance / math.Abs(myDistance-startDistance)
			myJogRatio := percentJog * thisRatio
			// For movable frames/joints, 0.03 is the actual smallest value we'll use.
			adjustedJogRatio := min(1, max(minJog, (myJogRatio*5)))

			if math.IsNaN(adjustedJogRatio) {
				adjustedJogRatio = 1
			}

			logger.Debugf("idx: %d myDistance: %0.2f thisRatio: %0.3f myJogRatio: %0.3f adjustJogRatio: %0.3f",
				idx, myDistance, thisRatio, myJogRatio, adjustedJogRatio)

			ratios = append(ratios, adjustedJogRatio)

			// Undo the above modification. Returning `start` back to its original state.
			start.Get(frame.Name())[idx] = orig
		}
	}

	logger.Debugf("inputChangeRatio result: %v", ratios)

	return ratios, nil
}
