package resource

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

type graphNodes map[Name]*GraphNode

type resourceDependencies map[Name]graphNodes

type transitiveClosureMatrix map[Name]map[Name]int

// Graph The Graph maintains a collection of resources and their dependencies between each other.
type Graph struct {
	mu                      sync.Mutex
	nodes                   graphNodes // list of nodes
	children                resourceDependencies
	parents                 resourceDependencies
	transitiveClosureMatrix transitiveClosureMatrix
	// logicalClock keeps track of updates to the graph. Each GraphNode has a
	// pointer to this logicalClock. Whenever SwapResource is called on a node
	// (the resource updates), the logicalClock is incremented.
	logicalClock *atomic.Int64
}

// NewGraph creates a new resource graph.
func NewGraph() *Graph {
	return &Graph{
		children:                resourceDependencies{},
		parents:                 resourceDependencies{},
		nodes:                   graphNodes{},
		transitiveClosureMatrix: transitiveClosureMatrix{},
		logicalClock:            &atomic.Int64{},
	}
}

// CurrLogicalClockValue returns current the logical clock value.
func (g *Graph) CurrLogicalClockValue() int64 {
	return g.logicalClock.Load()
}

func (g *Graph) getAllChildrenOf(node Name) graphNodes {
	if _, ok := g.nodes[node]; !ok {
		return nil
	}
	out := graphNodes{}
	for k, parents := range g.parents {
		if _, ok := parents[node]; ok {
			out[k] = &GraphNode{}
		}
	}
	return out
}

func (g *Graph) getAllParentOf(node Name) graphNodes {
	if _, ok := g.nodes[node]; !ok {
		return nil
	}
	out := graphNodes{}
	for k, children := range g.children {
		if _, ok := children[node]; ok {
			out[k] = &GraphNode{}
		}
	}
	return out
}

func copyNodes(s graphNodes) graphNodes {
	out := make(graphNodes, len(s))
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
		nodes:                   copyNodes(g.nodes),
		parents:                 copyNodeMap(g.parents),
		transitiveClosureMatrix: copyTransitiveClosureMatrix(g.transitiveClosureMatrix),
		logicalClock:            g.logicalClock,
	}
}

func addResToSet(rd resourceDependencies, key, node Name) {
	// check if a graphNodes exists for a key, otherwise create one
	nodes, ok := rd[key]
	if !ok {
		nodes = graphNodes{}
		rd[key] = nodes
	}
	nodes[node] = &GraphNode{}
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

// AddNode adds a node to the graph. Once added, the graph
// owns the node and to access it further, use Node.
func (g *Graph) AddNode(node Name, nodeVal *GraphNode) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.addNode(node, nodeVal)
}

// Node returns the node named name.
func (g *Graph) Node(node Name) (*GraphNode, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	rNode, ok := g.nodes[node]
	return rNode, ok
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

// FindNodesByShortNameAndAPI will look for resources matching both the API and the name.
func (g *Graph) FindNodesByShortNameAndAPI(name Name) []Name {
	g.mu.Lock()
	defer g.mu.Unlock()
	var ret []Name
	for k, v := range g.nodes {
		if name.Name == k.Name && name.API == k.API && v != nil {
			ret = append(ret, k)
		}
	}
	return ret
}

// FindNodesByAPI finds nodes with the given API.
func (g *Graph) FindNodesByAPI(api API) []Name {
	g.mu.Lock()
	defer g.mu.Unlock()
	var ret []Name
	for k := range g.nodes {
		if k.API == api {
			ret = append(ret, k)
		}
	}
	return ret
}

// findNodesByShortName returns all resources matching the given short name.
func (g *Graph) findNodesByShortName(name string) []Name {
	hasRemote := strings.Contains(name, ":")
	var matches []Name
	for nodeName := range g.nodes {
		if !(nodeName.API.IsComponent() || nodeName.API.IsService()) {
			continue
		}
		if hasRemote {
			// check the whole remote. we could technically check
			// a prefix of the remote but thats excluded for now.
			if nodeName.ShortName() == name {
				matches = append(matches, nodeName)
			}
			continue
		}

		// check without the remote name
		if nodeName.Name == name {
			matches = append(matches, nodeName)
		}
	}
	return matches
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

func (g *Graph) addNode(node Name, nodeVal *GraphNode) error {
	if nodeVal == nil {
		golog.Global().Errorw("addNode called with a nil value; setting to uninitialized", "name", node)
		nodeVal = NewUninitializedNode()
	}
	if val, ok := g.nodes[node]; ok {
		if !val.IsUninitialized() {
			return errors.Errorf("initialized node already exists with name %q; must swap instead", node)
		}
		return val.replace(nodeVal)
	}
	nodeVal.setGraphLogicalClock(g.logicalClock)
	g.nodes[node] = nodeVal

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
	return nil
}

// AddChild add a dependency to a parent, create the parent if it doesn't exists yet.
func (g *Graph) AddChild(child, parent Name) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.addChild(child, parent)
}

// RemoveChild unlink a child from its parent.
func (g *Graph) RemoveChild(child, parent Name) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.removeChild(child, parent)
}

func (g *Graph) addChild(child, parent Name) error {
	if child == parent {
		return errors.Errorf("%q cannot depend on itself", child.Name)
	}
	// Maybe we haven't encountered yet the parent so let's add it here and assign an uninitialized node
	if _, ok := g.nodes[parent]; !ok {
		if err := g.addNode(parent, NewUninitializedNode()); err != nil {
			return err
		}
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

func (g *Graph) removeChild(child, parent Name) {
	// Link nodes
	removeResFromSet(g.children, parent, child)
	removeResFromSet(g.parents, child, parent)
	g.removeTransitiveClosure(child, parent)
}

func (g *Graph) addTransitiveClosure(child, parent Name) {
	for u := range g.transitiveClosureMatrix {
		for v := range g.transitiveClosureMatrix[u] {
			g.transitiveClosureMatrix[u][v] += g.transitiveClosureMatrix[u][child] * g.transitiveClosureMatrix[parent][v]
		}
	}
}

func (g *Graph) removeTransitiveClosure(child, parent Name) {
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

// MarkForRemoval marks the given graph for removal at a later point
// by RemoveMarked.
func (g *Graph) MarkForRemoval(toMark *Graph) {
	toMark.mu.Lock()
	defer toMark.mu.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, v := range toMark.nodes {
		v.MarkForRemoval()
	}
}

// RemoveMarked removes previously marked nodes from the graph and returns
// all the resources removed that should now be closed by the caller.
func (g *Graph) RemoveMarked() []Resource {
	g.mu.Lock()
	defer g.mu.Unlock()

	// iterate in topological order so that we can close properly; otherwise
	// don't modify the cloned graph
	cloned := g.clone()
	sorted := cloned.TopologicalSort()

	var toClose []Resource
	for _, name := range sorted {
		rNode, ok := g.nodes[name]
		if !ok {
			// will never happen
			golog.Global().Errorw("invariant: expected to find node during removal", "name", name)
			continue
		}
		if rNode.MarkedForRemoval() {
			toClose = append(toClose, NewCloseOnlyResource(name, rNode.Close))
			g.remove(name)
		}
	}
	return toClose
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
		if err := g.addNode(node, toAdd.nodes[node]); err != nil {
			return err
		}
		parents := toAdd.getAllChildrenOf(node)
		for parent := range parents {
			if err := g.addChild(parent, node); err != nil {
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
		if err := g.addChild(parent, node); err != nil {
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
		if err := g.addNode(node, r); err != nil {
			return err
		}
		children := origin.getAllChildrenOf(node)
		for child := range children {
			if _, ok := g.nodes[child]; !ok {
				if err := g.addNode(child, NewUninitializedNode()); err != nil {
					return err
				}
			}
			if err := g.addChild(child, node); err != nil {
				return err
			}
		}
	}
	return nil
}

// TopologicalSort returns an array of nodes' Name ordered by fewest edges first.
// This can also be seen as being ordered where each name has no subsequent name
// depending on it.
func (g *Graph) TopologicalSort() []Name {
	_, sorted := g.topologicalSortInLevels()
	return sorted
}

func (g *Graph) topologicalSortInLevels() ([][]Name, []Name) {
	var ordered [][]Name
	var orderedFlattened []Name
	temp := g.Clone()
	for {
		leaves := temp.leaves()
		if len(leaves) == 0 {
			break
		}
		ordered = append(ordered, leaves)
		orderedFlattened = append(orderedFlattened, leaves...)
		for _, leaf := range leaves {
			temp.remove(leaf)
		}
	}
	return ordered, orderedFlattened
}

// TopologicalSortInLevels returns an array of array of nodes' Name ordered by fewest edges first.
// This can also be seen as being ordered where each name has no subsequent name
// depending on it.
func (g *Graph) TopologicalSortInLevels() [][]Name {
	sorted, _ := g.topologicalSortInLevels()
	return sorted
}

// ReverseTopologicalSort returns an array of nodes' Name ordered by most edges first.
// This can also be seen as being ordered where each name has no prior name
// depending on it.
func (g *Graph) ReverseTopologicalSort() []Name {
	ordered := g.TopologicalSort()
	for i, j := 0, len(ordered)-1; i < j; i, j = i+1, j-1 {
		ordered[i], ordered[j] = ordered[j], ordered[i]
	}
	return ordered
}

// ResolveDependencies attempts to link up unresolved dependencies after
// new changes to the graph.
func (g *Graph) ResolveDependencies(logger golog.Logger) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var allErrs error
	for nodeName, node := range g.nodes {
		unresolvedDeps := node.UnresolvedDependencies()

		if !node.hasUnresolvedDependencies() {
			continue
		}

		remainingDeps := make([]string, 0, len(unresolvedDeps))
		for _, dep := range unresolvedDeps {
			tryResolve := func() (Name, bool) {
				if dep == nodeName.String() {
					allErrs = multierr.Combine(errors.Errorf("node cannot depend on itself: %q", nodeName))
					logger.Errorw("node cannot depend on itself", "name", nodeName)
					return Name{}, false
				}
				if resName, err := NewFromString(dep); err == nil {
					return resName, true
				}

				// if a name is later added that conflicts, it will not
				// necessarily be caught unless the resource config changes.
				nodeNames := g.findNodesByShortName(dep)
				switch len(nodeNames) {
				case 0:
				case 1:
					if nodeNames[0].String() == nodeName.String() {
						allErrs = multierr.Combine(errors.Errorf("node cannot depend on itself: %q", nodeName))
						logger.Errorw("node cannot depend on itself", "name", nodeName)
						return Name{}, false
					}
					logger.Debugw(
						"dependency resolved for resource",
						"name", nodeName,
						"dependency", nodeNames[0],
					)
					return nodeNames[0], true
				default:
					allErrs = multierr.Combine(
						allErrs,
						errors.Errorf("conflicting names for resource %q: %v", nodeName, nodeNames))
					logger.Errorw(
						"cannot resolve dependency for resource due to multiple matching names",
						"name", nodeName,
						"conflicts", nodeNames,
					)
				}
				return Name{}, false
			}

			resolvedName, resolved := tryResolve()
			if resolved {
				if err := g.addChild(nodeName, resolvedName); err != nil {
					allErrs = multierr.Combine(allErrs, err)
					logger.Errorw(
						"error adding dependency for resource as a child to parent",
						"name", nodeName,
						"parent", resolvedName,
						"error", err,
					)
					resolved = false
				}
			}
			if !resolved {
				remainingDeps = append(remainingDeps, dep)
			}
		}
		node.setUnresolvedDependencies(remainingDeps...)
		if len(remainingDeps) == 0 {
			node.setDependenciesResolved()
		}
	}
	return allErrs
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
