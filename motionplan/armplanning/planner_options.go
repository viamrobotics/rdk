package armplanning

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

// default values for planning options.
const (
	defaultCollisionBufferMM = 1e-8

	// Number of IK solutions that should be generated before stopping.
	defaultSolutionsToSeed = 100

	// Check constraints are still met every this many mm/degrees of movement.
	defaultResolution = 2.0

	// If an IK solution scores below this much, return it immediately.
	defaultMinIkScore = 0.01

	// default number of seconds to try to solve in total before returning.
	defaultTimeout = 300.

	// random seed.
	defaultRandomSeed = 0

	// When breaking down a path into smaller waypoints, add a waypoint every this many mm of movement.
	defaultStepSizeMM = 10

	// The maximum percent of a joints range of motion to allow per step.
	defaultFrameStep = 0.01

	// If the dot product between two sets of configurations is less than this, consider them identical.
	defaultInputIdentDist = 0.0001

	// Number of iterations to run before beginning to accept randomly seeded locations.
	defaultIterBeforeRand = 50

	defaultOptimalityMultiple = 3.0
)

var defaultNumThreads = utils.MinInt(runtime.NumCPU()/2, 10)

func init() {
	defaultNumThreads = utils.GetenvInt("MP_NUM_THREADS", defaultNumThreads)
}

// NewBasicPlannerOptions specifies a set of basic options for the planner.
func NewBasicPlannerOptions() *PlannerOptions {
	opt := &PlannerOptions{}
	opt.GoalMetricType = motionplan.SquaredNorm
	opt.ConfigurationDistanceMetric = motionplan.FSConfigurationL2DistanceMetric

	// TODO: RSDK-6079 this should be properly used, and deduplicated with defaultEpsilon, InputIdentDist, etc.
	opt.GoalThreshold = 0.1
	// Set defaults
	opt.MaxSolutions = defaultSolutionsToSeed
	opt.MinScore = defaultMinIkScore
	opt.Resolution = defaultResolution
	opt.Timeout = defaultTimeout

	opt.FrameStep = defaultFrameStep
	opt.InputIdentDist = defaultInputIdentDist
	opt.IterBeforeRand = defaultIterBeforeRand

	opt.CollisionBufferMM = defaultCollisionBufferMM
	opt.RandomSeed = defaultRandomSeed

	return opt
}

// PlannerOptions are a set of options to be passed to a planner which will specify how to solve a motion planning problem.
type PlannerOptions struct {
	// This is used to create functions which are passed to IK for solving. This may be used to turn starting or ending state poses into
	// configurations for nodes.
	GoalMetricType motionplan.GoalMetricType `json:"goal_metric_type"`

	// For the below values, if left uninitialized, default values will be used. To disable, set < 0
	// Max number of ik solutions to consider
	MaxSolutions int `json:"max_ik_solutions"`

	// Movements that score below this amount are considered "good enough" and returned immediately
	MinScore float64 `json:"min_ik_score"`

	// Check constraints are still met every this many mm/degrees of movement.
	Resolution float64 `json:"resolution"`

	// Number of seconds before terminating planner
	Timeout float64 `json:"timeout"`

	// How close to get to the goal
	GoalThreshold float64 `json:"goal_threshold"`

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

	// Determines the algorithm that the planner will use to measure the degree of "closeness" between two states of the robot
	// See metrics.go for options
	ConfigurationDistanceMetric motionplan.SegmentFSMetricType `json:"configuration_distance_metric"`

	// No two geometries that did not start the motion in collision may come within this distance of
	// one another at any time during a motion.
	CollisionBufferMM float64 `json:"collision_buffer_mm"`

	// The random seed used by motion algorithms during planning. This parameter guarantees deterministic
	// outputs for a given set of identical inputs
	RandomSeed int `json:"rseed"`

	// Setting indicating that all mesh geometries should be converted into octrees.
	MeshesAsOctrees bool `json:"meshes_as_octrees"`
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

	return opt, nil
}

// getGoalMetric creates the distance metric for the solver using the configured options.
func (p *PlannerOptions) getGoalMetric(goals referenceframe.FrameSystemPoses) motionplan.StateFSMetric {
	cartesianScale := 0.1
	orientScale := 10.0

	if p.GoalMetricType == motionplan.PositionOnly {
		orientScale = 0
	}

	return func(state *motionplan.StateFS) float64 {
		score := 0.
		for frame, goal := range goals {
			dq, err := state.FS.TransformToDQ(state.Configuration, frame, goal.Parent())
			if err != nil {
				panic(fmt.Errorf("frame: %v goal parent: %s", frame, goal.Parent()))
			}

			score += motionplan.WeightedSquaredNormDistanceWithOptions(goal.Pose(), &dq, cartesianScale, orientScale)
		}
		return score
	}
}

// SetMaxSolutions sets the maximum number of IK solutions to generate for the planner.
func (p *PlannerOptions) SetMaxSolutions(maxSolutions int) {
	p.MaxSolutions = maxSolutions
}

// SetMinScore specifies the IK stopping score for the planner.
func (p *PlannerOptions) SetMinScore(minScore float64) {
	p.MinScore = minScore
}

func (p *PlannerOptions) timeoutDuration() time.Duration {
	return time.Duration(p.Timeout * float64(time.Second))
}
