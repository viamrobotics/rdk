package motionplan

import (
	"math"
	"runtime"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
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
	defaultSolutionsToSeed = 100

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

	// constraints passed over the wire do not get names and we want to call them something.
	defaultConstraintName = "unnamed constraint"

	// When breaking down a path into smaller waypoints, add a waypoint every this many mm of movement.
	defaultStepSizeMM = 10

	// This is commented out due to Go compiler bug. See comment in newBasicPlannerOptions for explanation.
	// var defaultPlanner = newCBiRRTMotionPlanner.
)

var (
	defaultNumThreads                            = utils.MinInt(runtime.NumCPU()/2, 10)
	defaultTimeMultipleAfterFindingFirstSolution = 10
)

func init() {
	defaultTimeMultipleAfterFindingFirstSolution = utils.GetenvInt("MP_TIME_MULTIPLIER", defaultTimeMultipleAfterFindingFirstSolution)
	defaultNumThreads = utils.GetenvInt("MP_NUM_THREADS", defaultNumThreads)
}

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
	opt.GoalMetricType = ik.SquaredNorm
	opt.ConfigurationDistanceMetric = ik.FSConfigurationL2DistanceMetric
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

	opt.PlanningAlgorithm = CBiRRT

	opt.SmoothIter = defaultSmoothIter

	opt.TimeMultipleAfterFindingFirstSolution = defaultTimeMultipleAfterFindingFirstSolution
	opt.NumThreads = defaultNumThreads

	opt.motionChains = &motionChains{}

	return opt
}

// plannerOptions are a set of options to be passed to a planner which will specify how to solve a motion planning problem.
type plannerOptions struct {
	ConstraintHandler
	motionChains *motionChains

	// This is used to create functions which are passed to IK for solving. This may be used to turn starting or ending state poses into
	// configurations for nodes.
	GoalMetricType ik.GoalMetricType `json:"goal_metric_type"`

	// Acceptable arc length around the goal orientation vector for any solution. This is the additional parameter used to acquire
	// the goal metric only if the GoalMetricType is ik.ArcLengthConvergence
	ArcLengthTolerance float64 `json:"arc_length_tolerance"`

	pathMetric ik.StateFSMetric // Distance function which converges on the valid manifold of intermediate path states

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

	// If at least one intermediate waypoint is solved for, but the plan fails before reaching the ultimate goal,
	// this will if true return the valid plan up to the last solved waypoint.
	ReturnPartialPlan bool `json:"return_partial_plan"`

	// ScoringMetricStr is an enum indicating the function that the planner will use to evaluate a plan for final cost comparisons.
	ScoringMetricStr ik.ScoringMetric `json:"scoring_metric"`

	// TPSpaceOrientationScale is the scale factor on orientation for the squared norm segment metric used
	// to calculate the distance between poses when planning for a TP-space frame
	TPSpaceOrientationScale float64 `json:"tp_space_orientation_scale"`

	// Determines the algorithm that the planner will use to measure the degree of "closeness" between two states of the robot
	ConfigurationDistanceMetric ik.SegmentFSMetricType

	// profile is the string representing the motion profile
	profile string

	PlanningAlgorithm PlanningAlgorithm `json:"planning_algorithm"`

	Fallback *plannerOptions

	TimeMultipleAfterFindingFirstSolution int
}

// getGoalMetric creates the distance metric for the solver using the configured options.
func (p *plannerOptions) getGoalMetric(goal referenceframe.FrameSystemPoses) ik.StateFSMetric {
	metrics := map[string]ik.StateMetric{}
	for frame, goalInFrame := range goal {
		switch p.GoalMetricType {
		case ik.PositionOnly:
			metrics[frame] = ik.NewPositionOnlyMetric(goalInFrame.Pose())
		case ik.SquaredNorm:
			metrics[frame] = ik.NewSquaredNormMetric(goalInFrame.Pose())
		case ik.ArcLengthConvergence:
			metrics[frame] = ik.NewPoseFlexOVMetricConstructor(p.ArcLengthTolerance)(goalInFrame.Pose())
		default:
			metrics[frame] = ik.NewSquaredNormMetric(goalInFrame.Pose())
		}
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

// In the scenario where we use TP-space, we call this to retrieve a function that computes distances
// in cartesian space rather than configuration space. The planner will use this to measure the degree of "closeness"
// between two poses.
func (p *plannerOptions) getPoseDistanceFunc() ik.SegmentMetric {
	return ik.NewSquaredNormSegmentMetric(p.TPSpaceOrientationScale)
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

func (p *plannerOptions) useTPspace() bool {
	if p.motionChains == nil {
		return false
	}
	return p.motionChains.useTPspace
}

func (p *plannerOptions) ptgFrameName() string {
	if p.motionChains == nil {
		return ""
	}
	return p.motionChains.ptgFrameName
}

func (p *plannerOptions) ScoringMetric() ik.ScoringMetric {
	if p.ScoringMetricStr == "" {
		return ik.FSConfigL2ScoringMetric
	}
	return p.ScoringMetricStr
}

func (p *plannerOptions) getScoringFunction() ik.SegmentFSMetric {
	switch p.ScoringMetric() {
	case ik.FSConfigScoringMetric:
		return ik.FSConfigurationDistance
	case ik.FSConfigL2ScoringMetric:
		return ik.FSConfigurationL2Distance
	case ik.PTGDistance:
		return tpspace.NewPTGDistanceMetric([]string{p.ptgFrameName()})
	default:
		return ik.FSConfigurationL2Distance
	}
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
	p.AddStateFSConstraint(defaultConstraintName, constraint)

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
	p.AddStateFSConstraint(defaultConstraintName, constraint)

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
	p.AddStateFSConstraint(defaultConstraintName, constraint)
	p.pathMetric = ik.CombineFSMetrics(p.pathMetric, pathDist)
	return nil
}
