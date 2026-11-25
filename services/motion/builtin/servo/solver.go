package servo

import (
	"fmt"
	"time"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// Defaults ported from diff-ik's LoopConfig.
const (
	singularEps = 0.04 // smallest singular value below which damping ramps in
	maxDamping  = 0.1  // damping at a fully singular configuration
	maxDLSDoF   = 6    // dlsStep needs n <= 6
)

// Task selects what the solver tracks.
type Task int

// Task values.
const (
	TaskPose     Task = iota // full 6-D pose
	TaskPointing             // position plus tool axis; roll free (arms under 6 DOF)
)

// StepInput is one tick's problem. Target is in the arm's base frame, mm. FF is the feed-forward
// twist (m/s, rad/s). The solver returns an unlimited joint step; the caller applies Limits.
type StepInput struct {
	Model  referenceframe.Model
	Q      []float64
	Target spatialmath.Pose
	FF     [6]float64
	Gain   float64 // 1/s
	Dt     float64
	Task   Task
	Limits Limits // read-only: solvers may use the velocity box, never bypass Apply
}

// StepInfo reports per-tick diagnostics for status and the benchmark.
type StepInfo struct {
	Damping float64
	// TCPCapped: the linear or angular TCP limit capped this tick's twist.
	TCPCapped bool
	SigmaMin  float64 // smallest singular value of the Jacobian (DLS pose task only); 0 for SolveOnce
	// Evals counts cost/FK evaluations (SolveOnce), including NLopt's finite-difference probes: about (n+1) x the
	// NLopt evaluations, which the 4001 cap limits. 0 for DLS.
	Evals     int
	SolveTime time.Duration
}

// Solver computes one tick's joint step toward the target.
type Solver interface {
	Step(in StepInput) (dq []float64, info StepInfo, err error)
	Name() string
}

type dlsSolver struct{}

func newDLSSolver() Solver { return dlsSolver{} }

func (dlsSolver) Name() string { return "dls" }

// checkDims rejects limits that don't match the joint count (a malformed session, not a panic).
func checkDims(in StepInput) error {
	if n := len(in.Q); len(in.Limits.Lo) != n || len(in.Limits.Hi) != n || len(in.Limits.Vel) != n {
		return fmt.Errorf("limits have %d/%d/%d lo/hi/vel entries for %d joints", len(in.Limits.Lo), len(in.Limits.Hi), len(in.Limits.Vel), n)
	}
	return nil
}

// twist is one tick's Cartesian increment: lin in metres; ang in radians. For TaskPose ang is a base-frame
// rotation vector; for TaskPointing it is (a.X, a.Y) along the orthonormal pair u, v (u, v set).
type twist struct {
	lin, ang r3.Vector
	u, v     r3.Vector
	capped   bool // a TCP cap shortened lin or ang
}

// tickTwist builds the Cartesian twist FF + Gain·error, capped at Limits.MaxLinear/MaxAngular, times Dt,
// from the current FK (cp in metres, cq). Both solvers use it so they chase the same per-tick goal.
func tickTwist(in StepInput, cp r3.Vector, cq quat.Number) twist {
	tp, tq := in.Target.Point().Mul(1/mmPerM), in.Target.Orientation().Quaternion()
	e := poseError(tp, tq, cp, cq)
	var tw twist
	capped := func(v r3.Vector, limit float64) r3.Vector {
		tw.capped = tw.capped || v.Norm() > limit
		return capNorm(v, limit)
	}
	tw.lin = capped(r3.Vector{X: in.FF[0] + in.Gain*e[0], Y: in.FF[1] + in.Gain*e[1], Z: in.FF[2] + in.Gain*e[2]},
		in.Limits.MaxLinear).Mul(in.Dt)
	if in.Task == TaskPointing {
		r, u, v := pointingError(toolAxis(cq), toolAxis(tq))
		w := r3.Vector{X: in.FF[3], Y: in.FF[4], Z: in.FF[5]}
		tw.ang = capped(r3.Vector{X: u.Dot(w) + in.Gain*u.Dot(r), Y: v.Dot(w) + in.Gain*v.Dot(r)},
			in.Limits.MaxAngular).Mul(in.Dt)
		tw.u, tw.v = u, v
		return tw
	}
	tw.ang = capped(r3.Vector{X: in.FF[3] + in.Gain*e[3], Y: in.FF[4] + in.Gain*e[4], Z: in.FF[5] + in.Gain*e[5]},
		in.Limits.MaxAngular).Mul(in.Dt)
	return tw
}

// rotVec is the base-frame rotation vector the twist applies.
func (tw twist) rotVec(task Task) r3.Vector {
	if task == TaskPointing {
		return tw.u.Mul(tw.ang.X).Add(tw.v.Mul(tw.ang.Y))
	}
	return tw.ang
}

// tickGoal returns this tick's goal pose (mm) given the current FK (cp in metres, cq): the Cartesian twist
// FF + Gain·error, capped at Limits.MaxLinear/MaxAngular, integrated over Dt from the current pose. For
// TaskPointing the rotation is expRot(a.X·u + a.Y·v) applied to cq (u, v from pointingError), so roll stays
// where it is.
func tickGoal(in StepInput, cp r3.Vector, cq quat.Number) (spatialmath.Pose, bool) {
	tw := tickTwist(in, cp, cq)
	gq := quat.Mul(expRot(tw.rotVec(in.Task)), cq)
	return spatialmath.NewPose(cp.Add(tw.lin).Mul(mmPerM), (*spatialmath.Quaternion)(&gq)), tw.capped
}

func (dlsSolver) Step(in StepInput) (dq []float64, info StepInfo, err error) {
	start := time.Now()
	defer func() { info.SolveTime = time.Since(start) }()
	n := len(in.Q)
	if n > maxDLSDoF {
		return nil, info, fmt.Errorf("dls: %d-joint arms are not supported (max %d)", n, maxDLSDoF)
	}
	if err := checkDims(in); err != nil {
		return nil, info, err
	}
	cp, cq, err := FK(in.Model, in.Q)
	if err != nil {
		return nil, info, err
	}
	J, err := Jacobian(in.Model, in.Q)
	if err != nil {
		return nil, info, err
	}

	tw := tickTwist(in, cp, cq)
	lin := tw.lin
	info.TCPCapped = tw.capped

	if in.Task == TaskPointing {
		J1 := mat.DenseCopyOf(J.Slice(0, 3, 0, n))
		J2 := pointingRows(J, tw.u, tw.v)
		dx1, dx2 := []float64{lin.X, lin.Y, lin.Z}, []float64{tw.ang.X, tw.ang.Y}
		dq, l1, l2, err := prioritySolve(J1, J2, dx1, dx2, singularEps, maxDamping)
		// Lock outward-at-limit joints and re-solve so the post-hoc clamp doesn't corrupt the
		// position task. Each pass locks only new joints.
		for pass := 0; err == nil && pass < n && lockOutward(J1, J2, dq, in.Q, in.Limits.Lo, in.Limits.Hi); pass++ {
			dq, l1, l2, err = prioritySolve(J1, J2, dx1, dx2, singularEps, maxDamping)
		}
		if err != nil {
			return nil, info, err
		}
		info.Damping = max(l1, l2)
		return dq, info, nil
	}

	ang := tw.ang
	dx := [6]float64{lin.X, lin.Y, lin.Z, ang.X, ang.Y, ang.Z}
	dq, lambda, sigmaMin, err := dlsStep(J, [6]float64{1, 1, 1, 1, 1, 1}, dx, singularEps, maxDamping)
	if err != nil {
		return nil, info, err
	}
	info.Damping, info.SigmaMin = lambda, sigmaMin
	return dq, info, nil
}

// NewDLSSolver returns the damped-least-squares solver, for the glue.
func NewDLSSolver() Solver { return newDLSSolver() }
