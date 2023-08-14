package motionplan

import (
	"runtime"

	pb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// default values for planning options.
const (
	// max linear deviation from straight-line between start and goal, in mm.
	defaultLinearDeviation = 0.1

	// allowable deviation from slerp between start/goal orientations, unit is the number of degrees of rotation away from the most direct
	// arc from start orientation to goal orientation.
	defaultOrientationDeviation = 2.0

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
	defaultSmoothIter = 200

	// default amount of closeness to get to the goal.
	defaultGoalThreshold = defaultEpsilon * defaultEpsilon

	// descriptions of constraints.
	defaultLinearConstraintDesc         = "Constraint to follow linear path"
	defaultPseudolinearConstraintDesc   = "Constraint to follow pseudolinear path, with tolerance scaled to path length"
	defaultOrientationConstraintDesc    = "Constraint to maintain orientation within bounds"
	defaultObstacleConstraintDesc       = "Collision between the robot and an obstacle"
	defaultSelfCollisionConstraintDesc  = "Collision between two robot components that are moving"
	defaultRobotCollisionConstraintDesc = "Collision between a robot component that is moving and one that is stationary"

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

// NewBasicPlannerOptions specifies a set of basic options for the planner.
func newBasicPlannerOptions(frame referenceframe.Frame) *plannerOptions {
	opt := &plannerOptions{}
	opt.goalArcScore = JointMetric
	opt.DistanceFunc = L2InputMetric
	opt.pathMetric = NewZeroMetric() // By default, the distance to the valid manifold is zero, unless constraints say otherwise
	// opt.goalMetric is intentionally unset as it is likely dependent on the goal itself.

	// Set defaults
	opt.MaxSolutions = defaultSolutionsToSeed
	opt.MinScore = defaultMinIkScore
	opt.Resolution = defaultResolution
	opt.Timeout = defaultTimeout
	opt.GoalThreshold = defaultGoalThreshold

	opt.PlanIter = defaultPlanIter
	opt.FrameStep = defaultFrameStep
	opt.JointSolveDist = defaultJointSolveDist
	opt.IterBeforeRand = defaultIterBeforeRand
	opt.qstep = getFrameSteps(frame, defaultFrameStep)

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
	ConstraintHandler
	goalMetric   StateMetric // Distance function which converges to the final goal position
	goalArcScore SegmentMetric
	pathMetric   StateMetric // Distance function which converges on the valid manifold of intermediate path states

	extra map[string]interface{}

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

	// How close to get to the goal
	GoalThreshold float64 `json:"goal_threshold"`

	// Number of planner iterations before giving up.
	PlanIter int `json:"plan_iter"`

	// The maximum percent of a joints range of motion to allow per step.
	FrameStep float64 `json:"frame_step"`

	// If the dot product between two sets of joint angles is less than this, consider them identical.
	JointSolveDist float64 `json:"joint_solve_dist"`

	// Number of iterations to mrun before beginning to accept randomly seeded locations.
	IterBeforeRand int `json:"iter_before_rand"`

	// This is how far cbirrt will try to extend the map towards a goal per-step. Determined from FrameStep
	qstep []float64

	// DistanceFunc is the function that the planner will use to measure the degree of "closeness" between two states of the robot
	DistanceFunc SegmentMetric

	PlannerConstructor plannerConstructor

	Fallback *plannerOptions
}

// SetMetric sets the distance metric for the solver.
func (p *plannerOptions) SetGoalMetric(m StateMetric) {
	p.goalMetric = m
}

// SetPathDist sets the distance metric for the solver to move a constraint-violating point into a valid manifold.
func (p *plannerOptions) SetPathMetric(m StateMetric) {
	p.pathMetric = m
}

// SetMaxSolutions sets the maximum number of IK solutions to generate for the planner.
func (p *plannerOptions) SetMaxSolutions(maxSolutions int) {
	p.MaxSolutions = maxSolutions
}

// SetMinScore specifies the IK stopping score for the planner.
func (p *plannerOptions) SetMinScore(minScore float64) {
	p.MinScore = minScore
}

// addPbConstraints will add all constraints from the protobuf constraint specification. This will deal with only the topological
// constraints. It will return a bool indicating whether there are any to add.
func (p *plannerOptions) addPbTopoConstraints(from, to spatialmath.Pose, constraints *pb.Constraints) bool {
	topoConstraints := false
	for _, linearConstraint := range constraints.GetLinearConstraint() {
		topoConstraints = true
		p.addPbLinearConstraints(from, to, linearConstraint)
	}
	for _, orientationConstraint := range constraints.GetOrientationConstraint() {
		topoConstraints = true
		p.addPbOrientationConstraints(from, to, orientationConstraint)
	}
	return topoConstraints
}

func (p *plannerOptions) addPbLinearConstraints(from, to spatialmath.Pose, pbConstraint *pb.LinearConstraint) {
	// Linear constraints
	linTol := pbConstraint.GetLineToleranceMm()
	if linTol == 0 {
		// Default
		linTol = defaultLinearDeviation
	}
	orientTol := pbConstraint.GetOrientationToleranceDegs()
	if orientTol == 0 {
		orientTol = defaultOrientationDeviation
	}
	constraint, pathDist := NewAbsoluteLinearInterpolatingConstraint(from, to, float64(linTol), float64(orientTol))
	p.AddStateConstraint(defaultLinearConstraintDesc, constraint)

	p.pathMetric = CombineMetrics(p.pathMetric, pathDist)
}

func (p *plannerOptions) addPbOrientationConstraints(from, to spatialmath.Pose, pbConstraint *pb.OrientationConstraint) {
	orientTol := pbConstraint.GetOrientationToleranceDegs()
	if orientTol == 0 {
		orientTol = defaultOrientationDeviation
	}
	constraint, pathDist := NewSlerpOrientationConstraint(from, to, float64(orientTol))
	p.AddStateConstraint(defaultOrientationConstraintDesc, constraint)
	p.pathMetric = CombineMetrics(p.pathMetric, pathDist)
}
