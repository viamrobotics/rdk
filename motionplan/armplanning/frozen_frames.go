package armplanning

import (
	"fmt"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// freezeFrames returns a clone of fs in which each named frame is a 0-DoF static frame at its pose
// for its inputs in start. Its geometries, placed at those inputs, move to a static sibling frame
// named <name>_frozen_geometry so they stay collision obstacles under their original labels. The
// frozen inputs are returned keyed by frame name.
func freezeFrames(
	fs *referenceframe.FrameSystem, names []string, start referenceframe.FrameSystemInputs,
) (*referenceframe.FrameSystem, referenceframe.FrameSystemInputs, error) {
	if len(names) == 0 {
		return fs, nil, nil
	}
	frozenFS, err := fs.Clone()
	if err != nil {
		return nil, nil, err
	}
	frozen := referenceframe.FrameSystemInputs{}
	for _, name := range names {
		f := frozenFS.Frame(name)
		if f == nil {
			return nil, nil, fmt.Errorf("cannot freeze frame %q: %w", name, referenceframe.NewFrameMissingError(name))
		}
		if len(f.DoF()) == 0 {
			return nil, nil, fmt.Errorf("cannot freeze frame %q: it has no degrees of freedom", name)
		}
		inputs, ok := start[name]
		if !ok {
			return nil, nil, fmt.Errorf("cannot freeze frame %q: start state has no configuration for it", name)
		}
		pose, err := f.Transform(inputs)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot freeze frame %q: %w", name, err)
		}
		// Geometries come back expressed in the parent frame, so they hang off the parent at a zero
		// pose rather than off the frozen frame, which would apply pose twice.
		gif, err := f.Geometries(inputs)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot freeze frame %q: %w", name, err)
		}
		parent, err := frozenFS.Parent(f)
		if err != nil {
			return nil, nil, err
		}
		static, err := referenceframe.NewStaticFrame(name, pose)
		if err != nil {
			return nil, nil, err
		}
		if err := frozenFS.ReplaceFrame(static); err != nil {
			return nil, nil, err
		}
		if geoms := gif.Geometries(); len(geoms) > 0 {
			geomFrame, err := referenceframe.NewStaticFrameWithGeometries(name+"_frozen_geometry", spatialmath.NewZeroPose(), geoms)
			if err != nil {
				return nil, nil, err
			}
			if err := frozenFS.AddFrame(geomFrame, frozenFS.Frame(parent.Name())); err != nil {
				return nil, nil, err
			}
		}
		frozen[name] = inputs
	}
	return frozenFS, frozen, nil
}
