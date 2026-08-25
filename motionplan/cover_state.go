package motionplan

import (
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// The cover table lets the collision constraints evaluate most states without
// materializing any world-space geometry objects. For every moving frame with
// configuration-independent local geometry (zero degrees of freedom - the
// normal case for the link frames of a flattened arm model), it precomputes
// each geometry's conservative sphere cover in the frame's parent
// coordinates. A state check then needs only memoized FK for the parent
// frames plus point transforms of the cover centers; full geometry objects
// are built only for the few frames the sphere phase cannot clear.

// geomCover is one geometry's precomputed local-frame cover.
type geomCover struct {
	label string
	// cover spheres in the owning frame's parent coordinates.
	local []spatialmath.SphereBound
	// bound encloses the whole cover: one sphere for pair-level broadphase.
	bound spatialmath.SphereBound
}

// frameCoverEntry is the cover data for one moving frame.
type frameCoverEntry struct {
	frameName  string
	parentName string
	geoms      []geomCover
}

// frameCoverTable is the per-checker precomputation. materialize lists moving
// frames whose geometry could not be covered (input-dependent geometry or
// unsupported types); they are always evaluated exactly.
type frameCoverTable struct {
	entries     []frameCoverEntry
	materialize map[string]bool
}

// buildFrameCoverTable precomputes covers for the moving frames. Returns nil
// when nothing could be covered (callers then keep the materialize-everything
// path).
func buildFrameCoverTable(fs *referenceframe.FrameSystem, movingFrameNames map[string]bool) *frameCoverTable {
	t := &frameCoverTable{materialize: map[string]bool{}}
	for name := range movingFrameNames {
		frame := fs.Frame(name)
		if frame == nil {
			continue
		}
		parent, err := fs.Parent(frame)
		if err != nil || len(frame.DoF()) != 0 {
			// Input-dependent geometry (or no parent): always exact.
			t.materialize[name] = true
			continue
		}
		gif, err := frame.Geometries([]referenceframe.Input{})
		if err != nil || gif == nil || len(gif.Geometries()) == 0 {
			if err != nil {
				t.materialize[name] = true
			}
			continue
		}
		entry := frameCoverEntry{frameName: name, parentName: parent.Name()}
		covered := true
		for _, g := range gif.Geometries() {
			cover := spatialmath.SphereCover(g)
			if len(cover) == 0 {
				covered = false
				break
			}
			entry.geoms = append(entry.geoms, geomCover{
				label: g.Label(),
				local: cover,
				bound: enclosingSphere(cover),
			})
		}
		if !covered {
			t.materialize[name] = true
			continue
		}
		t.entries = append(t.entries, entry)
	}
	if len(t.entries) == 0 {
		return nil
	}
	return t
}

// enclosingSphere returns one sphere containing every sphere of the cover.
func enclosingSphere(cover []spatialmath.SphereBound) spatialmath.SphereBound {
	minPt := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	maxPt := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	for _, sb := range cover {
		minPt.X = math.Min(minPt.X, sb.Center.X-sb.R)
		minPt.Y = math.Min(minPt.Y, sb.Center.Y-sb.R)
		minPt.Z = math.Min(minPt.Z, sb.Center.Z-sb.R)
		maxPt.X = math.Max(maxPt.X, sb.Center.X+sb.R)
		maxPt.Y = math.Max(maxPt.Y, sb.Center.Y+sb.R)
		maxPt.Z = math.Max(maxPt.Z, sb.Center.Z+sb.R)
	}
	c := minPt.Add(maxPt).Mul(0.5)
	return spatialmath.SphereBound{Center: c, R: maxPt.Sub(c).Norm()}
}

// coveredWorldGeom is one geometry's cover transformed into world coordinates
// for a specific state.
type coveredWorldGeom struct {
	frameName string
	label     string
	world     []spatialmath.SphereBound
	bound     spatialmath.SphereBound
}

// stateCoverSet is the per-state evaluation of a frameCoverTable, shared by
// the three collision constraints of a checker via StateFS.
type stateCoverSet struct {
	geoms []coveredWorldGeom
	// failed is set when FK failed; callers must use the exact path.
	failed bool
}

// coverSetFor evaluates (and caches on the state) the world-space covers.
func coverSetFor(state *StateFS, fs *referenceframe.FrameSystem, table *frameCoverTable) *stateCoverSet {
	if state.coverSet != nil {
		return state.coverSet.(*stateCoverSet)
	}
	cs := &stateCoverSet{}
	fk := fs.NewFrameToWorld(state.Configuration)
	for i := range table.entries {
		e := &table.entries[i]
		q, tr, err := fk.PoseQT(e.parentName)
		if err != nil {
			cs.failed = true
			break
		}
		for gi := range e.geoms {
			gc := &e.geoms[gi]
			world := make([]spatialmath.SphereBound, len(gc.local))
			for si, sb := range gc.local {
				world[si] = spatialmath.SphereBound{Center: spatialmath.TransformPoint(q, tr, sb.Center), R: sb.R}
			}
			cs.geoms = append(cs.geoms, coveredWorldGeom{
				frameName: e.frameName,
				label:     gc.label,
				world:     world,
				bound: spatialmath.SphereBound{
					Center: spatialmath.TransformPoint(q, tr, gc.bound.Center),
					R:      gc.bound.R,
				},
			})
		}
	}
	state.coverSet = cs
	return cs
}

// materializedSubset returns the world-posed geometries of the requested
// frames, materializing them on demand and caching per state so the three
// constraints share the work.
func materializedSubset(
	state *StateFS, fs *referenceframe.FrameSystem, frames map[string]bool,
) ([]spatialmath.Geometry, error) {
	if len(frames) == 0 {
		return nil, nil
	}
	if state.partialGeoms == nil {
		state.partialGeoms = map[string]*referenceframe.GeometriesInFrame{}
	}
	missing := map[string]bool{}
	for f := range frames {
		if _, ok := state.partialGeoms[f]; !ok {
			missing[f] = true
		}
	}
	if len(missing) > 0 {
		got, err := referenceframe.FrameSystemGeometriesForFrames(fs, state.Configuration, missing)
		if err != nil {
			return nil, err
		}
		for f, gif := range got {
			state.partialGeoms[f] = gif
		}
		// Frames that produced no geometries still count as materialized.
		for f := range missing {
			if _, ok := state.partialGeoms[f]; !ok {
				state.partialGeoms[f] = referenceframe.NewGeometriesInFrame(referenceframe.World, nil)
			}
		}
	}
	var out []spatialmath.Geometry
	for f := range frames {
		if gif := state.partialGeoms[f]; gif != nil {
			out = append(out, gif.Geometries()...)
		}
	}
	return out, nil
}
