package resource

import (
	"github.com/pkg/errors"
)

type resourceNode map[Name]interface{}

type resourceDependencies map[Name]resourceNode

// ResourceGraph The ResourceGraph maintains a collection of resources and their dependencies between each other.
// nolint: revive
type ResourceGraph struct {
	Nodes    resourceNode // list of nodes
	children resourceDependencies
	parents  resourceDependencies
	visited  map[Name]bool
}

// NewResourceGraph creates a new resource graph.
func NewResourceGraph() *ResourceGraph {
	return &ResourceGraph{
		children: make(resourceDependencies),
		parents:  make(resourceDependencies),
		Nodes:    make(resourceNode),
		visited:  map[Name]bool{},
	}
}

func (g *ResourceGraph) getAllDependenciesOf(node Name) resourceNode {
	// if the node doesn't exists then it cannot have dependencies
	if _, ok := g.Nodes[node]; !ok {
		return nil
	}
	out := make(resourceNode)
	next := []Name{node}
	for len(next) > 0 {
		found := []Name{}
		for _, n := range next {
			for nn := range g.children[n] {
				if _, ok := out[nn]; !ok {
					out[nn] = struct{}{}
					found = append(found, nn)
				}
			}
		}
		next = found
	}
	return out
}

func (g *ResourceGraph) getAllParentsOf(node Name) resourceNode {
	if _, ok := g.Nodes[node]; !ok {
		return nil
	}
	out := make(resourceNode)
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

func (g *ResourceGraph) leaves() []Name {
	leaves := make([]Name, 0)

	for node := range g.Nodes {
		if _, ok := g.children[node]; !ok {
			leaves = append(leaves, node)
		}
	}

	return leaves
}

// Clone deep copy of the resource graph.
func (g *ResourceGraph) Clone() *ResourceGraph {
	return &ResourceGraph{
		children: copyNodeMap(g.children),
		Nodes:    copyNodes(g.Nodes),
	}
}

func addResToSet(rd resourceDependencies, key, node Name) {
	// check if a resourceNode exists for a key, otherwise create one
	nodes, ok := rd[key]
	if !ok {
		nodes = make(resourceNode)
		rd[key] = nodes
	}
	nodes[node] = struct{}{}
}

// AddNode adds a node to the graph.
func (g *ResourceGraph) AddNode(node Name, iface interface{}) {
	g.Nodes[node] = iface
	g.visited[node] = false
}

// AddChildren add a dependency to a parent, create the parent if it doesn't exists yet.
func (g *ResourceGraph) AddChildren(child, parent Name) error {
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

func (g *ResourceGraph) pathFromToExists(source Name, goal Name) bool {
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

// IsDependingOn return wether or not a child depends on a given parent.
func (g *ResourceGraph) IsDependingOn(child, parent Name) bool {
	deps := g.getAllDependenciesOf(parent)
	_, ok := deps[child]
	return ok
}

// Remove remove a given node and all it's dependencies.
func (g *ResourceGraph) Remove(node Name) {
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

// MergeRemove remove comons nodes in both graphs.
func (g *ResourceGraph) MergeRemove(toRemove *ResourceGraph) {
	for k := range toRemove.Nodes {
		g.Remove(k)
	}
}

// MergeAdd merges two ResourceGraphs, if a node exists in both graphs, then it is silently replaced.
func (g *ResourceGraph) MergeAdd(toAdd *ResourceGraph) error {
	for node, r := range toAdd.Nodes {
		if i, ok := g.Nodes[node]; ok && i != nil {
			g.Remove(node)
		}
		g.AddNode(node, r)
		parents := toAdd.getAllParentsOf(node)
		for parent := range parents {
			if err := g.AddChildren(node, parent); err != nil {
				return err
			}
		}
	}
	return nil
}

// MergeNode adds a Node and copies it's dpendencies from a ResourceGraphs to another. The children nodes won't added.
func (g *ResourceGraph) MergeNode(node Name, origin *ResourceGraph) error {
	if r, ok := origin.Nodes[node]; ok {
		g.AddNode(node, r)
		parents := origin.getAllParentsOf(node)
		for parent := range parents {
			if err := g.AddChildren(node, parent); err != nil {
				return err
			}
		}
	}
	return nil
}

// TopologicalSort returns an array of nodes' Name ordered by fewest edges first.
func (g *ResourceGraph) TopologicalSort() []Name {
	ordered := []Name{}
	temp := g.Clone()
	for {
		leaves := temp.leaves()
		if len(leaves) == 0 {
			break
		}
		ordered = append(ordered, leaves...)
		for _, leaf := range leaves {
			temp.Remove(leaf)
		}
	}
	return ordered
}
