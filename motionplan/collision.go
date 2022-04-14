package motionplan

import (
	"fmt"
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

// collisionEntity is an object that is used in collision checking and contains a named geometry.
type collisionEntity struct {
	name     string
	geometry spatial.Geometry
}

type (
	// type for function that performs a collision check between a key entity and and a test entity.
	collisionCheckFn func(key, test *collisionEntity) (float64, error)

	// collisionReportFn is a type for function that takes in a list of distances reported by a collision check between a key entity
	// and a set of test entities and returns a list of ints corresponding to elemements in the array that should be treated as collisions.
	collisionReportFn func(distances []float64) []int
)

// CollisionEntities defines an interface for a set of collisionEntities that can be treated as a single batch.
type CollisionEntities interface {
	count() int
	entityFromIndex(int) *collisionEntity
	indexFromName(string) (int, error)
	collisionCheckFn() collisionCheckFn
	collisionReportFn() collisionReportFn
}

// defaultCollisionEntities defines an implementation for CollisionEntities that other implementations can inherit from.
type defaultCollisionEntities struct {
	entities []*collisionEntity
	indices  map[string]int
	checkFn  collisionCheckFn
	reportFn collisionReportFn
}

// newCollisionEntities is a constructor for a defaultCollisionEntities and takes in geometries, and 2 functions defining their treatment
//     - checkFn defines how the entities should be checked for collision
//     - reportFn defines the collision entities will report their collisions
func newCollisionEntities(
	geometries map[string]spatial.Geometry,
	checkFn collisionCheckFn,
	reportFn collisionReportFn,
) (*defaultCollisionEntities, error) {
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
	return &defaultCollisionEntities{entities, indices, checkFn, reportFn}, nil
}

// count returns the number of collisionEntities in a CollisionEntities class.
func (gce *defaultCollisionEntities) count() int {
	return len(gce.entities)
}

// entityFromIndex returns the entity in the CollisionEntities class that corresponds to the given index.
func (gce *defaultCollisionEntities) entityFromIndex(index int) *collisionEntity {
	return gce.entities[index]
}

// indexFromName returns the index in the CollisionEntities class that corresponds to the given name.
func (gce *defaultCollisionEntities) indexFromName(name string) (int, error) {
	if index, ok := gce.indices[name]; ok {
		return index, nil
	}
	return -1, fmt.Errorf("collision entity %q not found", name)
}

// collisionCheckFn returns the collisionCheckFn associated with the CollisionEntities class.
func (gce *defaultCollisionEntities) collisionCheckFn() collisionCheckFn {
	return gce.checkFn
}

// collisionReportFn returns the collisionReportFn associated with the CollisionEntities class.
func (gce *defaultCollisionEntities) collisionReportFn() collisionReportFn {
	return gce.reportFn
}

// ObjectCollisionEntities is an implementation of CollisionEntities for entities that occupy physical space and should not be intersected
// it is exported because the key CollisionEntities in a CollisionSystem must be of this type.
type ObjectCollisionEntities struct{ *defaultCollisionEntities }

// NewObjectCollisionEntities is a constructor for ObjectCollisionEntities, an exported implementation of CollisionEntities.
func NewObjectCollisionEntities(geometries map[string]spatial.Geometry) (*ObjectCollisionEntities, error) {
	entities, err := newCollisionEntities(
		geometries,
		func(key, test *collisionEntity) (float64, error) {
			distance, err := key.geometry.DistanceFrom(test.geometry)
			return -distance, err // multiply distance by -1 so that weights of edges are positive
		},
		func(distances []float64) []int {
			collisionIndices := make([]int, 0)
			for i := range distances {
				if distances[i] >= 0 {
					collisionIndices = append(collisionIndices, i)
				}
			}
			return collisionIndices
		},
	)
	return &ObjectCollisionEntities{entities}, err
}

// spaceCollisionEntities is an implementation of CollisionEntities for entities that do not occupy physical space but
// represent an area in which other entities should be encompassed by.
type spaceCollisionEntities struct{ *defaultCollisionEntities }

// NewSpaceCollisionEntities is a constructor for spaceCollisionEntities.
func NewSpaceCollisionEntities(geometries map[string]spatial.Geometry) (CollisionEntities, error) {
	entities, err := newCollisionEntities(
		geometries,
		func(key, test *collisionEntity) (float64, error) {
			encompassed, err := key.geometry.EncompassedBy(test.geometry)
			if err != nil {
				return math.NaN(), err
			}
			// TODO(rb): EncompassedBy should also report distance required to resolve the collision
			if !encompassed {
				return 1, nil
			}
			return -1, nil
		},
		func(distances []float64) []int {
			collisionIndices := make([]int, 0)
			for i := range distances {
				if distances[i] >= 0 {
					collisionIndices = append(collisionIndices, i)
				} else {
					return []int{}
				}
			}
			return collisionIndices
		},
	)
	return &spaceCollisionEntities{entities}, err
}

// collisionGraph is an implementation of an undirected graph used to track collisions between two set of CollisionEntities.
type collisionGraph struct {
	// key CollisionEntities
	key *ObjectCollisionEntities

	// test CollisionEntities are the set of CollisionEntities from which the collisionGraph takes its
	// collisionCheckFn and collisionReportFn functions to check and report collisions between the
	// test CollisionEntities and key Collision Entities
	test CollisionEntities

	// adjacencies is 2D array encoding edges between collisionEntiies in the collisionGraph.
	// if adjacencies[i][j] >= 0 this corresponds to an edge between the entities at key[i] and test[j]
	adjacencies [][]float64

	// triangular is a bool that describes if the adjacencies matrix is triangular, which will be the case when key == test
	triangular bool
}

// newCollisionGraph instantiates a collisionGraph object and checks for collisions between the key and test sets of CollisionEntities
// collisions that are reported in the reference CollisionSystem argument will be ignore and not stored as edges in the graph.
func newCollisionGraph(key *ObjectCollisionEntities, test CollisionEntities, reference *CollisionSystem) (*collisionGraph, error) {
	var err error
	cg := &collisionGraph{key: key, test: test, adjacencies: make([][]float64, key.count()), triangular: key == test}
	for i := range cg.adjacencies {
		cg.adjacencies[i] = make([]float64, test.count())
		keyi := key.entityFromIndex(i)
		startIndex := 0
		if cg.triangular {
			startIndex = i + 1
			for j := 0; j < startIndex; j++ {
				cg.adjacencies[i][j] = math.NaN()
			}
		}
		for j := startIndex; j < len(cg.adjacencies[i]); j++ {
			testj := test.entityFromIndex(j)
			if reference.CollisionBetween(keyi.name, testj.name) {
				cg.adjacencies[i][j] = math.NaN() // represent previously seen collisions as NaNs
			} else {
				cg.adjacencies[i][j], err = test.collisionCheckFn()(keyi, testj)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return cg, nil
}

// collisionBetween returns a bool describing if the collisionGraph has an edge between the two entities that are specified by name.
func (cg *collisionGraph) collisionBetween(keyName, testName string) bool {
	i, iOk := cg.key.indexFromName(keyName)
	j, jOk := cg.test.indexFromName(testName)
	if cg.triangular && i > j {
		i, j = j, i
	}
	if iOk == nil && jOk == nil && cg.adjacencies[i][j] >= 0 {
		return true
	}
	return false
}

// collisions returns a list of all the Collisions as reported by test CollisionEntities' collisionReportFn.
func (cg *collisionGraph) collisions() []Collision {
	collisions := make([]Collision, 0)
	for i := range cg.adjacencies {
		for _, j := range cg.test.collisionReportFn()(cg.adjacencies[i]) {
			collisions = append(collisions, Collision{cg.key.entityFromIndex(i).name, cg.test.entityFromIndex(j).name, cg.adjacencies[i][j]})
		}
	}
	return collisions
}

// CollisionSystem is an object that checks for and records collisions between CollisionEntities.
type CollisionSystem struct {
	graphs []*collisionGraph
}

// NewCollisionSystemFromReference creates a new collision system that checks for collisions
// between the entities in the key CollisionEntities and the entities in each of the optional CollisionEntities
// a reference CollisionSystem can also be specified, and edges between entities that exist in this reference system will
// not be duplicated in the newly constructed system.
func NewCollisionSystemFromReference(
	key *ObjectCollisionEntities,
	optional []CollisionEntities,
	reference *CollisionSystem,
) (*CollisionSystem, error) {
	var err error
	cs := &CollisionSystem{make([]*collisionGraph, len(optional)+1)}
	cs.graphs[0], err = newCollisionGraph(key, key, reference)
	if err != nil {
		return nil, err
	}
	for i := range optional {
		cs.graphs[i+1], err = newCollisionGraph(key, optional[i], reference)
		if err != nil {
			return nil, err
		}
	}
	return cs, nil
}

// NewCollisionSystem creates a new collision system that checks for collisions
// between the entities in the key CollisionEntities and the entities in each of the optional CollisionEntities.
func NewCollisionSystem(key *ObjectCollisionEntities, optional []CollisionEntities) (*CollisionSystem, error) {
	return NewCollisionSystemFromReference(key, optional, &CollisionSystem{})
}

// Collisions returns a list of all the reported collisions in the CollisionSystem.
func (cs *CollisionSystem) Collisions() []Collision {
	collisions := make([]Collision, 0)
	for _, graph := range cs.graphs {
		collisions = append(collisions, graph.collisions()...)
	}
	return collisions
}

// CollisionBetween returns a bool describing if a collision between the two named entities was reported in the CollisionSystem.
func (cs *CollisionSystem) CollisionBetween(keyName, testName string) bool {
	for _, graph := range cs.graphs {
		if graph.collisionBetween(keyName, testName) {
			return true
		}
	}
	return false
}
