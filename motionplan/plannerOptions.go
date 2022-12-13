package motionplan

import (
	"gonum.org/v1/gonum/floats"
)

// default values for planning options.
const (
	// max linear deviation from straight-line between start and goal, in mm.
	defaultLinearDeviation = 0.1

	// allowable deviation from slerp between start/goal orientations, unit is the norm of the R3AA between start and goal.
	defaultOrientationDeviation = 0.05

	// allowable linear and orientation deviation from direct interpolation path, as a proportion of the linear and orientation distances
	// between the start and goal.
	defaultPseudolinearTolerance = 0.8

	// Check constraints are still met every this many mm/degrees of movement.
	defaultResolution = 2.0

	// default number of seconds to try to solve in total before returning.
	defaultTimeout = 300.

	// default number of times to try to smooth the path.
	defaultSmoothIter = 20

	// names of constraints.
	defaultLinearConstraintName       = "defaultLinearConstraint"
	defaultPseudolinearConstraintName = "defaultPseudolinearConstraint"
	defaultOrientationConstraintName  = "defaultOrientationConstraint"
	defaultCollisionConstraintName    = "defaultCollisionConstraint"
	defaultJointConstraint            = "defaultJointSwingConstraint"

	// When breaking down a path into smaller waypoints, add a waypoint every this many mm of movement.
	defaultPathStepSize = 10

	// This is commented out due to Go compiler bug. See comment in newBasicPlannerOptions for explanation.
	// var defaultPlanner = newCBiRRTMotionPlanner.
)

// the set of supported motion profiles.
const (
	FreeMotionProfile         = "free"
	LinearMotionProfile       = "linear"
	PseudolinearMotionProfile = "pseudolinear"
	OrientationMotionProfile  = "orientation"
	PositionOnlyMotionProfile = "position_only"
)

// defaultDistanceFunc returns the square of the two-norm between the StartInput and EndInput vectors in the given ConstraintInput.
func defaultDistanceFunc(ci *ConstraintInput) (bool, float64) {
	diff := make([]float64, 0, len(ci.StartInput))
	for i, f := range ci.StartInput {
		diff = append(diff, f.Value-ci.EndInput[i].Value)
	}
	// 2 is the L value returning a standard L2 Normalization
	return true, floats.Norm(diff, 2)
}

// newBasicPlannerOptions specifies a set of basic options for the planner.
func newBasicPlannerOptions() *plannerOptions {
	opt := &plannerOptions{}
	opt.pathDist = NewSquaredNormMetric()

	// Set defaults
	opt.Resolution = defaultResolution
	opt.Timeout = defaultTimeout
	opt.DistanceFunc = defaultDistanceFunc

	// Note the direct reference to a default here.
	// This is due to a Go compiler issue where it will incorrectly refuse to compile with a circular reference error if this
	// is placed in a global default var.
	opt.PlannerConstructor = newCBiRRTMotionPlanner

	opt.SmoothIter = defaultSmoothIter
	opt.ikOptions = newBasicIKOptions()

	return opt
}

// plannerOptions are a set of options to be passed to a planner which will specify how to solve a motion planning problem.
type plannerOptions struct {
	*ikOptions
	pathDist Metric // Metric by which to measure nearness

	// Check constraints are still met every this many mm/degrees of movement.
	Resolution float64 `json:"resolution"`

	// Number of seconds before terminating planner
	Timeout float64 `json:"timeout"`

	// Number of times to try to smooth the path
	SmoothIter int `json:"smooth_iter"`

	// Function to use to measure distance between two inputs
	// TODO(rb): this should really become a Metric once we change the way the constraint system works, its awkward to return 2 values here
	DistanceFunc Constraint

	PlannerConstructor plannerConstructor

	Fallback *plannerOptions
}

// SetMetric sets the distance metric for the solver.
func (p *plannerOptions) SetMetric(m Metric) {
	p.metric = m
}

// SetPathDist sets the distance metric for the solver to move a constraint-violating point into a valid manifold.
func (p *plannerOptions) SetPathDist(m Metric) {
	p.pathDist = m
}

// SetMaxSolutions sets the maximum number of IK solutions to generate for the planner.
func (p *plannerOptions) SetMaxSolutions(maxSolutions int) {
	p.MaxSolutions = maxSolutions
}

// SetMinScore specifies the IK stopping score for the planner.
func (p *plannerOptions) SetMinScore(minScore float64) {
	p.MinScore = minScore
}
