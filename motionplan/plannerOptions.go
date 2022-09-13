package motionplan

import (
	"encoding/json"
	"errors"
	"math"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
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

	// names of constraints.
	defaultLinearConstraintName       = "defaultLinearConstraint"
	defaultPseudolinearConstraintName = "defaultPseudolinearConstraint"
	defaultOrientationConstraintName  = "defaultOrientationConstraint"
	defaultCollisionConstraintName    = "defaultCollisionConstraint"
	defaultJointConstraint            = "defaultJointSwingConstraint"

	// When breaking down a path into smaller waypoints, add a waypoint every this many mm of movement.
	defaultPathStepSize = 10
)

// the set of supported motion profiles.
const (
	FreeMotionProfile         = "free"
	LinearMotionProfile       = "linear"
	PseudolinearMotionProfile = "pseudolinear"
	OrientationMotionProfile  = "orientation"
)

// defaultDistanceFunc returns the square of the two-norm between the StartInput and EndInput vectors in the given ConstraintInput.
func defaultDistanceFunc(ci *ConstraintInput) (bool, float64) {
	dist := 0.
	for i, f := range ci.StartInput {
		dist += math.Pow(ci.EndInput[i].Value-f.Value, 2)
	}
	return true, dist
}

func plannerSetupFromMoveRequest(
	from, to spatial.Pose,
	f frame.Frame,
	fs frame.FrameSystem,
	seedMap map[string][]frame.Input,
	worldState *commonpb.WorldState,
	planningOpts map[string]interface{},
) (*PlannerOptions, error) {
	opt := NewBasicPlannerOptions()
	opt.extra = planningOpts

	collisionConstraint, err := NewCollisionConstraintFromWorldState(f, fs, worldState, seedMap)
	if err != nil {
		return nil, err
	}
	opt.AddConstraint(defaultCollisionConstraintName, collisionConstraint)

	// error handling around extracting motion_profile information from map[string]interface{}
	var motionProfile string
	profile, ok := planningOpts["motion_profile"]
	if ok {
		motionProfile, ok = profile.(string)
		if !ok {
			return nil, errors.New("could not interpret motion_profile field as string")
		}
	}

	switch motionProfile {
	case LinearMotionProfile:
		// Linear constraints
		linTol, ok := planningOpts["line_tolerance"].(float64)
		if !ok {
			// Default
			linTol = defaultLinearDeviation
		}
		orientTol, ok := planningOpts["orient_tolerance"].(float64)
		if !ok {
			// Default
			orientTol = defaultLinearDeviation
		}
		constraint, pathDist := NewAbsoluteLinearInterpolatingConstraint(from, to, linTol, orientTol)
		opt.AddConstraint(defaultLinearConstraintName, constraint)
		opt.pathDist = pathDist
	case PseudolinearMotionProfile:
		tolerance, ok := planningOpts["tolerance"].(float64)
		if !ok {
			// Default
			tolerance = defaultPseudolinearTolerance
		}
		constraint, pathDist := NewProportionalLinearInterpolatingConstraint(from, to, tolerance)
		opt.AddConstraint(defaultPseudolinearConstraintName, constraint)
		opt.pathDist = pathDist
	case OrientationMotionProfile:
		tolerance, ok := planningOpts["tolerance"].(float64)
		if !ok {
			// Default
			tolerance = defaultOrientationDeviation
		}
		constraint, pathDist := NewSlerpOrientationConstraint(from, to, tolerance)
		opt.AddConstraint(defaultOrientationConstraintName, constraint)
		opt.pathDist = pathDist
	case FreeMotionProfile:
		// No restrictions on motion
	default:
		// TODO(pl): once RRT* is workable, use here. Also, update to try pseudolinear first, and fall back to orientation, then to free
		// if unsuccessful
	}

	// convert map to json, then to a struct, overwriting present defaults
	jsonString, err := json.Marshal(planningOpts)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonString, opt)
	if err != nil {
		return nil, err
	}
	return opt, nil
}

// NewBasicPlannerOptions specifies a set of basic options for the planner.
func NewBasicPlannerOptions() *PlannerOptions {
	opt := &PlannerOptions{}
	opt.AddConstraint(defaultJointConstraint, NewJointConstraint(math.Inf(1)))
	opt.metric = NewSquaredNormMetric()
	opt.pathDist = NewSquaredNormMetric()

	// Set defaults
	opt.MaxSolutions = defaultSolutionsToSeed
	opt.MinScore = defaultMinIkScore
	opt.Resolution = defaultResolution
	opt.DistanceFunc = defaultDistanceFunc
	return opt
}

// PlannerOptions are a set of options to be passed to a planner which will specify how to solve a motion planning problem.
// TODO(rb): make this a private struct so that somebody can't just make their own and initialize wrong.
type PlannerOptions struct {
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

	// Function to use to measure distance between two inputs
	// TODO(rb): this should really become a Metric once we change the way the constraint system works, its awkward to return 2 values here
	DistanceFunc Constraint
}

// SetMetric sets the distance metric for the solver.
func (p *PlannerOptions) SetMetric(m Metric) {
	p.metric = m
}

// SetPathDist sets the distance metric for the solver to move a constraint-violating point into a valid manifold.
func (p *PlannerOptions) SetPathDist(m Metric) {
	p.pathDist = m
}

// SetMaxSolutions sets the maximum number of IK solutions to generate for the planner.
func (p *PlannerOptions) SetMaxSolutions(maxSolutions int) {
	p.MaxSolutions = maxSolutions
}

// SetMinScore specifies the IK stopping score for the planner.
func (p *PlannerOptions) SetMinScore(minScore float64) {
	p.MinScore = minScore
}
