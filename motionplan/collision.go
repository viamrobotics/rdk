package motionplan

import (
	"math"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

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

// TODO: comments
type geometryGraph struct {
	x, y map[string]spatial.Geometry

	distances map[string]map[string]float64

	reportDistances bool
}

func newGeometryGraph(x, y map[string]spatial.Geometry, reportDistances bool) geometryGraph {
	distances := make(map[string]map[string]float64)
	for name := range x {
		distances[name] = make(map[string]float64)
	}
	return geometryGraph{
		x:               x,
		y:               y,
		distances:       distances,
		reportDistances: reportDistances,
	}
}

type collisionGraph struct {
	geometryGraph
}

// newCollisionGraph instantiates a collisionGraph object and checks for collisions between the key and test sets of CollisionEntities
// collisions that are reported in the reference CollisionSystem argument will be ignore and not stored as edges in the graph.
func newCollisionGraph(x, y map[string]spatial.Geometry, reference *collisionGraph, reportDistances bool) (cg *collisionGraph, err error) {
	if y == nil {
		y = x
	}
	cg = &collisionGraph{newGeometryGraph(x, y, reportDistances)}

	var distance float64
	for xName, xGeometry := range cg.x {
		for yName, yGeometry := range cg.y {
			if _, ok := cg.distances[yName][xName]; ok || xGeometry == yGeometry {
				continue
			}
			if reference != nil && reference.collisionBetween(xName, yName) {
				distance = math.NaN() // represent previously seen collisions as NaNs
			} else if distance, err = cg.checkCollision(xGeometry, yGeometry); err != nil {
				return nil, err
			}
			cg.distances[xName][yName] = distance
			if !reportDistances && distance <= spatial.CollisionBuffer {
				return cg, nil
			}
		}
	}
	return cg, nil
}

func (cg *collisionGraph) checkCollision(x, y spatial.Geometry) (float64, error) {
	if cg.reportDistances {
		return x.DistanceFrom(y)
	}
	col, err := x.CollidesWith(y)
	if col {
		return -1, err
	}
	return 1, err
}

// collisionBetween returns a bool describing if the collisionGraph has an edge between the two entities that are specified by name.
func (cg *collisionGraph) collisionBetween(name1, name2 string) bool {
	distance, ok := cg.distances[name1][name2]
	if !ok {
		if distance, ok = cg.distances[name2][name1]; !ok {
			return false
		}
	}
	return distance <= spatial.CollisionBuffer
}

// collisions returns a list of all the Collisions as reported by test CollisionEntities' collisionReportFn.
func (cg *collisionGraph) collisions() []Collision {
	var collisions []Collision
	for xName, row := range cg.distances {
		for yName, distance := range row {
			if distance <= spatial.CollisionBuffer {
				collisions = append(collisions, Collision{xName, yName, distance})
				if !cg.reportDistances {
					return collisions
				}
			}
		}
	}
	return collisions
}

// ignoreCollision finds the specified collision and marks it as something never to check for or report
func (cg *collisionGraph) addCollisionSpecification(specification *Collision) {
	if _, ok := cg.distances[specification.name1][specification.name2]; ok {
		cg.distances[specification.name1][specification.name2] = math.NaN()
	} else if _, ok := cg.distances[specification.name2][specification.name1]; ok {
		cg.distances[specification.name2][specification.name1] = math.NaN()
	}
}
