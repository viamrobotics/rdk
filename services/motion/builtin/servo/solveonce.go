package servo

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

// Cost weights. The cost is in mm² (RDK's native unit): NLopt's stop value and ftol are 1e-6, which is
// about 1 µm in mm² but 1 mm in m², where it would stall ~25 mm short of the per-tick goal.
const (
	solveOnceRotWeight  = 1e4  // mm² per rad²: 1 rad ≈ 100 mm
	solveOnceStepWeight = 1e-2 // mm² per rad² of joint motion, a tie-breaker toward the smallest step
	boxEps              = 1e-6 // rad, minimum box width
	badCost             = 1e18 // returned when FK fails, so NLopt backs away
)

type solveOnceSolver struct{ ik *ik.NloptIK }

// newSolveOnceSolver errors on builds without NLopt (no_cgo, windows).
func newSolveOnceSolver(logger logging.Logger) (Solver, error) {
	// NLopt's cost wrapper formats float slices on every evaluation at DEBUG (test loggers are DEBUG).
	// A private INFO sublogger keeps that cost out of SolveOnce timings so the benchmark against DLS is
	// fair, and leaves the caller's logger level alone.
	quiet := logger.Sublogger("solveonce")
	quiet.SetLevel(logging.INFO)
	n, err := ik.CreateNloptSolver(quiet, 1, false /* exact */, true /* useRelTol */, 0)
	if err != nil {
		return nil, err
	}
	return &solveOnceSolver{ik: n}, nil
}

func (s *solveOnceSolver) Name() string { return "solveonce" }

// stepBox returns the per-joint box [max(Lo, q-Vel·dt), min(Hi, q+Vel·dt)]. It never returns Max <= Min:
// a collapsed box (q at or beyond a limit, or Vel·dt = 0) is widened by boxEps toward the side inside
// [Lo, Hi]. If q starts outside [Lo, Hi], NLopt clamps the seed into the widened box, so dq can exceed the
// velocity box; Limits.Apply scales it back.
func stepBox(q []float64, l Limits, dt float64) []referenceframe.Limit {
	box := make([]referenceframe.Limit, len(q))
	for j := range q {
		lo, hi := math.Max(l.Lo[j], q[j]-l.Vel[j]*dt), math.Min(l.Hi[j], q[j]+l.Vel[j]*dt)
		if lo >= hi {
			c := math.Min(math.Max(lo, l.Lo[j]), l.Hi[j])
			if c+boxEps <= l.Hi[j] {
				lo, hi = c, c+boxEps
			} else {
				lo, hi = c-boxEps, c
			}
		}
		box[j] = referenceframe.Limit{Min: lo, Max: hi}
	}
	return box
}

func (s *solveOnceSolver) Step(in StepInput) (dq []float64, info StepInfo, err error) {
	start := time.Now()
	defer func() { info.SolveTime = time.Since(start) }()
	if err := checkDims(in); err != nil {
		return nil, info, err
	}
	cp, cq, err := FK(in.Model, in.Q)
	if err != nil {
		return nil, info, err
	}
	goal, capped := tickGoal(in, cp, cq)
	info.TCPCapped = capped
	gp, gq := goal.Point(), goal.Orientation().Quaternion()
	ga := toolAxis(gq)

	cost := func(_ context.Context, x []float64) float64 {
		info.Evals++ // FK/cost calls, finite-difference probes included
		p, err := in.Model.Transform(x)
		if err != nil {
			return badCost
		}
		var rot float64
		if in.Task == TaskPointing {
			r, _, _ := pointingError(toolAxis(p.Orientation().Quaternion()), ga)
			rot = r.Norm2()
		} else {
			rot = rotVec(gq, p.Orientation().Quaternion()).Norm2()
		}
		var step float64
		for j := range x {
			d := x[j] - in.Q[j]
			step += d * d
		}
		return p.Point().Sub(gp).Norm2() + solveOnceRotWeight*rot + solveOnceStepWeight*step
	}
	x, err := s.ik.SolveOnce(context.Background(), cost, in.Q, stepBox(in.Q, in.Limits, in.Dt))
	if err != nil {
		return nil, info, fmt.Errorf("solveonce: %w", err)
	}
	dq = make([]float64, len(x))
	for j := range x {
		dq[j] = x[j] - in.Q[j]
	}
	return dq, info, nil
}

// NewSolveOnceSolver returns the NLopt SolveOnce solver, for the glue. It errors on builds without NLopt.
func NewSolveOnceSolver(logger logging.Logger) (Solver, error) { return newSolveOnceSolver(logger) }
