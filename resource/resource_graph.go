package resource

import (
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/ftdc"
	"go.viam.com/rdk/logging"
)

// NodeNotFoundError is returned when the name and API combination are not found after
// by FindBySimpleNameAndAPI.
type NodeNotFoundError struct {
	Name string
	API  API
}

func (e *NodeNotFoundError) Error() string {
	return fmt.Sprintf("no node found with api %s and name %s", e.API, e.Name)
}

// IsNodeNotFoundError returns if the given error is any kind of node not found error.
func IsNodeNotFoundError(err error) bool {
	var nnfErr *NodeNotFoundError
	return errors.As(err, &nnfErr)
}

// MultipleMatchingRemoteNodesError is returned when more than one remote resource node matches the name
// and API query passed to FindBySimpleNameAndAPI.
type MultipleMatchingRemoteNodesError struct {
	Name    string
	API     API
	Remotes []string
}

func (e *MultipleMatchingRemoteNodesError) Error() string {
	return fmt.Sprintf("found multiple nodes matching api %s and name %s from remotes %v", e.API, e.Name, e.Remotes)
}

// IsMultipleMatchingRemoteNodesError returns if the given error is any kind of multiple
// matching remote nodes error.
func IsMultipleMatchingRemoteNodesError(err error) bool {
	var mmrnErr *MultipleMatchingRemoteNodesError
	return errors.As(err, &mmrnErr)
}

type graphNodes map[Name]*GraphNode

// graphStorage is used by [Graph] to store the collection of nodes. It
// consists of
// - a map of [Name] to *[GraphNode]s, used to serve most queries
// - a map of ([API], string]) tuples to *[GraphNode]s, used to quickly serve [Graph.FindBySimpleNameAndAPI] queries
// The various methods on graphStorage handle keeping these two maps in sync.
// graphStorage itself is _not_ thread safe. and depends on [Graph] to handle
// synchronization between goroutines.
type graphStorage struct {
	nodes           graphNodes
	simpleNameCache simpleNameCache
}

func (s graphStorage) Get(name Name) (*GraphNode, bool) {
	node, ok := s.nodes[name]
	return node, ok
}

func (s graphStorage) Set(name Name, node *GraphNode) {
	s.nodes[name] = node
	s.setSimpleNameCache(name, node)
}

func (s graphStorage) setSimpleNameCache(name Name, node *GraphNode) {
	simpleName := simpleNameKey{node.prefix + name.Name, name.API}
	val := s.simpleNameCache[simpleName]
	if val == nil {
		val = &simpleNameVal{
			remote: map[string]*GraphNode{},
		}
		s.simpleNameCache[simpleName] = val
	}
	if name.Remote == "" {
		val.local = node
	} else {
		val.remote[name.Remote] = node
	}
}

func (s graphStorage) UpdateSimpleName(name Name, prevPrefix string, node *GraphNode) {
	if prevPrefix == node.prefix {
		return
	}
	prevSimpleName := simpleNameKey{prevPrefix + name.Name, name.API}

	prevVal := s.simpleNameCache[prevSimpleName]
	if prevVal != nil {
		if name.Remote == "" {
			prevVal.local = nil
		} else {
			delete(prevVal.remote, name.Remote)
		}
	}

	s.setSimpleNameCache(name, node)
}

func (s graphStorage) Delete(name Name) {
	node := s.nodes[name]
	delete(s.nodes, name)
	if node == nil {
		return
	}
	simpleName := simpleNameKey{node.prefix + name.Name, name.API}
	existing := s.simpleNameCache[simpleName]
	if existing == nil {
		return
	}
	if name.Remote == "" {
		existing.local = nil
		return
	}
	delete(existing.remote, name.Remote)
}

func (s graphStorage) Copy() graphStorage {
	out := graphStorage{
		nodes:           maps.Clone(s.nodes),
		simpleNameCache: simpleNameCache{},
	}
	for k, v := range s.simpleNameCache {
		out.simpleNameCache[k] = &simpleNameVal{
			local:  v.local,
			remote: maps.Clone(v.remote),
		}
	}
	return out
}

func (s graphStorage) FindBySimpleNameAndAPI(name string, api API) (*GraphNode, error) {
	val := s.simpleNameCache[simpleNameKey{name, api}]
	if val == nil {
		return nil, &NodeNotFoundError{name, api}
	}
	if val.local != nil {
		return val.local, nil
	}
	switch len(val.remote) {
	case 1:
		for _, result := range val.remote {
			return result, nil
		}
	case 0:
		return nil, &NodeNotFoundError{name, api}
	}
	return nil, &MultipleMatchingRemoteNodesError{
		Name:    name,
		API:     api,
		Remotes: slices.Collect(maps.Keys(val.remote)),
	}
}

func (s graphStorage) All() iter.Seq2[Name, *GraphNode] {
	return maps.All(s.nodes)
}

func (s graphStorage) Keys() iter.Seq[Name] {
	return maps.Keys(s.nodes)
}

func (s graphStorage) Values() iter.Seq[*GraphNode] {
	return maps.Values(s.nodes)
}

func (s graphStorage) Len() int {
	return len(s.nodes)
}

type simpleNameKey struct {
	name string
	api  API
}

type simpleNameVal struct {
	local  *GraphNode
	remote map[string]*GraphNode
}

type simpleNameCache map[simpleNameKey]*simpleNameVal

type resourceDependencies map[Name]graphNodes

type transitiveClosureMatrix map[Name]map[Name]int

// Graph The Graph maintains a collection of resources and their dependencies between each other.
type Graph struct {
	mu                      sync.RWMutex
	nodes                   graphStorage
	children                resourceDependencies
	parents                 resourceDependencies
	transitiveClosureMatrix transitiveClosureMatrix
	// logicalClock keeps track of updates to the graph. Each GraphNode has a
	// pointer to this logicalClock. Whenever SwapResource is called on a node
	// (the resource updates), the logicalClock is incremented.
	logicalClock *atomic.Int64
	logger       logging.Logger
	ftdc         *ftdc.FTDC
}

// NewGraph creates a new resource graph.
func NewGraph(logger logging.Logger) *Graph {
	return &Graph{
		children: resourceDependencies{},
		parents:  resourceDependencies{},
		nodes: graphStorage{
			nodes:           graphNodes{},
			simpleNameCache: simpleNameCache{},
		},
		transitiveClosureMatrix: transitiveClosureMatrix{},
		logicalClock:            &atomic.Int64{},
		logger:                  logger,
	}
}

// NewGraphWithFTDC creates a new resource graph with ftdc.
func NewGraphWithFTDC(logger logging.Logger, ftdc *ftdc.FTDC) *Graph {
	ret := NewGraph(logger)
	ret.ftdc = ftdc
	return ret
}

// CurrLogicalClockValue returns current the logical clock value.
func (g *Graph) CurrLogicalClockValue() int64 {
	return g.logicalClock.Load()
}

func (g *Graph) getAllChildrenOf(node Name) []Name {
	children := g.children[node]
	out := make([]Name, 0, len(children))
	for childName := range children {
		out = append(out, childName)
	}
	return out
}

func (g *Graph) getAllParentOf(node Name) []Name {
	parents := g.parents[node]
	out := make([]Name, 0, len(parents))
	for parentName := range parents {
		out = append(out, parentName)
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

	for node := range g.nodes.Keys() {
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
		nodes:                   g.nodes.Copy(),
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
func (g *Graph) Node(name Name) (*GraphNode, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	rNode, ok := g.nodes.Get(name)
	return rNode, ok
}

// UpdateNodePrefix sets the prefix for the node matching `name`.
func (g *Graph) UpdateNodePrefix(name Name, prefix string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	node, _ := g.nodes.Get(name)
	if node == nil {
		return
	}
	prevPrefix := node.GetPrefix()
	if prevPrefix == prefix {
		return
	}
	node.setPrefix(prefix)
	g.nodes.UpdateSimpleName(name, prevPrefix, node)
}

// FindBySimpleNameAndAPI returns a single graph node based on a simple name string and an
// API. It returns an error in the case that no matching node is found or multiple remote
// nodes are found.
func (g *Graph) FindBySimpleNameAndAPI(name string, api API) (*GraphNode, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nodes.FindBySimpleNameAndAPI(name, api)
}

// Names returns all the resource graph names.
func (g *Graph) Names() []Name {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.AppendSeq(
		make([]Name, 0, g.nodes.Len()),
		g.nodes.Keys(),
	)
}

func seq2First[K, V any](seq iter.Seq2[K, V]) (K, V, bool) {
	var resK K
	var resV V
	var ok bool
	seq(func(k K, v V) bool {
		resK = k
		resV = v
		ok = true
		return false
	})
	return resK, resV, ok
}

// SimpleNamesWhere returns a list of resource names with any configured remote
// prefix applied. Names are only included in the return if filter returns true.
func (g *Graph) SimpleNamesWhere(filter func(Name, *GraphNode) bool) []Name {
	g.mu.Lock()
	defer g.mu.Unlock()
	var result []Name
	for k, v := range g.nodes.simpleNameCache {
		if v.local == nil && len(v.remote) != 1 {
			continue
		}
		name := Name{
			API:  k.api,
			Name: k.name,
		}
		if v.local != nil {
			if !filter(name, v.local) {
				continue
			}
		} else {
			remName, remNode, _ := seq2First(maps.All(v.remote))
			name.Remote = remName
			if !filter(name, remNode) {
				continue
			}
		}
		result = append(result, name)
	}
	return result
}

// ReachableNames returns the all resource graph names, excluding remote resources that are unreached.
func (g *Graph) ReachableNames() []Name {
	g.mu.Lock()
	defer g.mu.Unlock()
	names := make([]Name, 0, g.nodes.Len())
	for nodeName, node := range g.nodes.All() {
		if node.unreachable {
			continue
		}
		names = append(names, nodeName)
	}
	return names
}

// FindNodesByShortNameAndAPI will look for resources matching both the API and the name.
func (g *Graph) FindNodesByShortNameAndAPI(name Name) []Name {
	g.mu.Lock()
	defer g.mu.Unlock()
	var ret []Name
	for k, v := range g.nodes.All() {
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
	for k := range g.nodes.Keys() {
		if k.API == api {
			ret = append(ret, k)
		}
	}
	return ret
}

// FindBySimpleName returns all unmodified resource names that match the
// provided string after applying any remote prefixes. This means that while
// the "name" argument to this function should include remote prefix(es), the
// Name.Name field of the return value: will not include remote prefix(es).
func (g *Graph) FindBySimpleName(name string) []Name {
	var result []Name
	for key, val := range g.nodes.simpleNameCache {
		if name != key.name || !(key.api.IsComponent() || key.api.IsService()) {
			continue
		}
		if val.local != nil {
			result = append(result, Name{API: key.api, Name: name})
			continue
		}
		for remote, node := range val.remote {
			result = append(result, Name{
				API:    key.api,
				Name:   strings.Replace(name, node.prefix, "", 1),
				Remote: remote,
			})
		}
	}
	return result
}

// GetAllChildrenOf returns all direct children of a node.
func (g *Graph) GetAllChildrenOf(node Name) []Name {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.getAllChildrenOf(node)
}

// GetAllParentsOf returns all parents of a given node.
func (g *Graph) GetAllParentsOf(node Name) []Name {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.getAllParentOf(node)
}

func (g *Graph) addNode(name Name, node *GraphNode) error {
	if node == nil {
		g.logger.Errorw("addNode called with a nil value; setting to uninitialized", "name", name)
		node = NewUninitializedNode()
	}
	if val, ok := g.nodes.Get(name); ok {
		if !val.IsUninitialized() {
			return errors.Errorf("initialized node already exists with name %q; must swap instead", name)
		}
		prevPrefix := val.prefix
		if err := val.replace(node); err != nil {
			return err
		}
		g.nodes.UpdateSimpleName(name, prevPrefix, val)
		return nil
	}
	node.setGraphLogicalClock(g.logicalClock)
	if g.ftdc != nil {
		g.ftdc.Add(name.String(), node)
	}
	g.nodes.Set(name, node)

	if _, ok := g.transitiveClosureMatrix[name]; !ok {
		g.transitiveClosureMatrix[name] = map[Name]int{}
	}
	for n := range g.nodes.Keys() {
		for v := range g.transitiveClosureMatrix {
			if _, ok := g.transitiveClosureMatrix[n][v]; !ok {
				g.transitiveClosureMatrix[n][v] = 0
			}
		}
	}
	g.transitiveClosureMatrix[name][name] = 1
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
		return errors.Errorf("%v cannot depend on itself", child.Name)
	}
	// Maybe we haven't encountered yet the parent so let's add it here and assign an uninitialized node
	if _, ok := g.nodes.Get(parent); !ok {
		if err := g.addNode(parent, NewUninitializedNode()); err != nil {
			return err
		}
	} else if g.transitiveClosureMatrix[parent][child] != 0 {
		return errors.Errorf("circular dependency - %v already depends on %v", parent.Name, child.Name)
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
	g.nodes.Delete(node)
	if g.ftdc != nil {
		g.ftdc.Remove(node.String())
	}
}

// MarkForRemoval marks the given graph for removal at a later point
// by RemoveMarked.
func (g *Graph) MarkForRemoval(toMark *Graph) {
	toMark.mu.Lock()
	defer toMark.mu.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()

	for v := range toMark.nodes.Values() {
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
		rNode, ok := g.nodes.Get(name)
		if !ok {
			// will never happen
			g.logger.Errorw("invariant: expected to find node during removal", "name", name)
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
		if i, ok := g.nodes.Get(node); ok && i != nil {
			g.remove(node)
		}
		toAddNode, _ := toAdd.nodes.Get(node)
		if err := g.addNode(node, toAddNode); err != nil {
			return err
		}

		childrenToAdd := toAdd.getAllChildrenOf(node)
		for _, childName := range childrenToAdd {
			if err := g.addChild(childName, node); err != nil {
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
	if _, ok := g.nodes.Get(node); !ok {
		return errors.Errorf("cannot copy parents to non existing node %q", node.Name)
	}

	// Clear cached values
	for k := range g.parents[node] {
		g.removeTransitiveClosure(node, k)
	}
	for parentName, vertice := range g.parents {
		if _, ok := vertice[node]; ok {
			removeNodeFromNodeMap(g.parents, parentName, node)
		}
	}

	// For each child of `parentName` in the `other` graph, add a corresponding child into this graph
	otherChildren := other.getAllChildrenOf(node)
	for _, childName := range otherChildren {
		if err := g.addChild(childName, node); err != nil {
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
	if r, ok := origin.nodes.Get(node); ok {
		if err := g.addNode(node, r); err != nil {
			return err
		}
		children := origin.getAllChildrenOf(node)
		for _, childName := range children {
			if _, ok := g.nodes.Get(childName); !ok {
				if err := g.addNode(childName, NewUninitializedNode()); err != nil {
					return err
				}
			}
			if err := g.addChild(childName, node); err != nil {
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

// ReverseTopologicalSortInLevels returns a slice of node Name groups, ordered such that
// all node names only depend on node names in a prior group.
func (g *Graph) ReverseTopologicalSortInLevels() [][]Name {
	ordered := g.TopologicalSortInLevels()
	for i, j := 0, len(ordered)-1; i < j; i, j = i+1, j-1 {
		ordered[i], ordered[j] = ordered[j], ordered[i]
	}
	return ordered
}

// ResolveDependencies attempts to link up unresolved dependencies after
// new changes to the graph.
func (g *Graph) ResolveDependencies(logger logging.Logger) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var allErrs error
	for nodeName, node := range g.nodes.All() {
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
				nodeNames := g.FindBySimpleName(dep)
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
						errors.Errorf("conflicting names for resource %q: %v", nodeName, NamesToStrings(nodeNames)))
					logger.Errorw(
						"cannot resolve dependency for resource due to multiple matching names",
						"name", nodeName,
						"conflicts", NamesToStrings(nodeNames),
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
	if _, ok := g.nodes.Get(node); !ok {
		return false
	}
	if _, ok := g.nodes.Get(child); !ok {
		return false
	}
	return g.transitiveClosureMatrix[child][node] != 0
}

// SubGraphFrom returns a Sub-Graph containing all linked dependencies starting with node [Name].
func (g *Graph) SubGraphFrom(node Name) (*Graph, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.subGraphFromWithMutex(node)
}

// subGraphFrom returns a Sub-Graph containing all linked dependencies starting with node [Name].
// This method is NOT threadsafe: A client must hold [Graph.mu] while calling this method.
func (g *Graph) subGraphFromWithMutex(node Name) (*Graph, error) {
	if _, ok := g.nodes.Get(node); !ok {
		return nil, errors.Errorf("cannot create sub-graph from non existing node %v ", node.Name)
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

// MarkReachability marks all nodes in the subgraph from the given [Name] node as either reachable [true] or unreachable [false].
func (g *Graph) MarkReachability(node Name, reachable bool) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	subGraph, err := g.subGraphFromWithMutex(node)
	if err != nil {
		return err
	}
	for node := range subGraph.nodes.Values() {
		node.markReachability(reachable)
	}
	return nil
}

// Status returns a slice of all graph node statuses.
func (g *Graph) Status() []NodeStatus {
	g.mu.Lock()
	defer g.mu.Unlock()

	var result []NodeStatus
	for k, v := range g.nodes.simpleNameCache {
		if v.local != nil {
			status := v.local.Status()
			status.Name = Name{
				API:  k.api,
				Name: k.name,
			}
			result = append(result, status)
		}
		for remote, v := range v.remote {
			status := v.Status()
			status.Name = Name{
				API:    k.api,
				Name:   k.name,
				Remote: remote,
			}
			result = append(result, status)
		}
	}
	return result
}
