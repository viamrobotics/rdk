package motionplan

import (
	"encoding/json"
	"errors"
	"fmt"
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

// MotionProfile is an enum which indicates the motion profile to use when planning.
type MotionProfile string

// These are the currently supported motion profiles.
const (
	FreeMotionProfile         MotionProfile = "free"
	LinearMotionProfile       MotionProfile = "linear"
	PseudolinearMotionProfile MotionProfile = "pseudolinear"
	OrientationMotionProfile  MotionProfile = "orientation"
	PositionOnlyMotionProfile MotionProfile = "position_only"
)

// NewBasicPlannerOptions specifies a set of basic options for the planner.
func NewBasicPlannerOptions() *PlannerOptions {
	opt := &PlannerOptions{}
	opt.GoalMetricType = ik.SquaredNorm
	opt.ConfigurationDistanceMetric = ik.FSConfigurationL2DistanceMetric
	opt.ScoringMetric = ik.FSConfigL2ScoringMetric

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

	opt.PlanningAlgorithmSettings = AlgorithmSettings{
		Algorithm: UnspecifiedAlgorithm,
	}

	opt.SmoothIter = defaultSmoothIter

	opt.TimeMultipleAfterFindingFirstSolution = defaultTimeMultipleAfterFindingFirstSolution
	opt.NumThreads = defaultNumThreads

	opt.LineTolerance = defaultLinearDeviation
	opt.OrientationTolerance = defaultOrientationDeviation
	opt.ToleranceFactor = defaultPseudolinearTolerance

	opt.PathStepSize = defaultStepSizeMM
	opt.CollisionBufferMM = defaultCollisionBufferMM
	opt.RandomSeed = defaultRandomSeed

	return opt
}

// PlannerOptions are a set of options to be passed to a planner which will specify how to solve a motion planning problem.
type PlannerOptions struct {
	// This is used to create functions which are passed to IK for solving. This may be used to turn starting or ending state poses into
	// configurations for nodes.
	GoalMetricType ik.GoalMetricType `json:"goal_metric_type"`

	// Acceptable arc length around the goal orientation vector for any solution. This is the additional parameter used to acquire
	// the goal metric only if the GoalMetricType is ik.ArcLengthConvergence
	ArcLengthTolerance float64 `json:"arc_length_tolerance"`

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
	ScoringMetric ik.ScoringMetric `json:"scoring_metric"`

	// TPSpaceOrientationScale is the scale factor on orientation for the squared norm segment metric used
	// to calculate the distance between poses when planning for a TP-space frame
	TPSpaceOrientationScale float64 `json:"tp_space_orientation_scale"`

	// Determines the algorithm that the planner will use to measure the degree of "closeness" between two states of the robot
	// See metrics.go for options
	ConfigurationDistanceMetric ik.SegmentFSMetricType `json:"configuration_distance_metric"`

	// A profile indicating which of the tolerance parameters listed below should be considered
	// for further constraining the motion.
	MotionProfile MotionProfile `json:"motion_profile"`

	// Linear tolerance for translational deviation for a path. Only used when the
	// `MotionProfile` is `LinearMotionProfile`.
	LineTolerance float64 `json:"line_tolerance"`

	// Orientation tolerance for angular deviation for a path. Used for either the `LinearMotionProfile`
	// or the `OrientationMotionProfile`.
	OrientationTolerance float64 `json:"orient_tolerance"`

	// A factor by which the entire pose is allowed to deviate for a path. Used only for a PseudolinearMotionProfile.
	ToleranceFactor float64 `json:"tolerance"`

	// No two geometries that did not start the motion in collision may come within this distance of
	// one another at any time during a motion.
	CollisionBufferMM float64 `json:"collision_buffer_mm"`

	// The algorithm used for pathfinding along with any configurable settings for that algorithm. If this
	// object is not provided, motion planning will attempt to use RRT* and, in the event of failure
	// to find an acceptable path, it will fallback to cBiRRT.
	PlanningAlgorithmSettings AlgorithmSettings `json:"planning_algorithm_settings"`

	// The random seed used by motion algorithms during planning. This parameter guarantees deterministic
	// outputs for a given set of identical inputs
	RandomSeed int `json:"rseed"`

	// The max movement allowed for each step on the path from the initial random seed for a solution
	// to the goal.
	PathStepSize float64 `json:"path_step_size"`

	// Setting indicating that all mesh geometries should be converted into octrees.
	MeshesAsOctrees bool `json:"meshes_as_octrees"`

	// A set of fallback options to use on initial planning failure. This is used to facilitate the default
	// behavior described above in the comment for `PlanningAlgorithmSettings`. This will be populated
	// automatically if needed and is not meant to be set by users of the library.
	Fallback *PlannerOptions `json:"fallback_options"`

	// For inverse kinematics, the time within which each pending solution must finish its computation is
	// a multiple of the time taken to compute the first solution. This parameter is a way to
	// set that multiplicative factor.
	TimeMultipleAfterFindingFirstSolution int `json:"time_multiple_after_finding_first_solution"`
}

// NewPlannerOptionsFromExtra returns basic default settings updated by overridden parameters
// found in the "extra" of protobuf MoveRequest. The extra must be converted to an instance of
// map[string]interface{} first.
func NewPlannerOptionsFromExtra(extra map[string]interface{}) (*PlannerOptions, error) {
	opt := NewBasicPlannerOptions()

	jsonString, err := json.Marshal(extra)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonString, opt)
	if err != nil {
		return nil, err
	}

	if opt.CollisionBufferMM < 0 {
		return nil, errors.New("collision_buffer_mm can't be negative")
	}

	// we want to deprecate, rather than break, usage of the "tolerance" key for
	// OrientationMotionProfile
	if opt.MotionProfile == OrientationMotionProfile {
		opt.OrientationTolerance = opt.ToleranceFactor
	}
	return opt, nil
}

// Returns an updated PlannerOptions taking into account whether TP-Space is being used and whether
// a Free or Position-Only motion profile was requested with an unspecified algorithm (indicating the desire
// to let motionplan handle what algorithm to use and allow for a fallback).
func updateOptionsForPlanning(opt *PlannerOptions, useTPSpace bool) (*PlannerOptions, error) {
	optCopy := *opt
	planningAlgorithm := optCopy.PlanningAlgorithm()
	if useTPSpace && (planningAlgorithm != UnspecifiedAlgorithm) && (planningAlgorithm != TPSpace) {
		return nil, fmt.Errorf("cannot specify a planning algorithm when planning for a TP-space frame. alg specified was %s",
			planningAlgorithm)
	}

	if useTPSpace {
		// overwrite default with TP space
		optCopy.PlanningAlgorithmSettings = AlgorithmSettings{
			Algorithm: TPSpace,
		}

		optCopy.TPSpaceOrientationScale = defaultTPspaceOrientationScale

		optCopy.Resolution = defaultPTGCollisionResolution

		// If we have PTGs, then we calculate distances using the PTG-specific distance function.
		// Otherwise we just use squared norm on inputs.
		optCopy.ScoringMetric = ik.PTGDistance
	}

	if optCopy.MotionProfile == FreeMotionProfile || optCopy.MotionProfile == PositionOnlyMotionProfile {
		if optCopy.PlanningAlgorithm() == UnspecifiedAlgorithm {
			fallbackOpts := &optCopy

			optCopy.Timeout = defaultFallbackTimeout
			optCopy.PlanningAlgorithmSettings = AlgorithmSettings{
				Algorithm: RRTStar,
			}
			optCopy.Fallback = fallbackOpts
		}
	}

	return &optCopy, nil
}

// PlanningAlgorithm returns the label of the planning algorithm in plannerOptions.
func (p *PlannerOptions) PlanningAlgorithm() PlanningAlgorithm {
	return p.PlanningAlgorithmSettings.Algorithm
}

// getGoalMetric creates the distance metric for the solver using the configured options.
func (p *PlannerOptions) getGoalMetric(goal referenceframe.FrameSystemPoses) ik.StateFSMetric {
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
func (p *PlannerOptions) getPoseDistanceFunc() ik.SegmentMetric {
	return ik.NewSquaredNormSegmentMetric(p.TPSpaceOrientationScale)
}

// SetMaxSolutions sets the maximum number of IK solutions to generate for the planner.
func (p *PlannerOptions) SetMaxSolutions(maxSolutions int) {
	p.MaxSolutions = maxSolutions
}

// SetMinScore specifies the IK stopping score for the planner.
func (p *PlannerOptions) SetMinScore(minScore float64) {
	p.MinScore = minScore
}

func (p *PlannerOptions) getScoringFunction(mcs *motionChains) ik.SegmentFSMetric {
	switch p.ScoringMetric {
	case ik.FSConfigScoringMetric:
		return ik.FSConfigurationDistance
	case ik.FSConfigL2ScoringMetric:
		return ik.FSConfigurationL2Distance
	case ik.PTGDistance:
		return tpspace.NewPTGDistanceMetric([]string{mcs.ptgFrameName})
	default:
		return ik.FSConfigurationL2Distance
	}
}
