package motionplan

import (
	"fmt"
	"math"
	"runtime"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/service/motion/v1"
	"gonum.org/v1/gonum/floats"

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
	opt.pathDist = NewZeroMetric() // By default, the distance to the valid manifold is zero, unless constraints say otherwise

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
	p.AddConstraint(defaultLinearConstraintName, constraint)

	p.pathDist = CombineMetrics(p.pathDist, pathDist)
}

func (p *plannerOptions) addPbOrientationConstraints(from, to spatialmath.Pose, pbConstraint *pb.OrientationConstraint) {
	orientTol := pbConstraint.GetOrientationToleranceDegs()
	if orientTol == 0 {
		orientTol = defaultOrientationDeviation
	}
	constraint, pathDist := NewSlerpOrientationConstraint(from, to, float64(orientTol))
	p.AddConstraint(defaultLinearConstraintName, constraint)
	p.pathDist = CombineMetrics(p.pathDist, pathDist)
}

func (p *plannerOptions) createCollisionConstraints(
	frame referenceframe.Frame,
	fs referenceframe.FrameSystem,
	worldState *referenceframe.WorldState,
	seedMap map[string][]referenceframe.Input,
	pbConstraint []*pb.CollisionSpecification,
	reportDistances bool,
	logger golog.Logger,
) error {
	allowedCollisions := []*Collision{}

	// List of all names which may be specified for collision ignoring.
	validGeoms := map[string]bool{}

	addGeomNames := func(parentName string, geomsInFrame *referenceframe.GeometriesInFrame) error {
		for _, geom := range geomsInFrame.Geometries() {
			geomName := geom.Label()

			// Ensure we're not double-adding components which only have one geometry, named identically to the component.
			// Truly anonymous geometries e.g. passed via worldstate are skipped unless they are labeled
			if (parentName != "" && geomName == parentName) || geomName == "" {
				continue
			}
			if _, ok := validGeoms[geomName]; ok {
				return fmt.Errorf("geometry %s is specified by name more than once", geomName)
			}
			validGeoms[geomName] = true
		}
		return nil
	}

	// Get names of world state obstacles
	if worldState != nil {
		for _, geomsInFrame := range worldState.Obstacles {
			err := addGeomNames("", geomsInFrame)
			if err != nil {
				return err
			}
		}
	}

	// TODO(pl): non-moving frame system geometries are not currently supported for collision avoidance ( RSDK-2129 ) but are included here
	// in anticipation of support and to prevent spurious errors.
	allFsGeoms, err := referenceframe.FrameSystemGeometries(fs, seedMap, logger)
	if err != nil {
		return err
	}
	for frameName, geomsInFrame := range allFsGeoms {
		validGeoms[frameName] = true
		err = addGeomNames(frameName, geomsInFrame)
		if err != nil {
			return err
		}
	}

	// This allows the user to specify an entire component with sub-geometries, e.g. "myUR5arm", and the specification will apply to all
	// sub-pieces, e.g. myUR5arm:upper_arm_link, myUR5arm:base_link, etc. Individual sub-pieces may also be so addressed.
	var allowNameToSubGeoms func(cName string) ([]string, error) // Pre-define to allow recursive call
	allowNameToSubGeoms = func(cName string) ([]string, error) {
		// Check if an entire component is specified
		if geomsInFrame, ok := allFsGeoms[cName]; ok {
			subNames := []string{}
			for _, subGeom := range geomsInFrame.Geometries() {
				subNames = append(subNames, subGeom.Label())
			}
			// If this is an entire component, it likely has an origin frame. Collect any origin geometries as well if so.
			// These will be the geometries that a user specified for this component in their RDK config.
			originGeoms, err := allowNameToSubGeoms(cName + "_origin")
			if err == nil && len(originGeoms) > 0 {
				subNames = append(subNames, originGeoms...)
			}
			return subNames, nil
		}
		// Check if it's a single sub-component
		if validGeoms[cName] {
			return []string{cName}, nil
		}

		// generate the list of available names to return in error message
		availNames := make([]string, 0, len(validGeoms))
		for name := range validGeoms {
			availNames = append(availNames, name)
		}

		return nil, fmt.Errorf("geometry specification allow name %s does not match any known geometries. Available: %v", cName, availNames)
	}

	// Actually create the collision pairings
	for _, collisionSpec := range pbConstraint {
		for _, allowPair := range collisionSpec.GetAllows() {
			allow1 := allowPair.GetFrame1()
			allow2 := allowPair.GetFrame2()
			allowNames1, err := allowNameToSubGeoms(allow1)
			if err != nil {
				return err
			}
			allowNames2, err := allowNameToSubGeoms(allow2)
			if err != nil {
				return err
			}
			for _, allowName1 := range allowNames1 {
				for _, allowName2 := range allowNames2 {
					allowedCollisions = append(allowedCollisions, &Collision{name1: allowName1, name2: allowName2})
				}
			}
		}
	}

	// add collision constraints
	selfCollisionConstraint, err := newSelfCollisionConstraint(frame, seedMap, allowedCollisions, reportDistances)
	if err != nil {
		return err
	}
	obstacleConstraint, err := newObstacleConstraint(frame, fs, worldState, seedMap, allowedCollisions, reportDistances)
	if err != nil {
		return err
	}
	p.AddConstraint(defaultObstacleConstraintName, obstacleConstraint)
	p.AddConstraint(defaultSelfCollisionConstraintName, selfCollisionConstraint)

	return nil
}
