package armplanning

import (
	"context"
	"sync/atomic"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	// goalRetreatStepMM and goalRetreatCount shape the retreat corridors: the
	// cumulative upward offsets of the validated chain grown from each goal
	// root. See seedGoalRetreatCorridors.
	goalRetreatStepMM = 75.0
	goalRetreatCount  = 3
)

// midOrientationSlackDegs returns how much an intermediate keypoint's
// orientation may deviate from its nominal target. With orientation
// constraints, the constraint band (the keypoint must satisfy it anyway, and
// the state check enforces that); with path-shape constraints, none; with no
// constraints at all, unlimited - a stepping stone's orientation is irrelevant.
func midOrientationSlackDegs(c *motionplan.Constraints) float64 {
	if c == nil {
		return 360
	}
	if len(c.LinearConstraint) > 0 || len(c.PseudolinearConstraint) > 0 {
		return 0
	}
	if len(c.OrientationConstraint) == 0 {
		return 360
	}
	slack := c.OrientationConstraint[0].OrientationToleranceDegs
	for _, oc := range c.OrientationConstraint[1:] {
		slack = min(slack, oc.OrientationToleranceDegs)
	}
	return max(0, slack)
}

// retreatChain jogs the end effector of `frame` straight up from `from` in
// validated steps, returning the chain [from, step1, step2, ...] as far as it
// gets (possibly just [from]). Each hop is produced by a short gradient
// descent seeded at the previous configuration, then state- and path-checked.
func retreatChain(
	ctx context.Context,
	psc *PlanSegmentContext,
	solver *ik.NloptIK,
	frame string,
	from *referenceframe.LinearInputs,
) []*referenceframe.LinearInputs {
	chain := []*referenceframe.LinearInputs{from}
	cur := from
	for step := 0; step < goalRetreatCount; step++ {
		if ctx.Err() != nil {
			return chain
		}
		curDQ, err := psc.pc.fs.TransformToDQ(cur, frame, referenceframe.World)
		if err != nil {
			return chain
		}
		curPt := curDQ.Point()
		target := spatialmath.NewPose(
			r3.Vector{X: curPt.X, Y: curPt.Y, Z: curPt.Z + goalRetreatStepMM},
			curDQ.Orientation())

		metric := psc.pc.planOpts.GetGoalMetricWithOrientationSlack(referenceframe.FrameSystemPoses{
			frame: referenceframe.NewPoseInFrame(referenceframe.World, target),
		}, midOrientationSlackDegs(psc.pc.request.Constraints))
		linearSeed := cur.GetLinearizedInputs()
		var totalAttempts atomic.Int32
		solutions, _, err := ik.DoSolve(ctx, solver, &totalAttempts,
			psc.pc.LinearizeFSMetric(metric),
			[][]float64{linearSeed},
			[][]referenceframe.Limit{ik.ComputeAdjustLimits(linearSeed, psc.pc.lis.GetLimits(), .05)})
		if err != nil || len(solutions) == 0 {
			return chain
		}
		retreatCfg, err := psc.pc.lis.FloatsToInputs(solutions[0])
		if err != nil {
			return chain
		}
		if _, err := psc.Checker.CheckStateFSConstraints(ctx, &motionplan.StateFS{
			FS: psc.pc.fs, Configuration: retreatCfg,
		}); err != nil {
			return chain
		}
		if err := psc.CheckPath(ctx, cur, retreatCfg, true, nil); err != nil {
			return chain
		}
		chain = append(chain, retreatCfg)
		cur = retreatCfg
	}
	return chain
}
