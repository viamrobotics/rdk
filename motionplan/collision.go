package motionplan

import (
	"fmt"
	"math"

	spatial "go.viam.com/rdk/spatialmath"
)

// Collision is a pair of strings corresponding to names of Geometry objects in collision.
type Collision struct {
	name1, name2     string
	penetrationDepth float64
}

// CollisionGraph stores the relationship between Geometries, describing which are in collision.  The information for each
// Geometry is stored in a node in the graph, and edges between these nodes represent collisions.
type CollisionGraph struct {
	// indices is a mapping of Geometry names to their index in the nodes list and adjacency matrix
	indices map[string]int

	// nodes is a list of the nodes that comprise the graph
	nodes []*geometryNode

	// adjacencies represents the edges in the CollisionGraph as an adjacency matrix
	// For a pair of nodes (nodes[i], nodes[j]), there exists an edge between them if adjacencies[i][j] is true
	// This is always an undirected graph, this matrix will always be symmetric (adjacencies[i][j] == adjacencies[j][i])
	adjacencies [][]float64
}

// geometryNode defines a node for the CollisionGraph and only exists within this scope.
type geometryNode struct {
	name     string
	geometry spatial.Geometry
}

// newCollisionGraph is a helper function to instantiate a new CollisionGraph.  Note that since it does not set the
// adjacencies matrix, returned CollisionGraphs are not correct on their own and need further processing
// internal geometires represent geometries that are part of the robot and need to be checked against all geometries
// external geometries represent obstacles and other objects that are not a part of the robot. Collisions between 2 external
// geometries are not important and therefore not checked.
func newCollisionGraph(internal, external map[string]spatial.Geometry) (*CollisionGraph, error) {
	cg := &CollisionGraph{
		indices:     make(map[string]int, len(internal)+len(external)),
		nodes:       make([]*geometryNode, len(internal)+len(external)),
		adjacencies: make([][]float64, len(internal)+len(external)),
	}

	// add the geometries as nodes into the graph
	size := 0
	addGeometryMap := func(geometries map[string]spatial.Geometry) error {
		for name, geometry := range geometries {
			if _, ok := cg.indices[name]; ok {
				return fmt.Errorf("error calculating collisions, found geometry with duplicate name: %s", name)
			}
			cg.indices[name] = size
			cg.nodes[size] = &geometryNode{name, geometry}
			size++
		}
		return nil
	}
	if err := addGeometryMap(internal); err != nil {
		return nil, err
	}
	if err := addGeometryMap(external); err != nil {
		return nil, err
	}

	// initialize the adjacency matrix
	for i := range cg.adjacencies {
		cg.adjacencies[i] = make([]float64, len(internal))
		for j := range cg.adjacencies[i] {
			cg.adjacencies[i][j] = math.NaN()
		}
	}
	return cg, nil
}

// Collisions returns a list of Collision objects, with each element corresponding to a pair of names of nodes that
// are in collision within the specified CollisionGraph.
func (cg *CollisionGraph) Collisions() []Collision {
	collisions := make([]Collision, 0)
	for i := 1; i < len(cg.nodes); i++ {
		for j := 0; j < i && j < len(cg.adjacencies[i]); j++ {
			if cg.adjacencies[i][j] >= 0 {
				collisions = append(collisions, Collision{cg.nodes[i].name, cg.nodes[j].name, cg.adjacencies[i][j]})
			}
		}
	}
	return collisions
}

// CheckCollisions checks each possible Geometry pair for a collision, and if there is it will be stored as an edge in a
// newly instantiated CollisionGraph that is returned.
func CheckCollisions(internal, external map[string]spatial.Geometry) (*CollisionGraph, error) {
	cg, err := newCollisionGraph(internal, external)
	if err != nil {
		return nil, err
	}

	// iterate through all Geometry pairs and store collisions as edges in graph
	for i := 1; i < len(cg.nodes); i++ {
		for j := 0; j < i && j < len(cg.adjacencies[i]); j++ {
			distance, err := cg.nodes[i].geometry.DistanceFrom(cg.nodes[j].geometry)
			if err != nil {
				return nil, err
			}
			cg.adjacencies[i][j] = -distance
		}
	}
	return cg, nil
}

// CheckUniqueCollisions checks each possible Geometry pair for a collision, and if there is it will be stored as an edge
// in a newly instantiated CollisionGraph that is returned. Edges between geometries that already exist in the passed in
// "seen" CollisionGraph will not be present in the returned CollisionGraph.
func CheckUniqueCollisions(internal, external map[string]spatial.Geometry, seen *CollisionGraph) (*CollisionGraph, error) {
	cg, err := newCollisionGraph(internal, external)
	if err != nil {
		return nil, err
	}

	// iterate through all Geometry pairs and store new collisions as edges in graph
	var distance float64
	for i := 1; i < len(cg.nodes); i++ {
		for j := 0; j < i && j < len(cg.adjacencies[i]); j++ {
			// check for previously seen collisions and ignore them
			x, xOk := seen.indices[cg.nodes[i].name]
			y, yOk := seen.indices[cg.nodes[j].name]
			if y > x {
				x, y = y, x
			}
			if xOk && yOk && seen.adjacencies[x][y] >= 0 {
				// represent previously seen collisions as NaNs
				distance = math.NaN()
			} else {
				distance, err = cg.nodes[i].geometry.DistanceFrom(cg.nodes[j].geometry)
				if err != nil {
					return nil, err
				}
			}
			cg.adjacencies[i][j] = -distance
		}
	}
	return cg, nil
}
