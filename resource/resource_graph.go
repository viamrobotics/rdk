package resource

import (
	"sync"

	"github.com/pkg/errors"
)

type resourceNode map[Name]interface{}

type resourceDependencies map[Name]resourceNode

type transitiveClosureMatrix map[Name]map[Name]int

// Graph The Graph maintains a collection of resources and their dependencies between each other.
type Graph struct {
	mu                      sync.Mutex
	nodes                   resourceNode // list of nodes
	children                resourceDependencies
	parents                 resourceDependencies
	transitiveClosureMatrix transitiveClosureMatrix
}

// NewGraph creates a new resource graph.
func NewGraph() *Graph {
	return &Graph{
		children:                resourceDependencies{},
		parents:                 resourceDependencies{},
		nodes:                   resourceNode{},
		transitiveClosureMatrix: transitiveClosureMatrix{},
	}
}

func (g *Graph) getAllChildrenOf(node Name) resourceNode {
	if _, ok := g.nodes[node]; !ok {
		return nil
	}
	out := resourceNode{}
	for k, parents := range g.parents {
		if _, ok := parents[node]; ok {
			out[k] = struct{}{}
		}
	}
	return out
}

func (g *Graph) getAllParentOf(node Name) resourceNode {
	if _, ok := g.nodes[node]; !ok {
		return nil
	}
	out := resourceNode{}
	for k, children := range g.children {
		if _, ok := children[node]; ok {
			out[k] = struct{}{}
		}
	}
	return out
}

func copynodes(s resourceNode) resourceNode {
	out := make(resourceNode, len(s))
	for k, v := range s {
		out[k] = v
	}
	return out
}

func copyNodeMap(m resourceDependencies) resourceDependencies {
	out := make(resourceDependencies, len(m))
	for k, v := range m {
		out[k] = copynodes(v)
	}
	return out
}

func copyTransitiveClosureMatrix(m transitiveClosureMatrix) transitiveClosureMatrix {
	out := make(transitiveClosureMatrix, len(m))
	for i := range m {
		out[i] = make(map[Name]int, len(m[i]))
		for j, v := range m[i] {
			out[i][j] = v
		}
	}
	return out
}

func removeNodeFromNodeMap(dm resourceDependencies, key, node Name) {
	if nodes := dm[key]; len(nodes) == 1 {
		delete(dm, key)
	} else {
		delete(nodes, node)
	}
}

func (g *Graph) leaves() []Name {
	leaves := make([]Name, 0)

	for node := range g.nodes {
		if _, ok := g.children[node]; !ok {
			leaves = append(leaves, node)
		}
	}

	return leaves
}

// Clone deep copy of the resource graph.
func (g *Graph) Clone() *Graph {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.clone()
}

func (g *Graph) clone() *Graph {
	return &Graph{
		children:                copyNodeMap(g.children),
		nodes:                   copynodes(g.nodes),
		parents:                 copyNodeMap(g.parents),
		transitiveClosureMatrix: copyTransitiveClosureMatrix(g.transitiveClosureMatrix),
	}
}

func addResToSet(rd resourceDependencies, key, node Name) {
	// check if a resourceNode exists for a key, otherwise create one
	nodes, ok := rd[key]
	if !ok {
		nodes = resourceNode{}
		rd[key] = nodes
	}
	nodes[node] = struct{}{}
}

func removeResFromSet(rd resourceDependencies, key, node Name) {
	if nodes, ok := rd[key]; ok {
		delete(nodes, node)
		if len(nodes) == 0 {
			delete(rd, key)
		}
	}
}

// IsNodeDependingOn returns true if child is depending on node.
func (g *Graph) IsNodeDependingOn(node, child Name) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.isNodeDependingOn(node, child)
}

// AddNode adds a node to the graph.
func (g *Graph) AddNode(node Name, iface interface{}) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.addNode(node, iface)
}

// Node returns the node named name.
func (g *Graph) Node(node Name) (interface{}, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	iface, ok := g.nodes[node]
	return iface, ok
}

// Names returns the all resource graph names.
func (g *Graph) Names() []Name {
	g.mu.Lock()
	defer g.mu.Unlock()
	names := make([]Name, len(g.nodes))
	i := 0
	for k := range g.nodes {
		names[i] = k
		i++
	}
	return names
}

// GetAllChildrenOf returns all direct children of a node.
func (g *Graph) GetAllChildrenOf(node Name) []Name {
	g.mu.Lock()
	defer g.mu.Unlock()
	names := []Name{}
	children := g.getAllChildrenOf(node)
	for child := range children {
		names = append(names, child)
	}
	return names
}

// GetAllParentsOf returns all parents of a given node.
func (g *Graph) GetAllParentsOf(node Name) []Name {
	g.mu.Lock()
	defer g.mu.Unlock()
	names := []Name{}
	children := g.getAllParentOf(node)
	for child := range children {
		names = append(names, child)
	}
	return names
}

func (g *Graph) addNode(node Name, iface interface{}) {
	g.nodes[node] = iface

	if _, ok := g.transitiveClosureMatrix[node]; !ok {
		g.transitiveClosureMatrix[node] = map[Name]int{}
	}
	for n := range g.nodes {
		for v := range g.transitiveClosureMatrix {
			if _, ok := g.transitiveClosureMatrix[n][v]; !ok {
				g.transitiveClosureMatrix[n][v] = 0
			}
		}
	}
	g.transitiveClosureMatrix[node][node] = 1
}

// RenameNode rename a node from old to new keeping it's dependencies. On success the old node is destroyed.
func (g *Graph) RenameNode(old, _new Name) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.renameNode(old, _new)
}

func (g *Graph) renameNode(old, _new Name) error {
	if _, ok := g.nodes[old]; !ok {
		return errors.Errorf("old node %q doesn't exists", old)
	}
	if _, ok := g.nodes[_new]; ok {
		return errors.Errorf("new node %q already exists", _new)
	}
	oldParents := g.getAllParentOf(old)
	oldChildren := g.getAllChildrenOf(old)

	g.addNode(_new, g.nodes[old])
	for p := range oldParents {
		g.removeChildren(old, p)
		if err := g.addChildren(_new, p); err != nil {
			return err
		}
	}
	for c := range oldChildren {
		g.removeChildren(c, old)
		if err := g.addChildren(c, _new); err != nil {
			return err
		}
	}
	g.remove(old)
	return nil
}

// AddChildren add a dependency to a parent, create the parent if it doesn't exists yet.
func (g *Graph) AddChildren(child, parent Name) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.addChildren(child, parent)
}

// RemoveChildren unlink a child from its parent.
func (g *Graph) RemoveChildren(child, parent Name) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.removeChildren(child, parent)
}

func (g *Graph) addChildren(child, parent Name) error {
	if child == parent {
		return errors.Errorf("%q cannot depend on itself", child.Name)
	}
	// Maybe we haven't encountered yet the parent so let's add it here and assign a nil interface
	if _, ok := g.nodes[parent]; !ok {
		g.addNode(parent, nil)
	} else if g.transitiveClosureMatrix[parent][child] != 0 {
		return errors.Errorf("circular dependency - %q already depends on %q", parent.Name, child.Name)
	}
	if _, ok := g.parents[child][parent]; ok {
		return nil
	}
	// Link nodes
	addResToSet(g.children, parent, child)
	addResToSet(g.parents, child, parent)
	g.addTransitiveClosure(child, parent)
	return nil
}

func (g *Graph) removeChildren(child, parent Name) {
	// Link nodes
	removeResFromSet(g.children, parent, child)
	removeResFromSet(g.parents, child, parent)
	g.removeTransitiveClosure(child, parent)
}

func (g *Graph) addTransitiveClosure(child Name, parent Name) {
	for u := range g.transitiveClosureMatrix {
		for v := range g.transitiveClosureMatrix[u] {
			g.transitiveClosureMatrix[u][v] += g.transitiveClosureMatrix[u][child] * g.transitiveClosureMatrix[parent][v]
		}
	}
}

func (g *Graph) removeTransitiveClosure(child Name, parent Name) {
	for u := range g.transitiveClosureMatrix {
		for v := range g.transitiveClosureMatrix[u] {
			g.transitiveClosureMatrix[u][v] -= g.transitiveClosureMatrix[u][child] * g.transitiveClosureMatrix[parent][v]
		}
	}
}

func (g *Graph) remove(node Name) {
	for k := range g.parents[node] {
		g.removeTransitiveClosure(node, k)
	}
	for k := range g.children[node] {
		g.removeTransitiveClosure(k, node)
	}
	for k, vertice := range g.children {
		if _, ok := vertice[node]; ok {
			removeNodeFromNodeMap(g.children, k, node)
		}
	}
	for k, vertice := range g.parents {
		if _, ok := vertice[node]; ok {
			removeNodeFromNodeMap(g.parents, k, node)
		}
	}
	delete(g.transitiveClosureMatrix, node)
	delete(g.parents, node)
	delete(g.children, node)
	delete(g.nodes, node)
}

// Remove remove a given node and all it's dependencies.
func (g *Graph) Remove(node Name) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.remove(node)
}

// MergeRemove remove common nodes in both graphs.
func (g *Graph) MergeRemove(toRemove *Graph) {
	toRemove.mu.Lock()
	defer toRemove.mu.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()

	for k := range toRemove.nodes {
		g.remove(k)
	}
}

// MergeAdd merges two Graphs, if a node exists in both graphs, then it is silently replaced.
func (g *Graph) MergeAdd(toAdd *Graph) error {
	sorted := toAdd.TopologicalSort()
	toAdd.mu.Lock()
	defer toAdd.mu.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, node := range sorted {
		if i, ok := g.nodes[node]; ok && i != nil {
			g.remove(node)
		}
		g.addNode(node, toAdd.nodes[node])
		parents := toAdd.getAllChildrenOf(node)
		for parent := range parents {
			if err := g.addChildren(parent, node); err != nil {
				return err
			}
		}
	}
	return nil
}

// ReplaceNodesParents replaces all parent of a given node with the parents of the other graph.
func (g *Graph) ReplaceNodesParents(node Name, other *Graph) error {
	other.mu.Lock()
	defer other.mu.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.nodes[node]; !ok {
		return errors.Errorf("cannot copy parents to non existing node %q", node.Name)
	}
	for k := range g.parents[node] {
		g.removeTransitiveClosure(node, k)
	}
	for k, vertice := range g.parents {
		if _, ok := vertice[node]; ok {
			removeNodeFromNodeMap(g.parents, k, node)
		}
	}
	parents := other.getAllChildrenOf(node)
	for parent := range parents {
		if err := g.addChildren(parent, node); err != nil {
			return err
		}
	}
	return nil
}

// CopyNodeAndChildren adds a Node and it's children from another graph.
func (g *Graph) CopyNodeAndChildren(node Name, origin *Graph) error {
	origin.mu.Lock()
	defer origin.mu.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()
	if r, ok := origin.nodes[node]; ok {
		g.addNode(node, r)
		children := origin.getAllChildrenOf(node)
		for child := range children {
			if _, ok := g.nodes[child]; !ok {
				g.addNode(child, nil)
			}
			if err := g.addChildren(child, node); err != nil {
				return err
			}
		}
	}
	return nil
}

// TopologicalSort returns an array of nodes' Name ordered by fewest edges first.
func (g *Graph) TopologicalSort() []Name {
	ordered := []Name{}
	temp := g.Clone()
	for {
		leaves := temp.leaves()
		if len(leaves) == 0 {
			break
		}
		ordered = append(ordered, leaves...)
		for _, leaf := range leaves {
			temp.remove(leaf)
		}
	}
	return ordered
}

// ReverseTopologicalSort returns an array of nodes' Name ordered by most edges first.
func (g *Graph) ReverseTopologicalSort() []Name {
	ordered := g.TopologicalSort()
	for i, j := 0, len(ordered)-1; i < j; i, j = i+1, j-1 {
		ordered[i], ordered[j] = ordered[j], ordered[i]
	}
	return ordered
}

// FindNodeByName returns a full resource name based on name, note if name is a duplicate the first one found will be returned.
func (g *Graph) FindNodeByName(name string) (*Name, bool) {
	for nodeName := range g.nodes {
		if nodeName.Name == name && !nodeName.IsRemoteResource() {
			return &nodeName, true
		}
	}
	return nil, false
}

func (g *Graph) isNodeDependingOn(node, child Name) bool {
	if _, ok := g.nodes[node]; !ok {
		return false
	}
	if _, ok := g.nodes[child]; !ok {
		return false
	}
	return g.transitiveClosureMatrix[child][node] != 0
}

// SubGraphFrom returns a Sub-Graph containing all linked dependencies starting with node Name.
func (g *Graph) SubGraphFrom(node Name) (*Graph, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.nodes[node]; !ok {
		return nil, errors.Errorf("cannot create sub-graph from non existing node %q ", node.Name)
	}
	subGraph := g.clone()
	sorted := subGraph.ReverseTopologicalSort()
	for _, n := range sorted {
		if !subGraph.isNodeDependingOn(node, n) {
			subGraph.remove(n)
		}
	}
	return subGraph, nil
}
