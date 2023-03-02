package motionplan

import (
	"math"
	"runtime"

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

	// Number of IK solutions that should be generated before stopping.
	defaultSolutionsToSeed = 50

	// Check constraints are still met every this many mm/degrees of movement.
	defaultResolution = 2.0

	// If an IK solution scores below this much, return it immediately.
	defaultMinIkScore = 0.

	// Default distance below which two distances are considered equal.
	defaultEpsilon = 0.001

	// default number of seconds to try to solve in total before returning.
	defaultTimeout = 300.

	// default number of times to try to smooth the path.
	defaultSmoothIter = 20

	// names of constraints.
	defaultLinearConstraintName        = "defaultLinearConstraint"
	defaultPseudolinearConstraintName  = "defaultPseudolinearConstraint"
	defaultOrientationConstraintName   = "defaultOrientationConstraint"
	defaultObstacleConstraintName      = "defaultObstacleConstraint"
	defaultSelfCollisionConstraintName = "defaultSelfCollisionConstraint"
	defaultJointConstraint             = "defaultJointSwingConstraint"

	// When breaking down a path into smaller waypoints, add a waypoint every this many mm of movement.
	defaultPathStepSize = 10

	// This is commented out due to Go compiler bug. See comment in newBasicPlannerOptions for explanation.
	// var defaultPlanner = newCBiRRTMotionPlanner.
)

var defaultNumThreads = runtime.NumCPU() / 2

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

// NewBasicPlannerOptions specifies a set of basic options for the planner.
func newBasicPlannerOptions() *plannerOptions {
	opt := &plannerOptions{}
	opt.AddConstraint(defaultJointConstraint, NewJointConstraint(math.Inf(1)))
	opt.metric = NewSquaredNormMetric()
	opt.pathDist = NewSquaredNormMetric()

	// Set defaults
	opt.MaxSolutions = defaultSolutionsToSeed
	opt.MinScore = defaultMinIkScore
	opt.Resolution = defaultResolution
	opt.Timeout = defaultTimeout
	opt.DistanceFunc = defaultDistanceFunc

	// Note the direct reference to a default here.
	// This is due to a Go compiler issue where it will incorrectly refuse to compile with a circular reference error if this
	// is placed in a global default var.
	opt.PlannerConstructor = newCBiRRTMotionPlanner

	opt.SmoothIter = defaultSmoothIter

	opt.NumThreads = defaultNumThreads

	return opt
}

// plannerOptions are a set of options to be passed to a planner which will specify how to solve a motion planning problem.
type plannerOptions struct {
	constraintHandler
	metric   Metric // Distance function to the goal
	pathDist Metric // Distance function to the nearest valid point
	extra    map[string]interface{}

	// For the below values, if left uninitialized, default values will be used. To disable, set < 0
	// Max number of ik solutions to consider
	MaxSolutions int `json:"max_ik_solutions"`

	// Movements that score below this amount are considered "good enough" and returned immediately
	MinScore float64 `json:"min_ik_score"`

	// Check constraints are still met every this many mm/degrees of movement.
	Resolution float64 `json:"resolution"`

	// Percentage interval of max iterations after which to print debug logs
	LoggingInterval float64 `json:"logging_interval"`

	// Number of seconds before terminating planner
	Timeout float64 `json:"timeout"`

	// Number of times to try to smooth the path
	SmoothIter int `json:"smooth_iter"`

	// Number of cpu cores to use
	NumThreads int `json:"num_threads"`

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
