package motionplan

import (
	"context"
	"math"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

const(
	// max linear deviation from straight-line between start and goal, in mm
	defaultLinearDeviation = 0.1
	// allowable deviation from slerp between start/goal orientations, unit is the norm of the R3AA between 
	defaultOrientationDeviation = 0.05
	// allowable linear and orientation deviation from direct interpolation path, as a proportion of the linear and orientation distances
	// between the start and goal
	defaultPseudolinearTolerance = 0.8
	// names of constraints
	defaultLinearConstraintName = "defaultLinearConstraint"
	defaultPseudolinearConstraintName = "defaultPseudolinearConstraint"
	defaultFreeConstraintName = "defaultFreeConstraint"
	defaultCollisionConstraintName = "defaultCollisionConstraint"
)

func plannerSetupFromMoveRequest(
	ctx context.Context,
	from, to spatial.Pose,
	f frame.Frame,
	fs frame.FrameSystem,
	seedMap map[string][]frame.Input,
	worldState *commonpb.WorldState,
	planningOpts map[string]interface{},
) (*PlannerOptions, error) {
	
	opt := NewBasicPlannerOptions()
	opt.extras = planningOpts
	
	collisionConstraint, err := NewCollisionConstraintFromWorldState(f, fs, worldState, seedMap)
	if err != nil {
		return nil, err
	}
	opt.AddConstraint(defaultCollisionConstraintName, collisionConstraint)
	
	switch planningOpts["motionProfile"] {
	case "linear":
		// Linear constraints 
		linTol, ok := planningOpts["lineTolerance"].(float64)
		if !ok {
			// Default
			linTol = defaultLinearDeviation
		}
		orientTol, ok := planningOpts["orientTolerance"].(float64)
		if !ok {
			// Default
			orientTol = defaultLinearDeviation
		}
		constraint, pathDist := NewAbsoluteLinearInterpolatingConstraint(from, to, linTol, orientTol)
		opt.AddConstraint(defaultLinearConstraintName, constraint)
		opt.pathDist = pathDist
	case "pseudolinear":
		tolerance, ok := planningOpts["tolerance"].(float64)
		if !ok {
			// Default
			tolerance = defaultPseudolinearTolerance
		}
		constraint, pathDist := NewProportionalLinearInterpolatingConstraint(from, to, tolerance)
		opt.AddConstraint(defaultLinearConstraintName, constraint)
		opt.pathDist = pathDist
	case "free":
		
	default:
		// By default, we will default to pseudolinear at first for a limited number of iterations, then will fall back to `free` if no
		// direct path is found.
		// TODO(pl): once RRT* is workable, replace `free` with that here.
	}
	
	return opt, nil
}


// NewBasicPlannerOptions specifies a set of basic options for the planner.
func NewBasicPlannerOptions() *PlannerOptions {
	opt := &PlannerOptions{}
	opt.AddConstraint(jointConstraint, NewJointConstraint(math.Inf(1)))
	opt.metric = NewSquaredNormMetric()
	opt.pathDist = NewSquaredNormMetric()
	return opt
}

// PlannerOptions are a set of options to be passed to a planner which will specify how to solve a motion planning problem.
type PlannerOptions struct {
	constraintHandler
	metric   Metric // Distance function to the goal
	pathDist Metric // Distance function to the nearest valid point
	extras map[string]interface{}
	// For the below values, if left uninitialized, default values will be used. To disable, set < 0
	// Max number of ik solutions to consider
	maxSolutions int
	// Movements that score below this amount are considered "good enough" and returned immediately
	minScore float64
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
	p.maxSolutions = maxSolutions
}

// SetMinScore specifies the IK stopping score for the planner.
func (p *PlannerOptions) SetMinScore(minScore float64) {
	p.minScore = minScore
}

// Clone makes a deep copy of the PlannerOptions.
//~ func (p *PlannerOptions) Clone() *PlannerOptions {
	//~ opt := &PlannerOptions{}
	//~ opt.constraints = p.constraints
	//~ opt.metric = p.metric
	//~ opt.pathDist = p.pathDist
	//~ opt.maxSolutions = p.maxSolutions
	//~ opt.minScore = p.minScore

	//~ return opt
//~ }
