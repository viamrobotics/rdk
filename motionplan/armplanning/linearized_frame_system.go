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
	start referenceframe.FrameSystemInputs,
	distanceFunc motionplan.StateFSMetric,
	logger logging.Logger,
) []float64 {
	_, nonmoving := mc.framesFilteredByMovingAndNonmoving()

	startDistance := distanceFunc(&motionplan.StateFS{Configuration: start, FS: mc.fs})

	ratios := []float64{}

	for _, frame := range lfs.frames {
		if len(frame.DoF()) == 0 {
			continue
		}

		if slices.Contains(nonmoving, frame.Name()) {
			for range frame.DoF() {
				ratios = append(ratios, 0)
			}
			continue
		}
		const percentJog = .01

		for idx, r := range frame.DoF() {
			orig := start[frame.Name()][idx]

			x := r.Range() * percentJog
			y := orig.Value + x
			if y > r.Max {
				y -= (2 * x)
			}

			start[frame.Name()][idx] = referenceframe.Input{y}

			myDistance := distanceFunc(&motionplan.StateFS{Configuration: start, FS: mc.fs})
			thisRatio := startDistance / math.Abs(myDistance-startDistance)
			myJogRatio := percentJog * thisRatio
			adjustLogRatio := min(1, max(.03, myJogRatio*5))
			logger.Debugf("idx: %d startDistance: %0.2f myDistance: %0.2f thisRatio: %0.4f myJogRatio: %0.4f adjustLogRatio: %0.4f",
				idx, startDistance, myDistance, thisRatio, myJogRatio, adjustLogRatio)

			ratios = append(ratios, adjustLogRatio)

			start[frame.Name()][idx] = orig
		}
	}

	logger.Debugf("inputChangeRatio result: %v", ratios)

	return ratios
}
