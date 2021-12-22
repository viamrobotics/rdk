package motionplan

import (
	spatial "go.viam.com/core/spatialmath"
)

type collision struct{ a, b string }

type volumeNode struct {
	name   string
	volume spatial.Volume
}

type CollisionGraph struct {
	indices     map[string]int
	nodes       map[int]*volumeNode
	adjacencies [][]bool
	size        int
}

func NewCollisionGraph(vols map[string]spatial.Volume) *CollisionGraph {
	cg := &CollisionGraph{}
	cg.indices = make(map[string]int)
	cg.nodes = make(map[int]*volumeNode)
	for name, vol := range vols {
		cg.indices[name] = cg.size
		cg.nodes[cg.size] = &volumeNode{name, vol}
		cg.size++
	}
	cg.adjacencies = make([][]bool, cg.size)
	for i := range cg.adjacencies {
		cg.adjacencies[i] = make([]bool, cg.size)
	}
	return cg
}

// Collisions returns a list with each element corresponding to a pair of names of nodes that are in collision within
// the specified CollisionGraph
func (cg *CollisionGraph) Collisions() []collision {
	collisions := make([]collision, 0)
	for i := 0; i < cg.size; i++ {
		for j := i + 1; j < cg.size; j++ {
			if cg.adjacencies[i][j] {
				collisions = append(collisions, collision{cg.nodes[i].name, cg.nodes[j].name})
			}
		}
	}
	return collisions
}

func CheckCollisions(vols map[string]spatial.Volume) (*CollisionGraph, error) {
	cg := NewCollisionGraph(vols)

	// iterate through all Volume pairs and store collisions as edges in graph
	for i := 0; i < cg.size; i++ {
		for j := i + 1; j < cg.size; j++ {
			err := cg.checkAddEdge(i, j)
			if err != nil {
				return nil, err
			}
		}
	}
	return cg, nil
}

func CheckUniqueCollisions(vols map[string]spatial.Volume, seen *CollisionGraph) (*CollisionGraph, error) {
	cg := NewCollisionGraph(vols)

	// iterate through all Volume pairs and store collisions as edges in graph
	for i := 0; i < cg.size; i++ {
		for j := i + 1; j < cg.size; j++ {
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

// checkAddEdge is a helper function to check for a collision at indices (i, j) and if one exists, add an edge between
// the nodes
func (cg *CollisionGraph) checkAddEdge(i, j int) error {
	collides, err := cg.nodes[i].volume.CollidesWith(cg.nodes[j].volume)
	if err != nil {
		return err
	}
	cg.adjacencies[i][j] = collides
	cg.adjacencies[j][i] = collides
	return nil
}
