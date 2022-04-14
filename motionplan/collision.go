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

type collisionEntity struct {
	name     string
	geometry spatial.Geometry
}

type collisionCheckFn func(key, test *collisionEntity) (float64, error)
type collisionReportFn func(distances []float64) []int

type CollisionEntities interface {
	count() int
	entityFromIndex(int) *collisionEntity
	indexFromName(string) (int, error)
	collisionCheckFn() collisionCheckFn
	collisionReportFn() collisionReportFn
}

type defaultCollisionEntities struct {
	entities []*collisionEntity
	indices  map[string]int
	checkFn  collisionCheckFn
	reportFn collisionReportFn
}

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

func (gce *defaultCollisionEntities) count() int {
	return len(gce.entities)
}

func (gce *defaultCollisionEntities) entityFromIndex(index int) *collisionEntity {
	return gce.entities[index]
}

func (gce *defaultCollisionEntities) indexFromName(name string) (int, error) {
	if index, ok := gce.indices[name]; ok {
		return index, nil
	}
	return -1, fmt.Errorf("collision entity %q not found", name)
}

func (gce *defaultCollisionEntities) collisionCheckFn() collisionCheckFn {
	return gce.checkFn
}

func (gce *defaultCollisionEntities) collisionReportFn() collisionReportFn {
	return gce.reportFn
}

// exported name because it is required that the key entities in the collision system be of type ObjectCollisionEntities
type ObjectCollisionEntities struct{ *defaultCollisionEntities }

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

type spaceCollisionEntities struct{ *defaultCollisionEntities }

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
			} else {
				return -1, nil
			}
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

type collisionGraph struct {
	key         *ObjectCollisionEntities
	test        CollisionEntities
	adjacencies [][]float64
	triangular  bool
}

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

func (cg *collisionGraph) collisions() []Collision {
	collisions := make([]Collision, 0)
	for i := range cg.adjacencies {
		for _, j := range cg.test.collisionReportFn()(cg.adjacencies[i]) {
			collisions = append(collisions, Collision{cg.key.entityFromIndex(i).name, cg.test.entityFromIndex(j).name, cg.adjacencies[i][j]})
		}
	}
	return collisions
}

type CollisionSystem struct {
	graphs []*collisionGraph
}

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

func NewCollisionSystem(key *ObjectCollisionEntities, optional []CollisionEntities) (*CollisionSystem, error) {
	return NewCollisionSystemFromReference(key, optional, &CollisionSystem{})
}

func (cs *CollisionSystem) Collisions() []Collision {
	collisions := make([]Collision, 0)
	for _, graph := range cs.graphs {
		collisions = append(collisions, graph.collisions()...)
	}
	return collisions
}

func (cs *CollisionSystem) CollisionBetween(keyName, testName string) bool {
	for _, graph := range cs.graphs {
		if graph.collisionBetween(keyName, testName) {
			return true
		}
	}
	return false
}
