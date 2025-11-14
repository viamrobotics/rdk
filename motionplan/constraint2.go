package motionplan

import (
	"fmt"

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

// short descriptions of constraints used in error messages.
const (
	linearConstraintDescription      = "linear constraint"
	orientationConstraintDescription = "orientation constraint"
	planarConstraintDescription      = "planar constraint"

	// various collision constraints that have different names in order to be unique keys in maps of constraints that are created.
	boundingRegionConstraintDescription = "bounding region constraint"
	obstacleConstraintDescription       = "obstacle constraint"
	selfCollisionConstraintDescription  = "self-collision constraint"
	robotCollisionConstraintDescription = "robot constraint" // collision between a moving robot component and one that is stationary

	defaultCollisionBufferMM = 1e-8
)

// StateFSConstraint tests whether a given robot configuration is valid
// If the returned error is nil, the constraint is satisfied and the state is valid.
type StateFSConstraint func(*StateFS) error

// CreateAllCollisionConstraints -.
func CreateAllCollisionConstraints(
	movingRobotGeometries, staticRobotGeometries, worldGeometries []spatial.Geometry,
	allowedCollisions []*Collision,
	collisionBufferMM float64,
) (map[string]StateFSConstraint, error) {
	constraintFSMap := map[string]StateFSConstraint{}

	if len(worldGeometries) > 0 {
		// create constraint to keep moving geometries from hitting world state obstacles
		obstacleConstraintFS, err := NewCollisionConstraintFS(
			movingRobotGeometries,
			worldGeometries,
			allowedCollisions,
			false,
			collisionBufferMM,
		)
		if err != nil {
			return nil, err
		}
		constraintFSMap[obstacleConstraintDescription] = obstacleConstraintFS
	}

	if len(staticRobotGeometries) > 0 {
		robotConstraintFS, err := NewCollisionConstraintFS(
			movingRobotGeometries,
			staticRobotGeometries,
			allowedCollisions,
			false,
			collisionBufferMM)
		if err != nil {
			return nil, err
		}
		constraintFSMap[robotCollisionConstraintDescription] = robotConstraintFS
	}

	// create constraint to keep moving geometries from hitting themselves
	if len(movingRobotGeometries) > 1 {
		selfCollisionConstraintFS, err := NewCollisionConstraintFS(
			movingRobotGeometries, nil, allowedCollisions, false, collisionBufferMM)
		if err != nil {
			return nil, err
		}
		constraintFSMap[selfCollisionConstraintDescription] = selfCollisionConstraintFS
	}
	return constraintFSMap, nil
}

func setupZeroCG(
	moving, static []spatial.Geometry,
	collisionSpecifications []*Collision,
	reportDistances bool,
	collisionBufferMM float64,
) (*collisionGraph, error) {
	// create the reference collisionGraph
	zeroCG, err := newCollisionGraph(moving, static, nil, reportDistances, collisionBufferMM)
	if err != nil {
		return nil, err
	}
	for _, specification := range collisionSpecifications {
		zeroCG.addCollisionSpecification(specification)
	}
	return zeroCG, nil
}

// NewCollisionConstraintFS is the most general method to create a collision constraint for a frame system,
// which will be violated if geometries constituting the given frame ever come into collision with obstacle geometries
// outside of the collisions present for the observationInput. Collisions specified as collisionSpecifications will also be ignored.
// If reportDistances is false, this check will be done as fast as possible, if true maximum information will be available for debugging.
func NewCollisionConstraintFS(
	moving, static []spatial.Geometry,
	collisionSpecifications []*Collision,
	reportDistances bool,
	collisionBufferMM float64,
) (StateFSConstraint, error) {
	zeroCG, err := setupZeroCG(moving, static, collisionSpecifications, true, collisionBufferMM)
	if err != nil {
		return nil, err
	}

	movingMap := map[string]spatial.Geometry{}
	for _, geom := range moving {
		movingMap[geom.Label()] = geom
	}

	// create constraint from reference collision graph
	constraint := func(state *StateFS) error {
		// Use FrameSystemGeometries to get all geometries in the frame system
		internalGeometries, err := referenceframe.FrameSystemGeometriesLinearInputs(state.FS, state.Configuration)
		if err != nil {
			return err
		}

		// We only want to compare *moving* geometries, so we filter what we get from the framesystem against what we were passed.
		var internalGeoms []spatial.Geometry
		for _, geosInFrame := range internalGeometries {
			if len(geosInFrame.Geometries()) > 0 {
				if _, ok := movingMap[geosInFrame.Geometries()[0].Label()]; ok {
					internalGeoms = append(internalGeoms, geosInFrame.Geometries()...)
				}
			}
		}

		return collisionCheckFinish(internalGeoms, static, zeroCG, reportDistances, collisionBufferMM)
	}
	return constraint, nil
}

func collisionCheckFinish(internalGeoms, static []spatial.Geometry, zeroCG *collisionGraph,
	reportDistances bool, collisionBufferMM float64,
) error {
	cg, err := newCollisionGraph(internalGeoms, static, zeroCG, reportDistances, collisionBufferMM)
	if err != nil {
		return err
	}
	cs := cg.collisions(collisionBufferMM)
	if len(cs) != 0 {
		// we could choose to amalgamate all the collisions into one error but its probably saner not to and choose just the first
		return fmt.Errorf("violation between %s and %s geometries (total collisions: %d)", cs[0].name1, cs[0].name2, len(cs))
	}
	return nil
}
