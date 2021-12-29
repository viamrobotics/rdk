package motionplan

import (
	spatial "go.viam.com/rdk/spatialmath"
)

// Collision is a pair of strings corresponding to names of Volume objects in collision.
type Collision struct{ name1, name2 string }

// CollisionGraph stores the relationship between Volumes, describing which are in collision.  The information for each
// Volume is stored in a node in the graph, and edges between these nodes represent collisions.
type CollisionGraph struct {
	// indices is a mapping of Volume names to their index in the nodes list and adjacency matrix
	indices map[string]int

	// nodes is a list of the nodes that comprise the graph
	nodes []*volumeNode

	// adjacencies represents the edges in the CollisionGraph as an adjacency matrix
	// For a pair of nodes (nodes[i], nodes[j]), there exists an edge between them if adjacencies[i][j] is true
	// This is always an undirected graph, this matrix will always be symmetric (adjacencies[i][j] == adjacencies[j][i])
	adjacencies [][]bool
}

// volumeNode defines a node for the CollisionGraph and only exists within this scope.
type volumeNode struct {
	name   string
	volume spatial.Volume
}

// newCollisionGraph is a helper function to instantiate a new CollisionGraph.  Note that since it does not set the
// adjacencies matrix, returned CollisionGraphs are not correct on their own and need further processing.
func newCollisionGraph(vols map[string]spatial.Volume) *CollisionGraph {
	cg := &CollisionGraph{}
	cg.indices = make(map[string]int, len(vols))
	cg.nodes = make([]*volumeNode, len(vols))

	size := 0
	for name, vol := range vols {
		cg.indices[name] = size
		cg.nodes[size] = &volumeNode{name, vol}
		size++
	}

	cg.adjacencies = make([][]bool, size)
	for i := range cg.adjacencies {
		cg.adjacencies[i] = make([]bool, size)
	}
	return cg
}

// Collisions returns a list of Collision objects, with each element corresponding to a pair of names of nodes that
// are in collision within the specified CollisionGraph.
func (cg *CollisionGraph) Collisions() []Collision {
	collisions := make([]Collision, 0)
	for i := 0; i < len(cg.nodes)-1; i++ {
		for j := i + 1; j < len(cg.nodes); j++ {
			if cg.adjacencies[i][j] {
				collisions = append(collisions, Collision{cg.nodes[i].name, cg.nodes[j].name})
			}
		}
	}
	return collisions
}

// checkAddEdge is a helper function to check for a collision at indices (i, j) and if one exists, add an edge between
// the nodes.
func (cg *CollisionGraph) checkAddEdge(i, j int) error {
	collides, err := cg.nodes[i].volume.CollidesWith(cg.nodes[j].volume)
	if err != nil {
		return err
	}
	cg.adjacencies[i][j] = collides
	cg.adjacencies[j][i] = collides
	return nil
}

// CheckCollisions checks each possible Volume pair for a collision, and if there is it will be stored as an edge in a
// newly instantiated CollisionGraph that is returned.
func CheckCollisions(vols map[string]spatial.Volume) (*CollisionGraph, error) {
	cg := newCollisionGraph(vols)

	// iterate through all Volume pairs and store collisions as edges in graph
	for i := 0; i < len(cg.nodes)-1; i++ {
		for j := i + 1; j < len(cg.nodes); j++ {
			err := cg.checkAddEdge(i, j)
			if err != nil {
				return nil, err
			}
		}
	}
	return cg, nil
}

// CheckUniqueCollisions checks each possible Volume pair for a collision, and if there is it will be stored as an edge
// in a newly instantiated CollisionGraph that is returned. Edges between volumes that already exist in the passed in
// "seen" CollisionGraph will not be present in the returned CollisionGraph.
func CheckUniqueCollisions(vols map[string]spatial.Volume, seen *CollisionGraph) (*CollisionGraph, error) {
	cg := newCollisionGraph(vols)

	// iterate through all Volume pairs and store new collisions as edges in graph
	for i := 0; i < len(cg.nodes)-1; i++ {
		for j := i + 1; j < len(cg.nodes); j++ {
			// ignore any previously seen collisions
			x, xk := seen.indices[cg.nodes[i].name]
			y, yk := seen.indices[cg.nodes[j].name]
			if xk && yk && seen.adjacencies[x][y] {
				continue
			}
			err := cg.checkAddEdge(i, j)
			if err != nil {
				return nil, err
			}
		}
	}
	return cg, nil
}
