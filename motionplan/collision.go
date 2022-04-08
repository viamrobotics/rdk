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

// geometryNode defines a node for the CollisionGraph and only exists within this scope.
// A node can either occupy positive or negative space as defined by the positiveSpace bool.
//		- positive space implies the geometry has physical volume
//      - negative space implies the geometry  is the absence of physical volume
type geometryNode struct {
	name          string
	geometry      spatial.Geometry
	positiveSpace bool
}

// CollisionGraph stores the relationship between Geometries.  The information for each Geometry is stored in a node in the graph,
// and edges between these nodes are added when the geometries satisfy one of the two conditions below
// 		- 2 positive space geometries intersect
//      - a positive space geometry is not fully encompassed by a negative space geometry
type CollisionGraph struct {
	// indices is a mapping of Geometry names to their index in the nodes list and adjacency matrix
	indices map[string]int

	// nodes is a list of the nodes that comprise the graph
	nodes []*geometryNode

	// adjacencies represents the edges in the CollisionGraph as an adjacency matrix
	// For a pair of nodes (nodes[i], nodes[j]), there exists an edge between them if adjacencies[i][j] is true
	adjacencies [][]float64

	// hasInteractionSpace is a boolean used to track if any interaction space needs to be accounted for
	hasInteractionSpace bool
}

// newCollisionGraph is a helper function to instantiate a new CollisionGraph.  Note that since it does not set the
// adjacencies matrix, returned CollisionGraphs are not correct on their own and need further processing
// robot geometries represent geometries that are part of the robot and need to be checked against all geometries
// obstacles geometries occupy positive space and are other objects that are not a part of the robot.
// interactionSpaces geometries occupy negative space and represent the space that must encompass the robot.
// The only collisions that are checked are ones between the robot and obstacles/interactionSpaces, the others are unimportant.
func newCollisionGraph(robot, obstacles, interactionSpaces map[string]spatial.Geometry) (*CollisionGraph, error) {
	cg := &CollisionGraph{
		indices:     make(map[string]int, len(robot)+len(obstacles)+len(interactionSpaces)),
		nodes:       make([]*geometryNode, len(robot)+len(obstacles)+len(interactionSpaces)),
		adjacencies: make([][]float64, len(robot)+len(obstacles)+len(interactionSpaces)),
	}

	// create all the nodes for the graph
	size := 0
	addGeometryMap := func(geometries map[string]spatial.Geometry, positiveSpace bool) error {
		for name, geometry := range geometries {
			if _, ok := cg.indices[name]; ok {
				return fmt.Errorf("error calculating collisions, found geometry with duplicate name: %s", name)
			}
			cg.indices[name] = size
			cg.nodes[size] = &geometryNode{name, geometry, positiveSpace}
			size++
		}
		return nil
	}
	if err := addGeometryMap(robot, true); err != nil {
		return nil, err
	}
	if err := addGeometryMap(obstacles, true); err != nil {
		return nil, err
	}
	if len(interactionSpaces) > 0 {
		cg.hasInteractionSpace = true
		if err := addGeometryMap(interactionSpaces, false); err != nil {
			return nil, err
		}
	}

	// initialize the adjacency matrix
	for i := range cg.adjacencies {
		cg.adjacencies[i] = make([]float64, len(robot))
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
	for i := 0; i < len(cg.adjacencies[0]); i++ {
		inInteractionSpace := false
		for j := i + 1; j < len(cg.nodes); j++ {
			if cg.adjacencies[j][i] >= 0 && cg.nodes[j].positiveSpace {
				collisions = append(collisions, Collision{cg.nodes[i].name, cg.nodes[j].name, cg.adjacencies[j][i]})
			} else if cg.adjacencies[j][i] == 0 && !cg.nodes[j].positiveSpace {
				inInteractionSpace = true
			}
		}
		if cg.hasInteractionSpace && !inInteractionSpace {
			collisions = append(collisions, Collision{cg.nodes[i].name, "interaction space", 1})
		}
	}
	return collisions
}

// CheckCollisions checks each possible Geometry pair for a collision, and if there is it will be stored as an edge in a
// newly instantiated CollisionGraph that is returned.
func CheckCollisions(robot, obstacles, interactionSpaces map[string]spatial.Geometry) (*CollisionGraph, error) {
	cg, err := newCollisionGraph(robot, obstacles, interactionSpaces)
	if err != nil {
		return nil, err
	}

	// iterate through all Geometry pairs and store collisions as edges in graph
	for i := 1; i < len(cg.nodes); i++ {
		for j := 0; j < i && j < len(cg.adjacencies[i]); j++ {
			distance, err := checkCollision(cg.nodes[j], cg.nodes[i])
			if err != nil {
				return nil, err
			}
			cg.adjacencies[i][j] = distance
		}
	}
	return cg, nil
}

// CheckUniqueCollisions checks each possible Geometry pair for a collision, and if there is it will be stored as an edge
// in a newly instantiated CollisionGraph that is returned. Edges between geometries that already exist in the passed in
// "seen" CollisionGraph will not be present in the returned CollisionGraph.
func CheckUniqueCollisions(
	internal, external, interactionSpaces map[string]spatial.Geometry,
	seen *CollisionGraph,
) (*CollisionGraph, error) {
	cg, err := newCollisionGraph(internal, external, interactionSpaces)
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
				distance, err = checkCollision(cg.nodes[j], cg.nodes[i])
				if err != nil {
					return nil, err
				}
			}
			cg.adjacencies[i][j] = distance
		}
	}
	return cg, nil
}

func checkCollision(positiveSpaceNode, otherNode *geometryNode) (distance float64, err error) {
	if otherNode.positiveSpace {
		distance, err = positiveSpaceNode.geometry.DistanceFrom(otherNode.geometry)
		if err != nil {
			return math.NaN(), err
		}
	} else {
		encompassed, err := positiveSpaceNode.geometry.EncompassedBy(otherNode.geometry)
		if err != nil {
			return math.NaN(), err
		}
		if !encompassed {
			distance = -1 // TODO(rb): EncompassedBy should also report distance required to resolve the collision
		}
	}
	distance = -distance // multiply distance by -1 so that weights of edges are positive
	return
}
