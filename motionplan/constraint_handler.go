//go:build !no_cgo

package motionplan

import (
	"fmt"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

var defaultMinStepCount = 2

// ConstraintHandler is a convenient wrapper for constraint handling which is likely to be common among most motion
// planners. Including a constraint handler as an anonymous struct member allows reuse.
type ConstraintHandler struct {
	segmentConstraints   map[string]SegmentConstraint
	segmentFSConstraints map[string]SegmentFSConstraint
	stateConstraints     map[string]StateConstraint
	stateFSConstraints   map[string]StateFSConstraint
}

// CheckStateConstraints will check a given input against all state constraints.
// Return values are:
// -- a bool representing whether all constraints passed
// -- if failing, a string naming the failed constraint.
func (c *ConstraintHandler) CheckStateConstraints(state *ik.State) (bool, string) {
	for name, cFunc := range c.stateConstraints {
		pass := cFunc(state)
		if !pass {
			return false, name
		}
	}
	return true, ""
}

// CheckStateFSConstraints will check a given input against all FS state constraints.
// Return values are:
// -- a bool representing whether all constraints passed
// -- if failing, a string naming the failed constraint.
func (c *ConstraintHandler) CheckStateFSConstraints(state *ik.StateFS) (bool, string) {
	for name, cFunc := range c.stateFSConstraints {
		pass := cFunc(state)
		if !pass {
			return false, name
		}
	}
	return true, ""
}

// CheckSegmentConstraints will check a given input against all segment constraints.
// Return values are:
// -- a bool representing whether all constraints passed
// -- if failing, a string naming the failed constraint.
func (c *ConstraintHandler) CheckSegmentConstraints(segment *ik.Segment) (bool, string) {
	for name, cFunc := range c.segmentConstraints {
		pass := cFunc(segment)
		if !pass {
			return false, name
		}
	}
	return true, ""
}

// CheckSegmentFSConstraints will check a given input against all FS segment constraints.
// Return values are:
// -- a bool representing whether all constraints passed
// -- if failing, a string naming the failed constraint.
func (c *ConstraintHandler) CheckSegmentFSConstraints(segment *ik.SegmentFS) (bool, string) {
	for name, cFunc := range c.segmentFSConstraints {
		pass := cFunc(segment)
		if !pass {
			return false, name
		}
	}
	return true, ""
}

// CheckStateConstraintsAcrossSegment will interpolate the given input from the StartInput to the EndInput, and ensure that all intermediate
// states as well as both endpoints satisfy all state constraints. If all constraints are satisfied, then this will return `true, nil`.
// If any constraints fail, this will return false, and an Segment representing the valid portion of the segment, if any. If no
// part of the segment is valid, then `false, nil` is returned.
func (c *ConstraintHandler) CheckStateConstraintsAcrossSegment(ci *ik.Segment, resolution float64) (bool, *ik.Segment) {
	interpolatedConfigurations, err := interpolateSegment(ci, resolution)
	if err != nil {
		return false, nil
	}
	var lastGood []referenceframe.Input
	for i, interpConfig := range interpolatedConfigurations {
		interpC := &ik.State{Frame: ci.Frame, Configuration: interpConfig}
		if resolveStatesToPositions(interpC) != nil {
			return false, nil
		}
		pass, _ := c.CheckStateConstraints(interpC)
		if !pass {
			if i == 0 {
				// fail on start pos
				return false, nil
			}
			return false, &ik.Segment{StartConfiguration: ci.StartConfiguration, EndConfiguration: lastGood, Frame: ci.Frame}
		}
		lastGood = interpC.Configuration
	}

	return true, nil
}

// interpolateSegment is a helper function which produces a list of intermediate inputs, between the start and end
// configuration of a segment at a given resolution value.
func interpolateSegment(ci *ik.Segment, resolution float64) ([][]referenceframe.Input, error) {
	// ensure we have cartesian positions
	if err := resolveSegmentsToPositions(ci); err != nil {
		return nil, err
	}

	steps := CalculateStepCount(ci.StartPosition, ci.EndPosition, resolution)
	if steps < defaultMinStepCount {
		// Minimum step count ensures we are not missing anything
		steps = defaultMinStepCount
	}

	var interpolatedConfigurations [][]referenceframe.Input
	for i := 0; i <= steps; i++ {
		interp := float64(i) / float64(steps)
		interpConfig, err := ci.Frame.Interpolate(ci.StartConfiguration, ci.EndConfiguration, interp)
		if err != nil {
			return nil, err
		}
		interpolatedConfigurations = append(interpolatedConfigurations, interpConfig)
	}
	return interpolatedConfigurations, nil
}

// interpolateSegmentFS is a helper function which produces a list of intermediate inputs, between the start and end
// configuration of a segment at a given resolution value.
func interpolateSegmentFS(ci *ik.SegmentFS, resolution float64) ([]referenceframe.FrameSystemInputs, error) {
	// Find the frame with the most steps by calculating steps for each frame
	maxSteps := defaultMinStepCount
	for frameName, startConfig := range ci.StartConfiguration {
		if len(startConfig) == 0 {
			// No need to interpolate 0dof frames
			continue
		}
		endConfig, exists := ci.EndConfiguration[frameName]
		if !exists {
			return nil, fmt.Errorf("frame %s exists in start config but not in end config", frameName)
		}

		// Get frame from FrameSystem
		frame := ci.FS.Frame(frameName)
		if frame == nil {
			return nil, fmt.Errorf("frame %s exists in start config but not in framesystem", frameName)
		}

		// Calculate positions for this frame's start and end configs
		startPos, err := frame.Transform(startConfig)
		if err != nil {
			return nil, err
		}
		endPos, err := frame.Transform(endConfig)
		if err != nil {
			return nil, err
		}

		// Calculate steps needed for this frame
		steps := CalculateStepCount(startPos, endPos, resolution)
		if steps > maxSteps {
			maxSteps = steps
		}
	}

	// Create interpolated configurations for all frames
	var interpolatedConfigurations []referenceframe.FrameSystemInputs
	for i := 0; i <= maxSteps; i++ {
		interp := float64(i) / float64(maxSteps)
		frameConfigs := make(referenceframe.FrameSystemInputs)

		// Interpolate each frame's configuration
		for frameName, startConfig := range ci.StartConfiguration {
			endConfig := ci.EndConfiguration[frameName]
			frame := ci.FS.Frame(frameName)

			interpConfig, err := frame.Interpolate(startConfig, endConfig, interp)
			if err != nil {
				return nil, err
			}
			frameConfigs[frameName] = interpConfig
		}

		interpolatedConfigurations = append(interpolatedConfigurations, frameConfigs)
	}

	return interpolatedConfigurations, nil
}

// CheckSegmentAndStateValidity will check an segment input and confirm that it 1) meets all segment constraints, and 2) meets all
// state constraints across the segment at some resolution. If it fails an intermediate state, it will return the shortest valid segment,
// provided that segment also meets segment constraints.
func (c *ConstraintHandler) CheckSegmentAndStateValidity(segment *ik.Segment, resolution float64) (bool, *ik.Segment) {
	valid, subSegment := c.CheckStateConstraintsAcrossSegment(segment, resolution)
	if !valid {
		if subSegment != nil {
			subSegmentValid, _ := c.CheckSegmentConstraints(subSegment)
			if subSegmentValid {
				return false, subSegment
			}
		}
		return false, nil
	}
	// all states are valid
	valid, _ = c.CheckSegmentConstraints(segment)
	return valid, nil
}

// AddStateConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *ConstraintHandler) AddStateConstraint(name string, cons StateConstraint) {
	if c.stateConstraints == nil {
		c.stateConstraints = map[string]StateConstraint{}
	}
	name = name + "_" + fmt.Sprintf("%p", cons)
	c.stateConstraints[name] = cons
}

// RemoveStateConstraint will remove the given constraint.
func (c *ConstraintHandler) RemoveStateConstraint(name string) {
	delete(c.stateConstraints, name)
}

// StateConstraints will list all state constraints by name.
func (c *ConstraintHandler) StateConstraints() []string {
	names := make([]string, 0, len(c.stateConstraints))
	for name := range c.stateConstraints {
		names = append(names, name)
	}
	return names
}

// AddSegmentConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *ConstraintHandler) AddSegmentConstraint(name string, cons SegmentConstraint) {
	if c.segmentConstraints == nil {
		c.segmentConstraints = map[string]SegmentConstraint{}
	}
	// Add function address to name to prevent collisions
	name = name + "_" + fmt.Sprintf("%p", cons)
	c.segmentConstraints[name] = cons
}

// RemoveSegmentConstraint will remove the given constraint.
func (c *ConstraintHandler) RemoveSegmentConstraint(name string) {
	delete(c.segmentConstraints, name)
}

// SegmentConstraints will list all segment constraints by name.
func (c *ConstraintHandler) SegmentConstraints() []string {
	names := make([]string, 0, len(c.segmentConstraints))
	for name := range c.segmentConstraints {
		names = append(names, name)
	}
	return names
}

// AddStateFSConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *ConstraintHandler) AddStateFSConstraint(name string, cons StateFSConstraint) {
	if c.stateFSConstraints == nil {
		c.stateFSConstraints = map[string]StateFSConstraint{}
	}
	name = name + "_" + fmt.Sprintf("%p", cons)
	c.stateFSConstraints[name] = cons
}

// RemoveStateFSConstraint will remove the given constraint.
func (c *ConstraintHandler) RemoveStateFSConstraint(name string) {
	delete(c.stateFSConstraints, name)
}

// StateFSConstraints will list all FS state constraints by name.
func (c *ConstraintHandler) StateFSConstraints() []string {
	names := make([]string, 0, len(c.stateFSConstraints))
	for name := range c.stateFSConstraints {
		names = append(names, name)
	}
	return names
}

// AddSegmentFSConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *ConstraintHandler) AddSegmentFSConstraint(name string, cons SegmentFSConstraint) {
	if c.segmentFSConstraints == nil {
		c.segmentFSConstraints = map[string]SegmentFSConstraint{}
	}
	name = name + "_" + fmt.Sprintf("%p", cons)
	c.segmentFSConstraints[name] = cons
}

// RemoveSegmentFSConstraint will remove the given constraint.
func (c *ConstraintHandler) RemoveSegmentFSConstraint(name string) {
	delete(c.segmentFSConstraints, name)
}

// SegmentFSConstraints will list all FS segment constraints by name.
func (c *ConstraintHandler) SegmentFSConstraints() []string {
	names := make([]string, 0, len(c.segmentFSConstraints))
	for name := range c.segmentFSConstraints {
		names = append(names, name)
	}
	return names
}

// CheckStateConstraintsAcrossSegmentFS will interpolate the given input from the StartConfiguration to the EndConfiguration, and ensure
// that all intermediate states as well as both endpoints satisfy all state constraints. If all constraints are satisfied, then this will
// return `true, nil`. If any constraints fail, this will return false, and an SegmentFS representing the valid portion of the segment,
// if any. If no part of the segment is valid, then `false, nil` is returned.
func (c *ConstraintHandler) CheckStateConstraintsAcrossSegmentFS(ci *ik.SegmentFS, resolution float64) (bool, *ik.SegmentFS) {
	interpolatedConfigurations, err := interpolateSegmentFS(ci, resolution)
	if err != nil {
		return false, nil
	}
	var lastGood referenceframe.FrameSystemInputs
	for i, interpConfig := range interpolatedConfigurations {
		interpC := &ik.StateFS{FS: ci.FS, Configuration: interpConfig}
		pass, _ := c.CheckStateFSConstraints(interpC)
		if !pass {
			if i == 0 {
				// fail on start pos
				return false, nil
			}
			return false, &ik.SegmentFS{StartConfiguration: ci.StartConfiguration, EndConfiguration: lastGood, FS: ci.FS}
		}
		lastGood = interpC.Configuration
	}

	return true, nil
}

// CheckSegmentAndStateValidityFS will check a segment input and confirm that it 1) meets all segment constraints, and 2) meets all
// state constraints across the segment at some resolution. If it fails an intermediate state, it will return the shortest valid segment,
// provided that segment also meets segment constraints.
func (c *ConstraintHandler) CheckSegmentAndStateValidityFS(segment *ik.SegmentFS, resolution float64) (bool, *ik.SegmentFS) {
	valid, subSegment := c.CheckStateConstraintsAcrossSegmentFS(segment, resolution)
	if !valid {
		if subSegment != nil {
			subSegmentValid, _ := c.CheckSegmentFSConstraints(subSegment)
			if subSegmentValid {
				return false, subSegment
			}
		}
		return false, nil
	}
	// all states are valid
	valid, _ = c.CheckSegmentFSConstraints(segment)
	return valid, nil
}
