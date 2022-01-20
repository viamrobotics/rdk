package resource

import (
	"sync"

	"github.com/pkg/errors"
)

type resourceNode map[Name]interface{}

type resourceDependencies map[Name]resourceNode

// Graph The Graph maintains a collection of resources and their dependencies between each other.
type Graph struct {
	mu       sync.Mutex
	Nodes    resourceNode // list of nodes
	children resourceDependencies
	parents  resourceDependencies
	visited  map[Name]bool
}

// NewGraph creates a new resource graph.
func NewGraph() *Graph {
	return &Graph{
		children: resourceDependencies{},
		parents:  resourceDependencies{},
		Nodes:    resourceNode{},
		visited:  map[Name]bool{},
	}
}

func (g *Graph) getAllParentsOf(node Name) resourceNode {
	if _, ok := g.Nodes[node]; !ok {
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

func copyNodes(s resourceNode) resourceNode {
	out := make(resourceNode, len(s))
	for k, v := range s {
		out[k] = v
	}
	return out
}

func copyNodeMap(m resourceDependencies) resourceDependencies {
	out := make(resourceDependencies, len(m))
	for k, v := range m {
		out[k] = copyNodes(v)
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

	for node := range g.Nodes {
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
	return &Graph{
		children: copyNodeMap(g.children),
		Nodes:    copyNodes(g.Nodes),
		parents:  copyNodeMap(g.parents),
		visited:  map[Name]bool{},
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

// AddNode adds a node to the graph.
func (g *Graph) AddNode(node Name, iface interface{}) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.addNode(node, iface)
}

func (g *Graph) addNode(node Name, iface interface{}) {
	g.Nodes[node] = iface
	g.visited[node] = false
}

// AddChildren add a dependency to a parent, create the parent if it doesn't exists yet.
func (g *Graph) AddChildren(child, parent Name) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.addChildren(child, parent)
}

func (g *Graph) addChildren(child, parent Name) error {
	if child == parent {
		return errors.Errorf("%q cannot depend on itself", child.Name)
	}
	// Maybe we haven't encountered yet the parent so let's add it here and assign a nil interface
	if _, ok := g.Nodes[parent]; !ok {
		g.Nodes[parent] = nil
	} else if g.pathFromToExists(parent, child) {
		return errors.Errorf("circular dependency - %q already depends on %q", parent.Name, child.Name)
	}
	// Link nodes
	addResToSet(g.children, parent, child)
	addResToSet(g.parents, child, parent)
	return nil
}

func (g *Graph) pathFromToExists(source Name, goal Name) bool {
	for node := range g.Nodes {
		g.visited[node] = false
	}
	g.visited[source] = true
	for node := range g.parents[source] {
		if node == goal {
			return true
		} else if !g.visited[node] {
			if g.pathFromToExists(node, goal) {
				return true
			}
		}
	}
	return false
}

func (g *Graph) remove(node Name) {
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
	delete(g.parents, node)
	delete(g.children, node)
	delete(g.Nodes, node)
	delete(g.visited, node)
}

// Remove remove a given node and all it's dependencies.
func (g *Graph) Remove(node Name) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.remove(node)
}

// MergeRemove remove comons nodes in both graphs.
func (g *Graph) MergeRemove(toRemove *Graph) {
	toRemove.mu.Lock()
	defer toRemove.mu.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()

	for k := range toRemove.Nodes {
		g.remove(k)
	}
}

// MergeAdd merges two Graphs, if a node exists in both graphs, then it is silently replaced.
func (g *Graph) MergeAdd(toAdd *Graph) error {
	toAdd.mu.Lock()
	defer toAdd.mu.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()
	for node, r := range toAdd.Nodes {
		if i, ok := g.Nodes[node]; ok && i != nil {
			g.remove(node)
		}
		g.addNode(node, r)
		parents := toAdd.getAllParentsOf(node)
		for parent := range parents {
			if err := g.addChildren(node, parent); err != nil {
				return err
			}
		}
	}
	return nil
}

// MergeNode adds a Node and copies it's dpendencies from a Graphs to another. The children nodes won't added.
func (g *Graph) MergeNode(node Name, origin *Graph) error {
	origin.mu.Lock()
	defer origin.mu.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()
	if r, ok := origin.Nodes[node]; ok {
		g.addNode(node, r)
		parents := origin.getAllParentsOf(node)
		for parent := range parents {
			if err := g.addChildren(node, parent); err != nil {
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
