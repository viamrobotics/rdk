package motionplan

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	pb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

const unnamedCollisionGeometryPrefix = "unnamedCollisionGeometry_"

// Collision is a pair of strings corresponding to names of Geometry objects in collision, and a penetrationDepth describing the Euclidean
// distance a Geometry would have to be moved to resolve the Collision.
type Collision struct {
	name1, name2     string
	penetrationDepth float64
}

// collisionsAlmostEqual compares two Collisions and returns if they are almost equal.
func collisionsAlmostEqual(c1, c2 Collision) bool {
	return ((c1.name1 == c2.name1 && c1.name2 == c2.name2) || (c1.name1 == c2.name2 && c1.name2 == c2.name1)) &&
		utils.Float64AlmostEqual(c1.penetrationDepth, c2.penetrationDepth, 0.1)
}

// collisionListsAlmostEqual compares two lists of Collisions and returns if they are almost equal.
func collisionListsAlmostEqual(cs1, cs2 []Collision) bool {
	if len(cs1) != len(cs2) {
		return false
	}

	// loop through list 1 and match with elements in list 2, mark on list of used indexes
	used := make([]bool, len(cs1))
	for _, c1 := range cs1 {
		for i, c2 := range cs2 {
			if collisionsAlmostEqual(c1, c2) {
				used[i] = true
				break
			}
		}
	}

	// loop through list of used indexes
	for _, c := range used {
		if !c {
			return false
		}
	}
	return true
}

func collisionSpecificationsFromProto(
	pbConstraint []*pb.CollisionSpecification,
	frameSystemGeometries map[string]*referenceframe.GeometriesInFrame,
	worldState *referenceframe.WorldState,
) (allowedCollisions []*Collision, err error) {
	// List of all names which may be specified for collision ignoring.
	// Can seed this map with worldState names which will never have duplicates
	validGeoms := worldState.ObstacleNames()

	// Get names of all geometries in frame system
	for frameName, geomsInFrame := range frameSystemGeometries {
		if _, ok := validGeoms[frameName]; ok {
			return nil, referenceframe.NewDuplicateGeometryNameError(frameName)
		}
		validGeoms[frameName] = true
		for _, geom := range geomsInFrame.Geometries() {
			geomName := geom.Label()

			// Ensure we're not double-adding components which only have one geometry, named identically to the component.
			if (frameName != "" && geomName == frameName) || geomName == "" {
				continue
			}
			if _, ok := validGeoms[geomName]; ok {
				return nil, referenceframe.NewDuplicateGeometryNameError(geomName)
			}
			validGeoms[geomName] = true
		}
	}

	// This allows the user to specify an entire component with sub-geometries, e.g. "myUR5arm", and the specification will apply to all
	// sub-pieces, e.g. myUR5arm:upper_arm_link, myUR5arm:base_link, etc. Individual sub-pieces may also be so addressed.
	var allowNameToSubGeoms func(cName string) ([]string, error) // Pre-define to allow recursive call
	allowNameToSubGeoms = func(cName string) ([]string, error) {
		// Check if an entire component is specified
		if geomsInFrame, ok := frameSystemGeometries[cName]; ok {
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

	// Create the structures that specify the allowed collisions
	for _, collisionSpec := range pbConstraint {
		for _, allowPair := range collisionSpec.GetAllows() {
			allow1 := allowPair.GetFrame1()
			allow2 := allowPair.GetFrame2()
			allowNames1, err := allowNameToSubGeoms(allow1)
			if err != nil {
				return nil, err
			}
			allowNames2, err := allowNameToSubGeoms(allow2)
			if err != nil {
				return nil, err
			}
			for _, allowName1 := range allowNames1 {
				for _, allowName2 := range allowNames2 {
					allowedCollisions = append(allowedCollisions, &Collision{name1: allowName1, name2: allowName2})
				}
			}
		}
	}
	return allowedCollisions, nil
}

// geometryGraph is a struct that stores distance relationships between sets of geometries.
type geometryGraph struct {
	// x and y are the two sets of geometries, each of which will be compared to the geometries in the other set
	x, y map[string]spatial.Geometry

	// distances is the data structure to store the distance relationships between two named geometries
	// can be acessed as distances[name1][name2] to get the distance between name1 and name2
	distances map[string]map[string]float64
}

// newGeometryGraph instantiates a geometryGraph with the x and y geometry sets.
func newGeometryGraph(x, y map[string]spatial.Geometry) geometryGraph {
	distances := make(map[string]map[string]float64)
	for name := range x {
		distances[name] = make(map[string]float64)
	}
	return geometryGraph{
		x:         x,
		y:         y,
		distances: distances,
	}
}

// setDistance takes two given geometry names and sets their distance in the distances table exactly once
// since the relationship between the geometries is bidirectional, the order that the names are passed in is not important.
func (gg *geometryGraph) setDistance(xName, yName string, distance float64) {
	if _, ok := gg.distances[yName][xName]; ok {
		gg.distances[yName][xName] = distance
	} else {
		gg.distances[xName][yName] = distance
	}
}

// getDistance finds the distance between the given geometry names by referencing the distances table
// a secondary return value of type bool is also returned, indicating if the distance was found in the table
// if the distance between the geometry names was never set, the return value will be (NaN, false).
func (cg *collisionGraph) getDistance(name1, name2 string) (float64, bool) {
	if distance, ok := cg.distances[name1][name2]; ok {
		return distance, true
	}
	if distance, ok := cg.distances[name2][name1]; ok {
		return distance, true
	}
	return math.NaN(), false
}

// collisionGraph utilizes the geometryGraph structure to make collision checks between geometries
// a collision is defined as a negative penetration depth and is stored in the distances table.
type collisionGraph struct {
	geometryGraph

	// reportDistances is a bool that determines how the collisionGraph will report collisions
	//    - true:  all distances will be determined and numerically reported
	//    - false: collisions will be reported as bools, not numerically. Upon finding a collision, will exit early
	reportDistances bool
}

// newCollisionGraph instantiates a collisionGraph object and checks for collisions between the x and y sets of geometries
// collisions that are reported in the reference CollisionSystem argument will be ignored and not stored as edges in the graph.
// if the set y is nil, the graph will be instantiated with y = x.
func newCollisionGraph(x, y []spatial.Geometry, reference *collisionGraph, reportDistances bool) (cg *collisionGraph, err error) {
	if y == nil {
		y = x
	}
	xMap, err := createUniqueCollisionMap(x)
	if err != nil {
		return nil, err
	}
	yMap, err := createUniqueCollisionMap(y)
	if err != nil {
		return nil, err
	}

	cg = &collisionGraph{
		geometryGraph:   newGeometryGraph(xMap, yMap),
		reportDistances: reportDistances,
	}

	var distance float64
	for xName, xGeometry := range cg.x {
		for yName, yGeometry := range cg.y {
			if _, ok := cg.getDistance(xName, yName); ok || xGeometry == yGeometry {
				// geometry pair already has distance information associated with it, or is comparing with itself - skip to next pair
				continue
			}
			if reference != nil && reference.collisionBetween(xName, yName) {
				// represent previously seen collisions as NaNs
				// per IEE standards, any comparison with NaN will return false, so these will never be considered collisions
				distance = math.NaN()
			} else if distance, err = cg.checkCollision(xGeometry, yGeometry); err != nil {
				return nil, err
			}
			cg.setDistance(xName, yName, distance)
			if !reportDistances && distance <= spatial.CollisionBuffer {
				// collision found, can return early
				return cg, nil
			}
		}
	}
	return cg, nil
}

// checkCollision takes a pair of geometries and returns the distance between them.
// If this number is less than the CollisionBuffer they can be considered to be in collision.
func (cg *collisionGraph) checkCollision(x, y spatial.Geometry) (float64, error) {
	// x is the robot geometries and therefore must use the primitives from spatialmath
	// y is a geometry type that could potentially live outside spatialmath and therefore knows more so we defer to it for collisions
	if cg.reportDistances {
		dist, err := x.DistanceFrom(y)
		if err != nil {
			return y.DistanceFrom(x)
		}
		return dist, nil
	}
	col, err := x.CollidesWith(y)
	if err != nil {
		col, err = y.CollidesWith(x)
		if err != nil {
			return math.Inf(-1), err
		}
	}
	if col {
		return math.Inf(-1), err
	}
	return math.Inf(1), err
}

// collisionBetween returns a bool describing if the collisionGraph has a collision between the two entities that are specified by name.
func (cg *collisionGraph) collisionBetween(name1, name2 string) bool {
	if distance, ok := cg.getDistance(name1, name2); ok {
		return distance <= spatial.CollisionBuffer
	}
	return false
}

// collisions returns a list of all the collisions present in the collisionGraph.
func (cg *collisionGraph) collisions() []Collision {
	var collisions []Collision
	for xName, row := range cg.distances {
		for yName, distance := range row {
			if distance <= spatial.CollisionBuffer {
				collisions = append(collisions, Collision{xName, yName, distance})
				if !cg.reportDistances {
					// collision found, can return early
					return collisions
				}
			}
		}
	}
	return collisions
}

// addCollisionSpecification marks the two objects specified as colliding.
func (cg *collisionGraph) addCollisionSpecification(specification *Collision) {
	cg.setDistance(specification.name1, specification.name2, math.Inf(-1))
}

func createUniqueCollisionMap(geoms []spatial.Geometry) (map[string]spatial.Geometry, error) {
	unnamedCnt := 0
	geomMap := map[string]spatial.Geometry{}

	for _, geom := range geoms {
		label := geom.Label()
		if label == "" {
			label = unnamedCollisionGeometryPrefix + strconv.Itoa(unnamedCnt)
			unnamedCnt++
		}
		if _, present := geomMap[label]; present {
			return nil, referenceframe.NewDuplicateGeometryNameError(label)
		}
		geomMap[label] = geom
	}
	return geomMap, nil
}

// CheckPlan checks if obstacles intersect the trajectory of the frame following the plan.
func CheckPlan(
	frame referenceframe.Frame,
	plan [][]referenceframe.Input,
	obstacles []*referenceframe.GeometriesInFrame,
	fs referenceframe.FrameSystem,
) (bool, error) {
	// ensure that we can actually perform the check
	if len(frame.DoF()) != len(plan[0]) {
		return false, errors.New("frame DOF length must match inputs length")
	}
	// check if we are working with ptgs
	ptgProv, ok := frame.(tpspace.PTGProvider)
	if ok {
		return checkPtgPlan(frame, plan, obstacles, fs, ptgProv.PTGs())
	}

	if len(plan) < 2 {
		return false, errors.New("plan must have at least two elements")
	}

	// construct planner with collision contraints
	opt := newBasicPlannerOptions(frame)
	sf, err := newSolverFrame(fs, frame.Name(), referenceframe.World, nil)
	if err != nil {
		return false, err
	}
	wrdlst, err := referenceframe.NewWorldState(obstacles, nil)
	if err != nil {
		return false, err
	}
	collisionConstraints, err := createAllCollisionConstraints(sf, fs, wrdlst, referenceframe.StartPositions(fs), nil)
	if err != nil {
		return false, err
	}
	for name, constraint := range collisionConstraints {
		opt.AddStateConstraint(name, constraint)
	}

	// go through plan and check that we can move from plan[i] to plan[i+1]
	for i := 0; i < len(plan)-1; i++ {
		if isValid, fault := opt.CheckSegmentAndStateValidity(
			&Segment{
				StartConfiguration: plan[i],
				EndConfiguration:   plan[i+1],
				Frame:              frame,
			},
			opt.Resolution,
		); !isValid {
			return false, fmt.Errorf("found collsion in segment:%v", fault)
		}
	}
	return true, nil
}

func checkPtgPlan(
	frame referenceframe.Frame,
	plan [][]referenceframe.Input,
	obstacles []*referenceframe.GeometriesInFrame,
	fs referenceframe.FrameSystem,
	ptgs []tpspace.PTG,
) (bool, error) {
	// ensure obstacles are in world frame
	wrdlst, err := referenceframe.NewWorldState(obstacles, nil)
	if err != nil {
		return false, err
	}
	transformedObstacles, err := wrdlst.ObstaclesInWorldFrame(fs, referenceframe.StartPositions(fs))
	if err != nil {
		return false, err
	}

	lastRecordedPose := spatial.NewZeroPose()
	latestPose := spatial.NewZeroPose()

	// inputs are:
	// [0] index of PTG to use
	// [1] index of the trajectory within that PTG
	// [2] distance to travel along that trajectory.
	for _, inputs := range plan {
		// find the relevant ptg with the 0th value of inputs
		ptg := ptgs[int(math.Round(inputs[0].Value))]
		// find relevant trajectories with the 1st value of inputs
		for _, traj := range ptg.Trajectory(uint(inputs[1].Value)) {
			// stop checking the trajectory once we have traveled its required distance
			if traj.Dist >= inputs[2].Value {
				lastRecordedPose = spatial.Compose(lastRecordedPose, latestPose)
				break
			}
			// transform the frame's geometries by inputs
			newInputs := []referenceframe.Input{
				inputs[0], inputs[1], {Value: traj.Dist},
			}
			baseGIFS, err := frame.Geometries(newInputs)
			if err != nil {
				return false, err
			}
			for _, baseGeom := range baseGIFS.Geometries() {
				baseGeom = baseGeom.Transform(lastRecordedPose)
				for _, obstacle := range transformedObstacles.Geometries() {
					if collides, err := baseGeom.CollidesWith(obstacle); err != nil {
						return false, err
					} else if collides {
						return false, fmt.Errorf("path is not valid, found collision with %v", obstacle)
					}
				}
			}
			latestPose = traj.Pose
		}
	}
	return true, nil
}
