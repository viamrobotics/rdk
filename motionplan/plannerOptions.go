//go:build !no_cgo

package motionplan

import (
	"math"
	"runtime"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// default values for planning options.
const (
	defaultCollisionBufferMM = 1e-8

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

	// default motion planning collision resolution is every 2mm.
	// For bases we increase this to 60mm, a bit more than 2 inches.
	defaultPTGCollisionResolution = 60

	// If an IK solution scores below this much, return it immediately.
	defaultMinIkScore = 0.

	// Default distance below which two distances are considered equal.
	defaultEpsilon = 0.001

	// default number of seconds to try to solve in total before returning.
	defaultTimeout = 300.

	// default number of times to try to smooth the path.
	defaultSmoothIter = 100

	// default number of position only seeds to use for tp-space planning.
	defaultTPspacePositionOnlySeeds = 16

	// random seed.
	defaultRandomSeed = 0

	// descriptions of constraints.
	defaultLinearConstraintDesc         = "Constraint to follow linear path"
	defaultPseudolinearConstraintDesc   = "Constraint to follow pseudolinear path, with tolerance scaled to path length"
	defaultOrientationConstraintDesc    = "Constraint to maintain orientation within bounds"
	defaultBoundingRegionConstraintDesc = "Constraint to maintain position within bounds"
	defaultObstacleConstraintDesc       = "Collision between the robot and an obstacle"
	defaultSelfCollisionConstraintDesc  = "Collision between two robot components that are moving"
	defaultRobotCollisionConstraintDesc = "Collision between a robot component that is moving and one that is stationary"

	// When breaking down a path into smaller waypoints, add a waypoint every this many mm of movement.
	defaultStepSizeMM = 10

	// This is commented out due to Go compiler bug. See comment in newBasicPlannerOptions for explanation.
	// var defaultPlanner = newCBiRRTMotionPlanner.
)

var defaultNumThreads = runtime.NumCPU() / 2

// TODO: Make this an enum
// the set of supported motion profiles.
const (
	FreeMotionProfile         = "free"
	LinearMotionProfile       = "linear"
	PseudolinearMotionProfile = "pseudolinear"
	OrientationMotionProfile  = "orientation"
	PositionOnlyMotionProfile = "position_only"
)

// NewBasicPlannerOptions specifies a set of basic options for the planner.
func newBasicPlannerOptions() *plannerOptions {
	opt := &plannerOptions{}
	opt.goalMetricConstructor = ik.NewSquaredNormMetric
	opt.configurationDistanceFunc = ik.FSConfigurationL2Distance
	opt.poseDistanceFunc = ik.WeightedSquaredNormSegmentMetric
	opt.nodeDistanceFunc = nodeConfigurationDistanceFunc
	opt.scoreFunc = ik.FSConfigurationL2Distance
	opt.pathMetric = ik.NewZeroFSMetric() // By default, the distance to the valid manifold is zero, unless constraints say otherwise

	// TODO: RSDK-6079 this should be properly used, and deduplicated with defaultEpsilon, InputIdentDist, etc.
	opt.GoalThreshold = 0.1
	// Set defaults
	opt.MaxSolutions = defaultSolutionsToSeed
	opt.MinScore = defaultMinIkScore
	opt.Resolution = defaultResolution
	opt.Timeout = defaultTimeout
	opt.PositionSeeds = defaultTPspacePositionOnlySeeds

	opt.PlanIter = defaultPlanIter
	opt.FrameStep = defaultFrameStep
	opt.InputIdentDist = defaultInputIdentDist
	opt.IterBeforeRand = defaultIterBeforeRand

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
	motionChains []*motionChain

	// This is used to create functions which are passed to IK for solving. This may be used to turn starting or ending state poses into
	// configurations for nodes.
	goalMetricConstructor func(spatialmath.Pose) ik.StateMetric

	pathMetric       ik.StateFSMetric         // Distance function which converges on the valid manifold of intermediate path states
	nodeDistanceFunc func(node, node) float64 // Node distance function used for nearest neighbor

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

	// If the dot product between two sets of inputs is less than this, consider them identical.
	InputIdentDist float64 `json:"input_ident_dist"`

	// Number of iterations to mrun before beginning to accept randomly seeded locations.
	IterBeforeRand int `json:"iter_before_rand"`

	// Number of seeds to pre-generate for bidirectional position-only solving.
	PositionSeeds int `json:"position_seeds"`

	// poseDistanceFunc is the function that the planner will use to measure the degree of "closeness" between two poses
	poseDistanceFunc ik.SegmentMetric

	// configurationDistanceFunc is the function that the planner will use to measure the degree of "closeness" between two states of the robot
	configurationDistanceFunc ik.SegmentFSMetric

	// scoreFunc is the function that the planner will use to evaluate a plan for final cost comparisons.
	scoreFunc ik.SegmentFSMetric

	// profile is the string representing the motion profile
	profile string

	PlannerConstructor plannerConstructor

	Fallback *plannerOptions

	useTPspace   bool
	ptgFrameName string
}

// getGoalMetric creates the distance metric for the solver using the configured options.
func (p *plannerOptions) getGoalMetric(goal referenceframe.FrameSystemPoses) ik.StateFSMetric {
	metrics := map[string]ik.StateMetric{}
	for frame, goalInFrame := range goal {
		metrics[frame] = p.goalMetricConstructor(goalInFrame.Pose())
	}
	return func(state *ik.StateFS) float64 {
		score := 0.
		for frame, goalMetric := range metrics {
			poseParent := goal[frame].Parent()
			currPose, err := state.FS.Transform(state.Configuration, referenceframe.NewZeroPoseInFrame(frame), poseParent)
			if err != nil {
				score += math.Inf(1)
			}
			score += goalMetric(&ik.State{
				Position:      currPose.(*referenceframe.PoseInFrame).Pose(),
				Configuration: state.Configuration[frame],
				Frame:         state.FS.Frame(frame),
			})
		}
		return score
	}
}

// SetPathDist sets the distance metric for the solver to move a constraint-violating point into a valid manifold.
func (p *plannerOptions) SetPathMetric(m ik.StateFSMetric) {
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

// addPbConstraints will add all constraints from the passed Constraint struct. This will deal with only the topological
// constraints. It will return a bool indicating whether there are any to add.
func (p *plannerOptions) addTopoConstraints(
	fs referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	constraints *Constraints,
) (bool, error) {
	topoConstraints := false
	for _, linearConstraint := range constraints.GetLinearConstraint() {
		topoConstraints = true
		// TODO RSDK-9224: Our proto for constraints does not allow the specification of which frames should be constrainted relative to
		// which other frames. If there is only one goal specified, then we assume that the constraint is between the moving and goal frame.
		err := p.addLinearConstraints(fs, startCfg, from, to, linearConstraint)
		if err != nil {
			return false, err
		}
	}
	for _, pseudolinearConstraint := range constraints.GetPseudolinearConstraint() {
		// pseudolinear constraints
		err := p.addPseudolinearConstraints(fs, startCfg, from, to, pseudolinearConstraint)
		if err != nil {
			return false, err
		}
	}
	for _, orientationConstraint := range constraints.GetOrientationConstraint() {
		topoConstraints = true
		// TODO RSDK-9224: Our proto for constraints does not allow the specification of which frames should be constrainted relative to
		// which other frames. If there is only one goal specified, then we assume that the constraint is between the moving and goal frame.
		err := p.addOrientationConstraints(fs, startCfg, from, to, orientationConstraint)
		if err != nil {
			return false, err
		}
	}
	return topoConstraints, nil
}

func (p *plannerOptions) addLinearConstraints(
	fs referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	linConstraint LinearConstraint,
) error {
	// Linear constraints
	linTol := linConstraint.LineToleranceMm
	if linTol == 0 {
		// Default
		linTol = defaultLinearDeviation
	}
	orientTol := linConstraint.OrientationToleranceDegs
	if orientTol == 0 {
		orientTol = defaultOrientationDeviation
	}
	constraint, pathDist, err := CreateAbsoluteLinearInterpolatingConstraintFS(fs, startCfg, from, to, linTol, orientTol)
	if err != nil {
		return err
	}
	p.AddStateFSConstraint(defaultLinearConstraintDesc, constraint)

	p.pathMetric = ik.CombineFSMetrics(p.pathMetric, pathDist)
	return nil
}

func (p *plannerOptions) addPseudolinearConstraints(
	fs referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	plinConstraint PseudolinearConstraint,
) error {
	// Linear constraints
	linTol := plinConstraint.LineToleranceFactor
	if linTol == 0 {
		// Default
		linTol = defaultPseudolinearTolerance
	}
	orientTol := plinConstraint.OrientationToleranceFactor
	if orientTol == 0 {
		orientTol = defaultPseudolinearTolerance
	}
	constraint, pathDist, err := CreateProportionalLinearInterpolatingConstraintFS(fs, startCfg, from, to, linTol, orientTol)
	if err != nil {
		return err
	}
	p.AddStateFSConstraint(defaultPseudolinearConstraintDesc, constraint)

	p.pathMetric = ik.CombineFSMetrics(p.pathMetric, pathDist)
	return nil
}

func (p *plannerOptions) addOrientationConstraints(
	fs referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	orientConstraint OrientationConstraint,
) error {
	orientTol := orientConstraint.OrientationToleranceDegs
	if orientTol == 0 {
		orientTol = defaultOrientationDeviation
	}
	constraint, pathDist, err := CreateSlerpOrientationConstraintFS(fs, startCfg, from, to, orientTol)
	if err != nil {
		return err
	}
	p.AddStateFSConstraint(defaultOrientationConstraintDesc, constraint)
	p.pathMetric = ik.CombineFSMetrics(p.pathMetric, pathDist)
	return nil
}

func (p *plannerOptions) fillMotionChains(fs referenceframe.FrameSystem, to *PlanState) error {
	motionChains := make([]*motionChain, 0, len(to.poses)+len(to.configuration))

	for frame, pif := range to.poses {
		chain, err := motionChainFromGoal(fs, frame, pif.Parent())
		if err != nil {
			return err
		}
		motionChains = append(motionChains, chain)
	}
	for frame := range to.configuration {
		chain, err := motionChainFromGoal(fs, frame, frame)
		if err != nil {
			return err
		}
		motionChains = append(motionChains, chain)
	}
	p.motionChains = motionChains
	return nil
}
