package armplanning

import (
	"context"
	"math"
	"math/rand"
	"sort"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

const (
	// nudgeMaxSegmentChecks bounds the total number of segment checks one
	// tryNudgedStraightLine call may spend per goal before giving up and letting
	// cBiRRT take over. Most checks here are short hops around the blocked
	// stretch, so this budget is cheap relative to even a few RRT iterations.
	nudgeMaxSegmentChecks = 120
	// nudgeCandidatesPerRadius is how many random deltas to screen (with cheap
	// state checks) at each radius before paying for segment checks.
	nudgeCandidatesPerRadius = 12
	// nudgeSegmentTriesPerRadius is how many of the screened candidates, best
	// clearance first, get the more expensive segment-check treatment.
	nudgeSegmentTriesPerRadius = 2
	// nudgeMaxDepth lets a failed hop be repaired with its own nudge, so a path
	// can pick up more than one detour when a single one isn't enough. Observed
	// behavior is geometric convergence — each level shrinks the blocked
	// stretch — so a few levels go a long way; the segment-check budget guards
	// against runaway recursion.
	nudgeMaxDepth = 4
	// nudgeMaxBlockedFraction skips goals whose blocked stretch is most of the
	// path: those need real planning, not a nudge, and their budget is better
	// spent on goals that are barely blocked.
	nudgeMaxBlockedFraction = 0.5
)

// nudgeRadiusSchedule is the per-joint perturbation magnitude in radians (or
// input units), smallest first: a path that is barely colliding should be
// repairable with a barely different configuration.
var nudgeRadiusSchedule = []float64{0.1, 0.2, 0.4, 0.7}

// nudgeRadiusScheduleFor picks the perturbation schedule for this segment.
// Under a linear constraint the entire repair must stay inside a tube of the
// constraint's tolerance; on an arm-scale lever the default 0.1-0.7 rad
// radii move the end effector tens to hundreds of millimeters, so every
// candidate died on the topological constraint before collision was even
// consulted - nudge could never repair a constrained segment. Scale the
// radii so candidate deviation is on the order of the tube radius.
func nudgeRadiusScheduleFor(psc *PlanSegmentContext) []float64 {
	lc := tightestLinearConstraintMM(psc.pc.request.Constraints)
	if lc <= 0 {
		return nudgeRadiusSchedule
	}
	// Conservative effective lever: EE deviation (mm) per radian of joint
	// motion on an arm-scale chain.
	const leverMM = 700.0
	base := lc / leverMM
	return []float64{base / 4, base / 2, base, base * 2}
}

// jointDelta is a per-moving-frame joint-space offset.
type jointDelta map[string][]float64

// tryNudgedStraightLine handles the common case where the straight-line path to
// an IK solution is barely blocked: instead of launching a full bidirectional
// RRT search, bracket the blocked stretch of the interpolation and try to step
// around it with small lateral perturbations. The prefix and suffix of the line
// outside the bracket are already verified by the bracketing sweeps, so via
// attempts only pay for short hops. Returns nil when no goal could be repaired
// within budget; the caller then falls back to cBiRRT.
func tryNudgedStraightLine(
	ctx context.Context,
	psc *PlanSegmentContext,
	goalMap rrtMap,
	rnd *rand.Rand,
	logger logging.Logger,
) []*referenceframe.LinearInputs {
	goals := make([]*node, 0, len(goalMap))
	for n := range goalMap {
		goals = append(goals, n)
	}
	// Best (lowest-cost) IK solutions first.
	sort.Slice(goals, func(i, j int) bool { return goals[i].cost < goals[j].cost })

	moving := movingFrameNamesWithDoF(psc)
	if len(moving) == 0 {
		return nil
	}

	// Nudge exists for barely-blocked straight lines; if the several
	// lowest-cost goal configurations all fail, more distant ones will not do
	// better, and each attempt burns a full segment-check budget.
	const maxNudgeGoals = 6
	for gi, g := range goals {
		if gi >= maxNudgeGoals || ctx.Err() != nil {
			return nil
		}
		budget := nudgeMaxSegmentChecks
		path := nudgeRepair(ctx, psc, psc.start, g.inputs, moving, nudgeMaxDepth, &budget, rnd, logger)
		if path != nil {
			logger.Debugf("nudged straight line to goal %d succeeded: %d waypoints, %d/%d segment checks used",
				gi, len(path), nudgeMaxSegmentChecks-budget, nudgeMaxSegmentChecks)
			return path
		}
		logger.Debugf("nudge to goal %d failed, %d/%d segment checks used", gi, nudgeMaxSegmentChecks-budget, nudgeMaxSegmentChecks)
	}
	return nil
}

// nudgeRepair tries to connect a to b with a straight segment; on failure it
// brackets the blocked stretch [fa, fb] of the interpolation from both ends
// (the outer portions a→fa and fb→b are validated by those very sweeps) and
// tries two repair shapes around the stretch with growing lateral
// perturbations:
//
//   - via:   fa → (mid+δ) → fb — step over a small blockage
//   - slide: fa → (fa+δ) → (fb+δ) → fb — move the whole stretch sideways,
//     for blockages too long for a single via
//
// Deltas are projected perpendicular to the path direction (a nudge along the
// path can't avoid anything) and screened with cheap state checks, best
// clearance first, before any segment check is spent. A failing hop may
// recurse (depth-1) so multi-detour paths remain reachable.
// Returns the waypoint list including both endpoints, or nil.
func nudgeRepair(
	ctx context.Context,
	psc *PlanSegmentContext,
	a, b *referenceframe.LinearInputs,
	moving []string,
	depth int,
	budget *int,
	rnd *rand.Rand,
	logger logging.Logger,
) []*referenceframe.LinearInputs {
	if *budget <= 0 || ctx.Err() != nil {
		return nil
	}
	*budget--
	var fbFwd PathFeedback
	if err := psc.CheckPath(ctx, a, b, true, &fbFwd); err == nil {
		return []*referenceframe.LinearInputs{a, b}
	}
	if depth <= 0 || *budget <= 0 {
		return nil
	}

	fa := fbFwd.LastGoodInputs
	if fa == nil {
		fa = a
	}
	*budget--
	fb := b
	var fbRev PathFeedback
	if err := psc.CheckPath(ctx, b, a, true, &fbRev); err != nil && fbRev.LastGoodInputs != nil {
		fb = fbRev.LastGoodInputs
	}

	totalL2 := linearInputsL2(a, b)
	blockedL2 := linearInputsL2(fa, fb)
	logger.Debugf("nudge depth %d: total l2 %.3f, blocked stretch l2 %.3f", depth, totalL2, blockedL2)
	// A mostly-blocked FULL path needs planning, not a nudge — but only gate the
	// top level: a recursive short hop spans the obstacle by construction, so a
	// high blocked fraction there is normal and exactly what nudging is for.
	if depth == nudgeMaxDepth && totalL2 > 0 && blockedL2/totalL2 > nudgeMaxBlockedFraction {
		return nil
	}

	mid, err := referenceframe.InterpolateFS(psc.pc.fs, fa, fb, 0.5)
	if err != nil {
		return nil
	}
	dir := pathDirection(a, b, moving)

	// stateClearance screens a candidate with a cheap state check; returns the
	// clearance and whether the state is valid at all.
	stateClearance := func(v *referenceframe.LinearInputs) (float64, bool) {
		closest, err := psc.Checker.CheckStateFSConstraints(ctx, &motionplan.StateFS{FS: psc.pc.fs, Configuration: v})
		return closest, err == nil
	}

	// connectHop checks one short hop, recursing to repair it when allowed.
	connectHop := func(x, y *referenceframe.LinearInputs) []*referenceframe.LinearInputs {
		return nudgeRepair(ctx, psc, x, y, moving, depth-1, budget, rnd, logger)
	}

	tryVias := func(vias ...*referenceframe.LinearInputs) []*referenceframe.LinearInputs {
		path := []*referenceframe.LinearInputs{fa}
		prev := fa
		for _, v := range append(vias, fb) {
			hop := connectHop(prev, v)
			if hop == nil {
				return nil
			}
			path = append(path, hop[1:]...)
			prev = v
		}
		return path
	}

	type candidate struct {
		vias      []*referenceframe.LinearInputs
		clearance float64
	}

	for _, radius := range nudgeRadiusScheduleFor(psc) {
		// Screen candidates with cheap state checks first.
		candidates := []candidate{}
		for i := 0; i < nudgeCandidatesPerRadius; i++ {
			delta := randomLateralDelta(psc.pc.fs, moving, radius, dir, rnd)
			if i%2 == 0 {
				via := applyJointDelta(psc.pc.fs, mid, delta)
				if clearance, ok := stateClearance(via); ok {
					candidates = append(candidates, candidate{[]*referenceframe.LinearInputs{via}, clearance})
				}
			} else {
				v1 := applyJointDelta(psc.pc.fs, fa, delta)
				v2 := applyJointDelta(psc.pc.fs, fb, delta)
				c1, ok1 := stateClearance(v1)
				if !ok1 {
					continue
				}
				c2, ok2 := stateClearance(v2)
				if !ok2 {
					continue
				}
				candidates = append(candidates, candidate{[]*referenceframe.LinearInputs{v1, v2}, math.Min(c1, c2)})
			}
		}
		// Pay for segment checks on the most promising candidates only.
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].clearance > candidates[j].clearance })
		for i := 0; i < len(candidates) && i < nudgeSegmentTriesPerRadius; i++ {
			if *budget <= 0 || ctx.Err() != nil {
				return nil
			}
			repaired := tryVias(candidates[i].vias...)
			if repaired == nil {
				continue
			}
			// Reattach the already-verified outer portions of the line.
			path := []*referenceframe.LinearInputs{}
			if fa != a {
				path = append(path, a)
			}
			path = append(path, repaired...)
			if fb != b {
				path = append(path, b)
			}
			return path
		}
	}
	return nil
}

// pathDirection returns the normalized joint-space direction of the path over
// the moving frames, used to make nudges lateral.
func pathDirection(a, b *referenceframe.LinearInputs, moving []string) jointDelta {
	dir := jointDelta{}
	norm := 0.0
	for _, name := range moving {
		av := a.Get(name)
		bv := b.Get(name)
		if len(av) != len(bv) {
			continue
		}
		d := make([]float64, len(av))
		for i := range av {
			d[i] = bv[i] - av[i]
			norm += d[i] * d[i]
		}
		dir[name] = d
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return nil
	}
	for _, d := range dir {
		for i := range d {
			d[i] /= norm
		}
	}
	return dir
}

// randomLateralDelta samples a per-joint offset with magnitude up to radius per
// joint and removes its component along the path direction — a nudge along the
// path can't move anything off an obstacle.
func randomLateralDelta(fs *referenceframe.FrameSystem, moving []string, radius float64, dir jointDelta, rnd *rand.Rand) jointDelta {
	delta := make(jointDelta, len(moving))
	maxAbs := 0.0
	dot := 0.0
	for _, name := range moving {
		f := fs.Frame(name)
		if f == nil {
			continue
		}
		d := make([]float64, len(f.DoF()))
		dirD := dir[name]
		for i := range d {
			d[i] = rnd.Float64()*2 - 1
			if i < len(dirD) {
				dot += d[i] * dirD[i]
			}
		}
		delta[name] = d
	}
	for name, d := range delta {
		dirD := dir[name]
		for i := range d {
			if i < len(dirD) {
				d[i] -= dot * dirD[i]
			}
			maxAbs = math.Max(maxAbs, math.Abs(d[i]))
		}
	}
	if maxAbs == 0 {
		return delta
	}
	scale := radius / maxAbs
	for _, d := range delta {
		for i := range d {
			d[i] *= scale
		}
	}
	return delta
}

// applyJointDelta returns a copy of base with delta added to the moving frames'
// inputs, clamped to joint limits; everything else — e.g. a parked second arm —
// is left untouched.
func applyJointDelta(
	fs *referenceframe.FrameSystem,
	base *referenceframe.LinearInputs,
	delta jointDelta,
) *referenceframe.LinearInputs {
	out := referenceframe.NewLinearInputs()
	for name, inputs := range base.Items() {
		d, ok := delta[name]
		if !ok || len(d) != len(inputs) {
			out.Put(name, inputs)
			continue
		}
		limits := fs.Frame(name).DoF()
		q := make([]referenceframe.Input, len(inputs))
		for i, v := range inputs {
			nv := v + d[i]
			nv = max(nv, limits[i].Min)
			nv = min(nv, limits[i].Max)
			q[i] = nv
		}
		out.Put(name, q)
	}
	return out
}

// linearInputsL2 is a debug helper: joint-space L2 distance between two configurations.
func linearInputsL2(a, b *referenceframe.LinearInputs) float64 {
	return referenceframe.InputsL2Distance(a.GetLinearizedInputs(), b.GetLinearizedInputs())
}

// movingFrameNamesWithDoF returns the names of frames that both carry DoF and
// sit on a motion chain for this segment — the joints the planner is actually
// steering toward the goal.
func movingFrameNamesWithDoF(psc *PlanSegmentContext) []string {
	moving, _ := psc.motionChains.framesFilteredByMovingAndNonmoving()
	out := []string{}
	for _, name := range moving {
		if f := psc.pc.fs.Frame(name); f != nil && len(f.DoF()) > 0 {
			out = append(out, name)
		}
	}
	return out
}
