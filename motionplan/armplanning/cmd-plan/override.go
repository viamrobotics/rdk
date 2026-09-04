package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
)

// frameSystemOverride is a set of edits applied to the frame system of every request in a run,
// before planning. Recorded requests carry the frame system the robot had at capture time, so an
// override is how you ask "what would these same plans do on hardware with different limits".
//
// The schema mirrors the builtin motion service's `input_range_override` config, so a limit worked
// out here can be pasted straight into a robot config:
//
//	{
//	  "input_range_override": {
//	    "arm1": {
//	      "0":                  {"min": -1.5707, "max": 1.5707},
//	      "shoulder_lift_joint": {"min": -3.1415, "max": 3.1415, "max_velocity": 1.0}
//	    }
//	  }
//	}
type frameSystemOverride struct {
	// InputRangeOverride is keyed by frame name, then by either a joint frame's name or its index
	// among the model's moveable joints.
	//
	// Positions are in the planner's own units, radians for revolute joints and millimeters for
	// prismatic ones, not the degrees a kinematics file uses. They replace what the model
	// declared. Velocity and acceleration bounds only ever tighten it, so an override can slow a
	// joint down but never speed it past what its model allows.
	InputRangeOverride map[string]map[string]referenceframe.Limit `json:"input_range_override"`

	// source is the file this was read from, for error messages.
	source string
}

// readFrameSystemOverride reads an override file. Unknown keys are rejected rather than ignored,
// because an override that silently does nothing looks exactly like a planner that ignored it.
func readFrameSystemOverride(fileName string) (*frameSystemOverride, error) {
	f, err := os.Open(filepath.Clean(fileName))
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(f.Close)

	override := &frameSystemOverride{source: fileName}
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(override); err != nil {
		return nil, fmt.Errorf("could not read frame system override %s: %w", fileName, err)
	}

	if len(override.InputRangeOverride) == 0 {
		return nil, fmt.Errorf("frame system override %s has no overrides in it", fileName)
	}

	return override, nil
}

// apply edits fs in place. The override is expected to hold for every request in a run, so a key
// that matches nothing is an error rather than a warning.
func (o *frameSystemOverride) apply(fs *referenceframe.FrameSystem, logger logging.Logger) error {
	for frameName, mods := range o.InputRangeOverride {
		frame := fs.Frame(frameName)
		if frame == nil {
			return fmt.Errorf("input_range_override names frame %q, which the frame system does not have. It has %v",
				frameName, fs.FrameNames())
		}

		model, ok := frame.(*referenceframe.SimpleModel)
		if !ok {
			return fmt.Errorf("can only override the joints of a SimpleModel, and frame %q is a %T", frameName, frame)
		}

		// Keys may name a joint frame or give its index among the moveable frames, matching what
		// `input_range_override` accepts in a robot config.
		moveableNames := model.MoveableFrameNames()
		resolved := make(map[string]referenceframe.Limit, len(mods))
		for key, limit := range mods {
			idx := slices.Index(moveableNames, key)
			if idx < 0 {
				keyAsIndex, err := strconv.Atoi(key)
				if err != nil || keyAsIndex < 0 || keyAsIndex >= len(moveableNames) {
					return fmt.Errorf("frame %q has no joint %q. Its moveable joints are %v", frameName, key, moveableNames)
				}
				idx = keyAsIndex
			}
			resolved[moveableNames[idx]] = limit
		}

		newModel, err := referenceframe.NewModelWithLimitOverrides(model, resolved)
		if err != nil {
			return err
		}
		if err := fs.ReplaceFrame(newModel); err != nil {
			return err
		}

		for _, name := range slices.Sorted(maps.Keys(resolved)) {
			logger.Infof("overrode limits of %s:%s to %s", frameName, name, describeLimit(resolved[name]))
		}
	}

	return nil
}

// describeLimit renders a Limit for a log line. Its velocity and acceleration bounds are pointers,
// so default formatting prints addresses.
func describeLimit(limit referenceframe.Limit) string {
	out := fmt.Sprintf("min %v max %v", limit.Min, limit.Max)
	if limit.MaxVelocity != nil {
		out += fmt.Sprintf(" max_velocity %v", *limit.MaxVelocity)
	}
	if limit.MaxAcceleration != nil {
		out += fmt.Sprintf(" max_acceleration %v", *limit.MaxAcceleration)
	}
	return out
}
