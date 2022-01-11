package resource

import (
	"github.com/pkg/errors"
)

type resnode map[Name]interface{}

type resdep map[Name]resnode

// ResourceGraph managing the dependency tree.
// nolint: revive
type ResourceGraph struct {
	Nodes    resnode // list of nodes
	children resdep
}

func (g *ResourceGraph) getAllDependenciesOf(node Name) resnode {
	// if the node doesn't exists then it cannot have dependencies
	if _, ok := g.Nodes[node]; !ok {
		return nil
	}
	out := make(resnode)
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

//nolint: unused
func (g *ResourceGraph) getAllChildrenOf(node Name) resnode {
	if _, ok := g.Nodes[node]; !ok {
		return nil
	}
	return g.children[node]
}

func (g *ResourceGraph) getAllParentsOf(node Name) resnode {
	if _, ok := g.Nodes[node]; !ok {
		return nil
	}
	out := make(resnode)
	for k, children := range g.children {
		if _, ok := children[node]; ok {
			out[k] = struct{}{}
		}
	}
	return out
}

func copyNodes(s resnode) resnode {
	out := make(resnode, len(s))
	for k, v := range s {
		out[k] = v
	}
	return out
}

func copyNodeMap(m resdep) resdep {
	out := make(resdep, len(m))
	for k, v := range m {
		out[k] = copyNodes(v)
	}
	return out
}

func removeNodeFromNodeMap(dm resdep, key, node Name) {
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

func addResToSet(rd resdep, key, node Name) {
	// check if a resnode exists for a key, otherwise create one
	nodes, ok := rd[key]
	if !ok {
		nodes = make(resnode)
		rd[key] = nodes
	}
	nodes[node] = struct{}{}
}

// NewResourceGraph creates a new resource graph.
func NewResourceGraph() *ResourceGraph {
	return &ResourceGraph{
		children: make(resdep),
		Nodes:    make(resnode),
	}
}

// AddNode add a node to the graph.
func (g *ResourceGraph) AddNode(node Name, iface interface{}) {
	g.Nodes[node] = iface
}

// AddChildren add a dependency to a parent, create the parent if it doesn't exists yet.
func (g *ResourceGraph) AddChildren(child, parent Name) error {
	if child == parent {
		return errors.Errorf("%s cannot depend on itself", child.Name)
	}
	if g.IsDependingOn(parent, child) {
		return errors.Errorf("circular dependency - %s already depends on %s", parent.Name, child.Name)
	}
	// Maybe we haven't encountered yet the parent so let's add it here and assign a nil interface
	if _, ok := g.Nodes[parent]; !ok {
		g.Nodes[parent] = nil
	}
	// Link nodes
	addResToSet(g.children, parent, child)
	return nil
}

// IsDependingOn return wether or not a child depends on a given parent.
func (g *ResourceGraph) IsDependingOn(child, parent Name) bool {
	deps := g.getAllDependenciesOf(parent)
	_, ok := deps[child]
	return ok
}

// Remove remove a given node and all of it's dependencies.
func (g *ResourceGraph) Remove(node Name) {
	for k, vertice := range g.children {
		if _, ok := vertice[node]; ok {
			removeNodeFromNodeMap(g.children, k, node)
		}
	}
	delete(g.children, node)
	delete(g.Nodes, node)
}

// MergeRemove remove comon nodes in both graphs.
func (g *ResourceGraph) MergeRemove(toRemove *ResourceGraph) {
	for k := range toRemove.Nodes {
		g.Remove(k)
	}
}

// MergeAdd merge two ResourceGraph, if a node exist in both graphs then it is silently replaced.
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

// MergeNode add a node and it's parrent to the left graph.
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

// TopologicalSort returns an array of nodes ordered by fewest edges first.
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

Go finished at Tue Jan 11 18:06:39
