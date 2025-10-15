package armplanning

import (
	"fmt"
	"math"
	"slices"
	"sort"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

const minJogPercent = .03

// linearizedFrameSystem wraps a framesystem, allowing conversion in a known order between a FrameConfiguratinos and a flat array of floats,
// useful for being able to call IK solvers against framesystems.
type linearizedFrameSystem struct {
	frames []referenceframe.Frame // cached ordering of frames. Order is unimportant but cannot change once set.
	dof    []referenceframe.Limit
}

func newLinearizedFrameSystem(fs *referenceframe.FrameSystem) (*linearizedFrameSystem, error) {
	frames := []referenceframe.Frame{}
	dof := []referenceframe.Limit{}

	frameNames := fs.FrameNames()
	sort.Strings(frameNames)
	for _, fName := range frameNames {
		frame := fs.Frame(fName)
		if frame == nil {
			return nil, fmt.Errorf("frame %s was returned in list of frame names, but was not found in frame system", fName)
		}
		frames = append(frames, frame)
		dof = append(dof, frame.DoF()...)
	}
	return &linearizedFrameSystem{
		frames: frames,
		dof:    dof,
	}, nil
}

// mapToSlice will flatten a map of inputs into a slice suitable for input to inverse kinematics, by concatenating
// the inputs together in the order of the frames in sf.frames.
func (lfs *linearizedFrameSystem) mapToSlice(inputs referenceframe.FrameSystemInputs) ([]float64, error) {
	var floatSlice []float64
	for _, frame := range lfs.frames {
		if len(frame.DoF()) == 0 {
			continue
		}
		input, ok := inputs[frame.Name()]
		if !ok {
			return nil, fmt.Errorf("frame %s missing from input map", frame.Name())
		}
		for _, i := range input {
			floatSlice = append(floatSlice, i.Value)
		}
	}
	return floatSlice, nil
}

func (lfs *linearizedFrameSystem) sliceToMap(floatSlice []float64) (referenceframe.FrameSystemInputs, error) {
	inputs := referenceframe.FrameSystemInputs{}
	i := 0
	for _, frame := range lfs.frames {
		if len(frame.DoF()) == 0 {
			continue
		}
		frameInputs := make([]referenceframe.Input, len(frame.DoF()))
		for j := range frame.DoF() {
			if i >= len(floatSlice) {
				return nil, fmt.Errorf("not enough values in float slice for frame %s", frame.Name())
			}
			frameInputs[j] = referenceframe.Input{Value: floatSlice[i]}
			i++
		}
		inputs[frame.Name()] = frameInputs
	}
	return inputs, nil
}

// return is floats from [0-1] given a percentage of their input range that should be searched
// for example, if the frame system has 2 arms, and only is moving, the inputs for the non-moving arm will all be 0
// the other arm will be scaled 0-1 based on the expected joint distance
// there is a chacne it's not enough and will need be moved more.
func (lfs *linearizedFrameSystem) inputChangeRatio(
	mc *motionChains,
	startNotMine referenceframe.FrameSystemInputs,
	distanceFunc motionplan.StateFSMetric,
	logger logging.Logger,
) []float64 {
	start := referenceframe.FrameSystemInputs{}
	for k, v := range startNotMine {
		start[k] = append([]referenceframe.Input{}, v...)
	}

	_, nonmoving := mc.framesFilteredByMovingAndNonmoving()

	startDistance := distanceFunc(&motionplan.StateFS{Configuration: start, FS: mc.fs})

	ratios := []float64{}

	for _, frame := range lfs.frames {
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
			orig := start[frame.Name()][idx]

			// Compute the new input for a specific joint that's one "jog" away. E.g: ~5 degrees for
			// a rotational joint.
			y := lfs.jog(len(ratios), orig.Value, percentJog)

			// Update the copied joint set in place. This is undone at the end of the loop.
			start[frame.Name()][idx] = referenceframe.Input{y}

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
			adjustedJogRatio := min(1, max(.03, myJogRatio*5))

			if math.IsNaN(adjustedJogRatio) {
				adjustedJogRatio = 1
			}

			logger.Debugf("idx: %d startDistance: %0.2f myDistance: %0.2f thisRatio: %0.4f myJogRatio: %0.4f adjustJogRatio: %0.4f",
				idx, startDistance, myDistance, thisRatio, myJogRatio, adjustedJogRatio)

			ratios = append(ratios, adjustedJogRatio)

			// Undo the above modification. Returning `start` back to its original state.
			start[frame.Name()][idx] = orig
		}
	}

	logger.Debugf("inputChangeRatio result: %v", ratios)

	return ratios
}

func (lfs *linearizedFrameSystem) jog(idx int, val, percentJog float64) float64 {
	r := lfs.dof[idx]

	x := r.Range() * percentJog

	val += x
	if val > r.Max {
		val -= (2 * x)
	}

	return val
}
