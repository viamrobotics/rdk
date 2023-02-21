package motionplan

import (
	"fmt"
	"math"

	"github.com/pkg/errors"

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

// collisionEntity is an object that is used in collision checking and contains a named geometry.
type collisionEntity struct {
	name     string
	geometry spatial.Geometry
}

// collisionEntities is an implementation of CollisionEntities for entities that occupy physical space and should not be intersected
// it is exported because the key CollisionEntities in a CollisionSystem must be of this type.
type collisionEntities struct {
	entities []*collisionEntity
	indices  map[string]int
}

func newCollisionEntities(geometries map[string]spatial.Geometry) (*collisionEntities, error) {
	entities := make([]*collisionEntity, len(geometries))
	indices := make(map[string]int, len(geometries))
	size := 0
	for name, geometry := range geometries {
		if _, ok := indices[name]; ok {
			return nil, fmt.Errorf("error creating CollisionEntities, found geometry with duplicate name: %s", name)
		}
		entities[size] = &collisionEntity{name, geometry}
		indices[name] = size
		size++
	}
	return &collisionEntities{entities, indices}, nil
}

// count returns the number of collisionEntities in a CollisionEntities class.
func (ce *collisionEntities) count() int {
	return len(ce.entities)
}

// entityFromIndex returns the entity in the CollisionEntities class that corresponds to the given index.
func (ce *collisionEntities) entityFromIndex(index int) *collisionEntity {
	return ce.entities[index]
}

// indexFromName returns the index in the CollisionEntities class that corresponds to the given name.
// a negative return value corresponds to an error.
func (ce *collisionEntities) indexFromName(name string) int {
	if index, ok := ce.indices[name]; ok {
		return index
	}
	return -1
}

// spaceCollisionEntities is an implementation of CollisionEntities for entities that do not occupy physical space but
// represent an area in which other entities should be encompassed by.
// type spaceCollisionEntities struct{ *ObjectCollisionEntities }

// // NewSpaceCollisionEntities is a constructor for spaceCollisionEntities.
// func NewSpaceCollisionEntities(geometries map[string]spatial.Geometry) (CollisionEntities, error) {
// 	entities, err := NewObjectCollisionEntities(geometries)
// 	return spaceCollisionEntities{entities}, err
// }

// func (sce spaceCollisionEntities) checkCollision(key, test *collisionEntity, reportDistances bool) (float64, error) {
// 	encompassed, err := key.geometry.EncompassedBy(test.geometry)
// 	if err != nil {
// 		return math.NaN(), err
// 	}
// 	// TODO(rb): EncompassedBy should also report distance required to resolve the collision
// 	if !encompassed {
// 		return 1, nil
// 	}
// 	return -1, nil
// }

// func (sce spaceCollisionEntities) reportCollisions(distances []float64) []int {
// 	collisionIndices := make([]int, 0)
// 	for i := range distances {
// 		if distances[i] >= spatial.CollisionBuffer {
// 			collisionIndices = append(collisionIndices, i)
// 		} else {
// 			return []int{}
// 		}
// 	}
// 	return collisionIndices
// }

// TODO: comments
type entityGraph struct {
	x, y *collisionEntities

	distances [][]float64

	// triangular is a bool that describes if the adjacencies matrix is triangular, which will be the case when x ==y
	triangular bool

	reportDistances bool
}

func newEntityGraph(x, y *collisionEntities, reportDistances bool) *entityGraph {
	return &entityGraph{
		x:               x,
		y:               y,
		distances:       make([][]float64, x.count()),
		reportDistances: reportDistances,
		triangular:      x == y,
	}
}

func (cg *entityGraph) getIndices(xName, yName string) (int, int, bool) {
	i := cg.x.indexFromName(xName)
	j := cg.y.indexFromName(yName)
	return i, j, i >= 0 && j >= 0
}

type collisionGraph struct {
	entityGraph
}

// newCollisionGraph instantiates a collisionGraph object and checks for collisions between the key and test sets of CollisionEntities
// collisions that are reported in the reference CollisionSystem argument will be ignore and not stored as edges in the graph.
func newCollisionGraph(x, y *collisionEntities, reference *collisionGraph, reportDistances bool) (cg *collisionGraph, err error) {
	cg = &collisionGraph{*newEntityGraph(x, y, reportDistances)}
	for i := range cg.distances {
		cg.distances[i] = make([]float64, y.count())
		xi := x.entityFromIndex(i)
		startIndex := 0
		if cg.triangular {
			startIndex = i + 1
			for j := 0; j < startIndex; j++ {
				cg.distances[i][j] = math.NaN()
			}
		}
		for j := startIndex; j < len(cg.distances[i]); j++ {
			yj := y.entityFromIndex(j)
			if reference != nil && reference.collisionBetween(xi.name, yj.name) {
				cg.distances[i][j] = math.NaN() // represent previously seen collisions as NaNs
			} else {
				cg.distances[i][j], err = cg.checkCollision(xi, yj)
				if err != nil {
					return nil, err
				}
				if !reportDistances && cg.distances[i][j] <= spatial.CollisionBuffer {
					return cg, nil
				}
			}
		}
	}
	return cg, nil
}

func (cg *collisionGraph) checkCollision(x, y *collisionEntity) (float64, error) {
	if cg.reportDistances {
		return x.geometry.DistanceFrom(y.geometry)
	}
	col, err := x.geometry.CollidesWith(y.geometry)
	if col {
		return -1, err
	}
	return 1, err
}

// collisionBetween returns a bool describing if the collisionGraph has an edge between the two entities that are specified by name.
func (cg *collisionGraph) collisionBetween(keyName, testName string) bool {
	if i, j, ok := cg.getIndices(keyName, testName); ok {
		if cg.triangular && i > j {
			i, j = j, i
		}
		if cg.distances[i][j] <= spatial.CollisionBuffer {
			return true
		}
	}
	return false
}

// collisions returns a list of all the Collisions as reported by test CollisionEntities' collisionReportFn.
func (cg *collisionGraph) collisions() []Collision {
	var collisions []Collision
	for i := range cg.distances {
		for j := range cg.distances[i] {
			if cg.distances[i][j] <= spatial.CollisionBuffer {
				collisions = append(collisions, Collision{cg.x.entityFromIndex(i).name, cg.y.entityFromIndex(j).name, cg.distances[i][j]})
			}
		}
	}
	return collisions
}

// ignoreCollision finds the specified collision and marks it as something never to check for or report
func (cg *collisionGraph) ignoreCollision(toIgnore *Collision) (err error) {
	i, j, ok := cg.getIndices(toIgnore.name1, toIgnore.name2)
	if !ok {
		i, j, ok = cg.getIndices(toIgnore.name2, toIgnore.name1)
	}
	if !ok {
		return errors.Errorf("cannot add collision specification between entities with names: %s, %s", toIgnore.name1, toIgnore.name2)
	}
	if cg.triangular && i > j {
		i, j = j, i
	}
	cg.distances[i][j] = math.NaN()
	return nil
}

type collisionSystem struct {
	graphs []*collisionGraph
}

func newCollisionSystem(robot, obstacle *collisionEntities, reference *collisionSystem, reportDistances bool) (*collisionSystem, error) {
	var selfCGreference, obstacleCGreference *collisionGraph
	if reference != nil {
		selfCGreference = reference.graphs[0]
		obstacleCGreference = reference.graphs[1]
	}
	selfCG, err := newCollisionGraph(robot, robot, selfCGreference, reportDistances)
	if err != nil {
		return nil, err
	}
	obstacleCG, err := newCollisionGraph(robot, obstacle, obstacleCGreference, reportDistances)
	if err != nil {
		return nil, err
	}
	return &collisionSystem{[]*collisionGraph{selfCG, obstacleCG}}, err
}

// collisions returns a list of all the reported collisions in the collisionSystem.
func (cs *collisionSystem) collisions() (collisions []Collision) {
	for _, graph := range cs.graphs {
		collisions = append(collisions, graph.collisions()...)
	}
	return collisions
}

// addCollisionSpecification takes a Collision as an argument and sets its underlying collisionGraphs to ignore it.
// If no collisionGraphs have a corresponding entity pair, then an error will be thrown
func (cs *collisionSystem) addCollisionSpecification(specification *Collision) (err error) {
	for _, graph := range cs.graphs {
		if err = graph.ignoreCollision(specification); err == nil {
			return nil
		}
	}
	return err
}
