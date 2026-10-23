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

// NodeNotFoundError is returned when the name and API combination are not found by
// FindBySimpleNameAndAPI, or when a simple name is not found by FindBySimpleName.
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

// MultipleMatchingNamesError is returned by FindBySimpleName when a simple name is claimed by more
// than one resource (multiple locals, or multiple remotes with no local) so no single owner can be
// resolved under machine-wide name uniqueness. Matches holds every colliding name.
type MultipleMatchingNamesError struct {
	Name    string
	Matches []Name
}

func (e *MultipleMatchingNamesError) Error() string {
	return fmt.Sprintf("found multiple resources matching name %q: %v", e.Name, NamesToStrings(e.Matches))
}

// IsMultipleMatchingNamesError returns if the given error is any kind of multiple matching names error.
func IsMultipleMatchingNamesError(err error) bool {
	var mmnErr *MultipleMatchingNamesError
	return errors.As(err, &mmnErr)
}

type graphNodes map[Name]*GraphNode

// graphStorage is used by [Graph] to store the collection of nodes. It
// consists of
// - a map of [Name] to *[GraphNode]s, used to serve most queries
// - a simpleNameIndex, used to serve [Graph.FindBySimpleNameAndAPI] (simpleName, api) lookups
// The various methods on graphStorage handle keeping these in sync.
// graphStorage itself is _not_ thread safe. and depends on [Graph] to handle
// synchronization between goroutines.
type graphStorage struct {
	nodes       graphNodes
	simpleNames simpleNameIndex
}

// compositeAPIsForNode returns the co-equal API set the node's composite serves, or nil if it is not a
// composite.
func compositeAPIsForNode(node *GraphNode) []API {
	if node == nil {
		return nil
	}
	// The built MultiAPIResource's own API set is authoritative for any composite once built. Fall back
	// to the configured model for a configured-but-unbuilt local composite: its instance doesn't exist
	// yet, but its model is known. A remote composite never needs the fallback — its client exists as
	// soon as the node does, and its model is "unknown" anyway, so it always uses the instance path.
	if res, err := node.UnsafeResource(); err == nil {
		if mar, ok := res.(MultiAPIResource); ok {
			return mar.APIs()
		}
	}
	return APIsForModel(node.Config().Model)
}

// simpleNameIndex resolves (simpleName, api) to a node. byName keys a node under the one api it is
// registered as — its configured api locally, its canonical (sorted-first) api when remote — and
// composite keys a composite's other co-equal apis, which byName does not carry. A remote prefix
// separates local nodes and remotes with the same simple name.
type simpleNameIndex struct {
	byName    simpleNameCache
	composite simpleNameCache
}

func newSimpleNameIndex() simpleNameIndex {
	return simpleNameIndex{byName: simpleNameCache{}, composite: simpleNameCache{}}
}

// putSimpleName records node under key in cache, as the local node or a remote one.
func putSimpleName(cache simpleNameCache, key simpleNameKey, remote string, node *GraphNode) {
	val := cache[key]
	if val == nil {
		val = &simpleNameVal{remote: map[string]*GraphNode{}}
		cache[key] = val
	}
	if remote == "" {
		val.local = node
	} else {
		val.remote[remote] = node
	}
}

// set records node under its own (simpleName, api) key.
func (ix simpleNameIndex) set(name Name, node *GraphNode) {
	putSimpleName(ix.byName, simpleNameKey{node.prefix + name.Name, name.API}, name.Remote, node)
}

// indexComposite records a composite node under each of its non-configured co-equal API keys. No-op
// for an ordinary (single-API) resource.
func (ix simpleNameIndex) indexComposite(name Name, node *GraphNode) {
	apis := compositeAPIsForNode(node)
	if len(apis) < 2 {
		return
	}
	for _, api := range apis {
		if api == name.API {
			continue
		}
		putSimpleName(ix.composite, simpleNameKey{node.prefix + name.Name, api}, name.Remote, node)
	}
}

// dropComposite removes every composite reference to node, dropping an entry that ends up empty.
func (ix simpleNameIndex) dropComposite(node *GraphNode) {
	for k, val := range ix.composite {
		if val.local == node {
			val.local = nil
		}
		for remote, v := range val.remote {
			if v == node {
				delete(val.remote, remote)
			}
		}
		if val.local == nil && len(val.remote) == 0 {
			delete(ix.composite, k)
		}
	}
}

// reindexComposite rewrites node's composite entries from its current model, dropping stale ones first.
// The caller must hold the graph lock.
func (ix simpleNameIndex) reindexComposite(name Name, node *GraphNode) {
	ix.dropComposite(node)
	ix.indexComposite(name, node)
}

// updatePrefix moves node's byName entry from its previous prefix to its current one.
func (ix simpleNameIndex) updatePrefix(name Name, prevPrefix string, node *GraphNode) {
	if prevPrefix == node.prefix {
		return
	}
	prevKey := simpleNameKey{prevPrefix + name.Name, name.API}
	if prevVal := ix.byName[prevKey]; prevVal != nil {
		if name.Remote == "" {
			prevVal.local = nil
		} else {
			delete(prevVal.remote, name.Remote)
		}
	}
	ix.set(name, node)
}

// delete removes node from both byName and the composite index.
func (ix simpleNameIndex) delete(name Name, node *GraphNode) {
	ix.dropComposite(node)
	key := simpleNameKey{node.prefix + name.Name, name.API}
	existing := ix.byName[key]
	if existing == nil {
		return
	}
	if name.Remote == "" {
		existing.local = nil
		return
	}
	delete(existing.remote, name.Remote)
}

// find resolves one (simpleName, api) to a node. A byName match and a composite match for the same key
// both count: a local from either wins, otherwise the remote matches from both are merged and the usual
// ambiguity rule applies, so a composite can't silently mask an equally-valid remote.
func (ix simpleNameIndex) find(name string, api API) (*GraphNode, error) {
	key := simpleNameKey{name, api}
	direct := ix.byName[key]
	composite := ix.composite[key]
	if direct == nil && composite == nil {
		return nil, &NodeNotFoundError{name, api}
	}
	if local := localNode(direct, composite); local != nil {
		return local, nil
	}
	remotes := mergeRemoteNodes(direct, composite)
	switch len(remotes) {
	case 1:
		for _, result := range remotes {
			return result, nil
		}
	case 0:
		return nil, &NodeNotFoundError{name, api}
	}
	return nil, &MultipleMatchingRemoteNodesError{
		Name:    name,
		API:     api,
		Remotes: slices.Collect(maps.Keys(remotes)),
	}
}

// localNode returns the first non-nil local node among the given simpleNameVals.
func localNode(vals ...*simpleNameVal) *GraphNode {
	for _, v := range vals {
		if v != nil && v.local != nil {
			return v.local
		}
	}
	return nil
}

// mergeRemoteNodes unions the remote-node maps of the given simpleNameVals, keyed by remote name.
func mergeRemoteNodes(vals ...*simpleNameVal) map[string]*GraphNode {
	out := map[string]*GraphNode{}
	for _, v := range vals {
		if v == nil {
			continue
		}
		for remote, node := range v.remote {
			out[remote] = node
		}
	}
	return out
}

// all iterates the byName entries (a node under its own key). The composite index is private; callers
// that need a composite's other APIs expand them from the node.
func (ix simpleNameIndex) all() iter.Seq2[simpleNameKey, *simpleNameVal] {
	return maps.All(ix.byName)
}

func (ix simpleNameIndex) copy() simpleNameIndex {
	out := newSimpleNameIndex()
	for k, v := range ix.byName {
		out.byName[k] = &simpleNameVal{local: v.local, remote: maps.Clone(v.remote)}
	}
	for k, v := range ix.composite {
		out.composite[k] = &simpleNameVal{local: v.local, remote: maps.Clone(v.remote)}
	}
	return out
}

// reindexCompositeAPIs rewrites node's composite index entries. The caller must hold the graph lock.
func (s graphStorage) reindexCompositeAPIs(name Name, node *GraphNode) {
	s.simpleNames.reindexComposite(name, node)
}

func (s graphStorage) Get(name Name) (*GraphNode, bool) {
	node, ok := s.nodes[name]
	return node, ok
}

func (s graphStorage) Set(name Name, node *GraphNode) {
	s.nodes[name] = node
	s.simpleNames.set(name, node)
}

func (s graphStorage) UpdateSimpleName(name Name, prevPrefix string, node *GraphNode) {
	s.simpleNames.updatePrefix(name, prevPrefix, node)
}

func (s graphStorage) Delete(name Name) {
	node := s.nodes[name]
	delete(s.nodes, name)
	if node == nil {
		return
	}
	s.simpleNames.delete(name, node)
}

func (s graphStorage) Copy() graphStorage {
	return graphStorage{
		nodes:       maps.Clone(s.nodes),
		simpleNames: s.simpleNames.copy(),
	}
}

func (s graphStorage) FindBySimpleNameAndAPI(name string, api API) (*GraphNode, error) {
	return s.simpleNames.find(name, api)
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
			nodes:       graphNodes{},
			simpleNames: newSimpleNameIndex(),
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
	g.mu.RLock()
	defer g.mu.RUnlock()
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
	g.mu.RLock()
	defer g.mu.RUnlock()
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
	g.mu.RLock()
	defer g.mu.RUnlock()
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
	// The composite index keys include the prefix, so rebuild it under the new one.
	g.nodes.reindexCompositeAPIs(name, node)
}

// FindBySimpleNameAndAPI returns a single graph node based on a simple name string and an
// API. It returns an error in the case that no matching node is found or multiple remote
// nodes are found.
func (g *Graph) FindBySimpleNameAndAPI(name string, api API) (*GraphNode, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nodes.FindBySimpleNameAndAPI(name, api)
}

// ReindexComposite refreshes name's node's compositeByAPI entries after an in-place model change. That
// goes through SwapResource and bypasses Set/UpdateSimpleName, so the index would otherwise keep stale
// entries. It reads the node's current model, so call it after the swap. No-op for a missing node or a
// single-API resource.
func (g *Graph) ReindexComposite(name Name) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if node, ok := g.nodes.Get(name); ok {
		g.nodes.reindexCompositeAPIs(name, node)
	}
}

// Names returns all the resource graph names.
func (g *Graph) Names() []Name {
	g.mu.RLock()
	defer g.mu.RUnlock()
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
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.simpleNamesWhere(filter)
}

// simpleNamesWhere is [Graph.SimpleNamesWhere] without locking. Caller must hold g.mu.
func (g *Graph) simpleNamesWhere(filter func(Name, *GraphNode) bool) []Name {
	type candidate struct {
		name Name
		node *GraphNode
	}
	var result []Name
	emit := func(c candidate) {
		if filter(c.name, c.node) {
			result = append(result, c.name)
		}
	}

	// Group candidates by simple name to enforce name uniqueness across APIs.
	byName := make(map[string][]candidate)
	for k, v := range g.nodes.simpleNames.all() {
		if v.local == nil && len(v.remote) != 1 {
			continue
		}
		c := candidate{name: Name{API: k.api, Name: k.name}}
		if v.local != nil {
			c.node = v.local
		} else {
			remName, remNode, _ := seq2First(maps.All(v.remote))
			c.name.Remote = remName
			c.node = remNode
		}

		// Name uniqueness covers only components and services. Other nodes — e.g. a remote-machine
		// connection, whose name is a prefix, not a resource name — skip grouping and pass straight
		// to the filter.
		if !(k.api.IsComponent() || k.api.IsService()) {
			emit(c)
			continue
		}
		byName[k.name] = append(byName[k.name], c)
	}

	for _, cands := range byName {
		chosen := cands[0]
		if len(cands) > 1 {
			// A local resource wins over remotes claiming the same name. If there is not
			// exactly one local claimant (e.g. remote resources collide), the name is hidden.
			var localCands []candidate
			for _, c := range cands {
				if c.name.Remote == "" {
					localCands = append(localCands, c)
				}
			}
			if len(localCands) != 1 {
				continue
			}
			chosen = localCands[0]
		}
		emit(chosen)
	}
	return result
}

// CollidingNames returns every local component/service whose simple name is shared by more than one
// local resource across APIs. Callers tear these down to keep every simple name unique machine-wide.
//
// Nothing is exempt: rdk-internal services use reserved "$"-prefixed names that config validation
// forbids for user resources, and "builtin" is reserved for the default services at config validation,
// so neither can ever be the duplicate a user introduces.
//
// This counts only local nodes. A remote resource is never torn down here: when it shares a name
// with a local one the local wins and the remote is hidden (not removed) by [Graph.SimpleNamesWhere].
// Remote-machine connection nodes are ignored as well.
func (g *Graph) CollidingNames() []Name {
	g.mu.RLock()
	defer g.mu.RUnlock()

	byName := make(map[string][]Name)
	for k, v := range g.nodes.simpleNames.all() {
		if v.local == nil {
			continue
		}
		// Nodes already marked for removal (e.g. a resource dropped by the incoming config) are on
		// their way out and must not be counted — otherwise a departing resource would spuriously
		// collide with the incoming resource that replaces its name.
		if v.local.MarkedForRemoval() {
			continue
		}
		if !(k.api.IsComponent() || k.api.IsService()) {
			continue
		}
		byName[k.name] = append(byName[k.name], Name{API: k.api, Name: k.name})
	}

	var colliding []Name
	for _, names := range byName {
		if len(names) > 1 {
			colliding = append(colliding, names...)
		}
	}
	return colliding
}

// ExpandCompositeNames replaces each composite name with one Name per co-equal API it serves, so a
// composite (stored as one node under its canonical API) is advertised as N same-named ResourceName
// messages, one per API. This is what lets a client detect and assemble a composite. Non-composite
// names, and names with no backing node, pass through unchanged.
func (g *Graph) ExpandCompositeNames(names []Name) []Name {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []Name
	for _, n := range names {
		node, ok := g.nodes.Get(n)
		if !ok {
			out = append(out, n)
			continue
		}
		apis := compositeAPIsForNode(node)
		if len(apis) < 2 {
			out = append(out, n)
			continue
		}
		for _, api := range apis {
			out = append(out, Name{API: api, Remote: n.Remote, Name: n.Name})
		}
	}
	return out
}

// ReachableNames returns the all resource graph names, excluding remote resources that are unreached.
func (g *Graph) ReachableNames() []Name {
	g.mu.RLock()
	defer g.mu.RUnlock()
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
	g.mu.RLock()
	defer g.mu.RUnlock()
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
	g.mu.RLock()
	defer g.mu.RUnlock()
	var ret []Name
	for k := range g.nodes.Keys() {
		if k.API == api {
			ret = append(ret, k)
		}
	}
	return ret
}

// FindBySimpleName resolves the given simple name to the single resource that owns it under
// machine-wide name uniqueness: a local resource wins over same-named remotes. It returns a
// NodeNotFoundError when nothing matches and a MultipleMatchingNamesError when the name has
// collisions (more than one local claimant, or no local claimant with more than one remote
// claimant). The "name" argument should include any remote prefix(es); the Name.Name field
// of the return value will not.
//
// Internal resources ("rdk-internal" namespace) are ignored because none of them besides the
// framesystem have exposed grpc methods. In any case, the framesystem is expected to be called for
// differently.
//
// Use [Graph.FindAllBySimpleName] instead when every matching name is needed (e.g. to enumerate
// collisions for logging) rather than the single resolved winner.
func (g *Graph) FindBySimpleName(name string) (Name, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.findBySimpleName(name)
}

// findBySimpleName is the unlocked implementation of [Graph.FindBySimpleName]. Callers must hold
// g.mu.
func (g *Graph) findBySimpleName(name string) (Name, error) {
	matches := g.namesMatchingSimpleName(name)
	var locals, remotes []Name
	for _, n := range matches {
		if n.Remote == "" {
			locals = append(locals, n)
		} else {
			remotes = append(remotes, n)
		}
	}
	switch {
	case len(locals) == 1:
		// A local resource wins its simple name over any same-named remotes.
		return locals[0], nil
	case len(locals) == 0 && len(remotes) == 1:
		return remotes[0], nil
	case len(matches) == 0:
		return Name{}, &NodeNotFoundError{Name: name}
	default:
		// More than one local claimant, or no local claimant with more than one remote claimant:
		// the name has no single owner.
		return Name{}, &MultipleMatchingNamesError{Name: name, Matches: matches}
	}
}

// FindAllBySimpleName returns every unmodified resource name that matches the provided string
// after applying any remote prefixes. The "name" argument should include any remote prefix(es);
// the Name.Name field of the return value will not. Unlike [Graph.FindBySimpleName] it does not
// apply machine-wide name-uniqueness resolution, so it may return several names sharing a simple
// name; use it where all matches are needed (e.g. to enumerate collisions for logging).
//
// Internal resources ("rdk-internal" namespace) are ignored because none of them besides the
// framesystem have exposed grpc methods. In any case, the framesystem is expected to be called for
// differently.
func (g *Graph) FindAllBySimpleName(name string) []Name {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.namesMatchingSimpleName(name)
}

// namesMatchingSimpleName is the unlocked all-matches scan backing [Graph.FindAllBySimpleName].
// Callers must hold g.mu.
func (g *Graph) namesMatchingSimpleName(name string) []Name {
	var result []Name
	for key, val := range g.nodes.simpleNames.all() {
		if key.api.Type.Namespace == APINamespaceRDKInternal {
			continue
		}
		if name != key.name || !(key.api.IsComponent() || key.api.IsService()) {
			continue
		}
		if val.local != nil && !val.local.MarkedForRemoval() {
			result = append(result, Name{API: key.api, Name: name})
			continue
		}
		for remote, node := range val.remote {
			// Skip nodes marked for removal. When a remote resource's advertised API under a name changes
			// (a composite's canonical API flips, a composite <-> single-API transition, or a resource
			// swapping its API), the old node is marked for removal but lingers under the same remote+name
			// as the new node until a later sweep. Excluding it leaves the live node the sole match, not a
			// same-remote collision; a name shared across different remotes stays a collision.
			if node.MarkedForRemoval() {
				continue
			}
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
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.getAllChildrenOf(node)
}

// GetAllParentsOf returns all parents of a given node.
func (g *Graph) GetAllParentsOf(node Name) []Name {
	g.mu.RLock()
	defer g.mu.RUnlock()
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
		oldFTDCName := LogName(name, compositeAPIsForNode(val))
		if err := val.replace(node); err != nil {
			return err
		}
		g.nodes.UpdateSimpleName(name, prevPrefix, val)
		// Index now: replacing an uninitialized placeholder with its configured node is where the model,
		// and thus the co-equal API set, first becomes known.
		g.nodes.reindexCompositeAPIs(name, val)
		// The model just became known, so the node may now be a composite. Its FTDC section was added
		// under the placeholder's label, so re-key it if the label changed.
		if g.ftdc != nil {
			if newFTDCName := LogName(name, compositeAPIsForNode(val)); newFTDCName != oldFTDCName {
				g.ftdc.Remove(oldFTDCName)
				g.ftdc.Add(newFTDCName, val)
			}
		}
		return nil
	}
	node.setGraphLogicalClock(g.logicalClock)
	if g.ftdc != nil {
		g.ftdc.Add(LogName(name, compositeAPIsForNode(node)), node)
	}
	g.nodes.Set(name, node)
	g.nodes.reindexCompositeAPIs(name, node)

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
	gn, _ := g.nodes.Get(node)
	g.nodes.Delete(node)
	if g.ftdc != nil {
		g.ftdc.Remove(LogName(node, compositeAPIsForNode(gn)))
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
//
// Note: This only resolves dependencies a node still lists as unresolved. Once a
// node resolves a dependency, the requirement lives on solely as a graph edge, so
// if that edge is later removed (e.g. the dependency is removed) without the dependent
// being removed or re-marking the dependency as unresolved, this pass will not rebuild
// the edge even if the dependency is later re-added. We guard on resolving a dependency
// whose node is already marked for removal, so that a dependent node will stay pending
// instead of building an edge that is about to be stripped, which prevents the one
// known way of reaching this state. Other unknown paths may still reach this state, and
// this pass would not recover a dependent stranded that way.
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
					allErrs = multierr.Combine(allErrs, errors.Errorf("node cannot depend on itself: %q", nodeName))
					logger.Errorw("node cannot depend on itself", "name", nodeName)
					return Name{}, false
				}
				if resName, err := NewFromString(dep); err == nil {
					return resName, true
				}

				// if a name is later added that conflicts, it will not
				// necessarily be caught unless the resource config changes.
				resolved, err := g.findBySimpleName(dep)
				if err != nil {
					var multiErr *MultipleMatchingNamesError
					if errors.As(err, &multiErr) {
						allErrs = multierr.Combine(
							allErrs,
							errors.Errorf("conflicting names for resource %q: %v", nodeName, NamesToStrings(multiErr.Matches)),
						)
						logger.Errorw(
							"cannot resolve dependency for resource due to multiple matching names",
							"name", nodeName,
							"conflicts", NamesToStrings(multiErr.Matches),
						)
					}
					return Name{}, false
				}
				if resolved.String() == nodeName.String() {
					allErrs = multierr.Combine(allErrs, errors.Errorf("node cannot depend on itself: %q", nodeName))
					logger.Errorw("node cannot depend on itself", "name", nodeName)
					return Name{}, false
				}
				logger.Debugw(
					"dependency resolved for resource",
					"name", nodeName,
					"dependency", resolved,
				)
				return resolved, true
			}

			resolvedName, resolved := tryResolve()
			if resolved {
				// Don't resolve a dependency whose node is already marked for removal. The node
				// is still present (removal happens later), but building an edge to it now would
				// be stripped when it is removed, leaving this resource resolved with no edge and
				// no way to notice. Instead, leave the dependency unresolved so it is re-resolved
				// on a later pass once a live node with that name exists.
				if depNode, ok := g.nodes.Get(resolvedName); ok && depNode.MarkedForRemoval() {
					resolved = false
				} else if err := g.addChild(nodeName, resolvedName); err != nil {
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
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.subGraphFromWithMutex(node)
}

// dependentsWithMutex returns node and every node that depends on it, directly or
// transitively, read from the transitive closure. This method is NOT threadsafe: A client
// must read-lock [Graph.mu] while calling this method.
func (g *Graph) dependentsWithMutex(node Name) (map[Name]*GraphNode, error) {
	if _, ok := g.nodes.Get(node); !ok {
		return nil, errors.Errorf("cannot create sub-graph from non existing node %v ", node.Name)
	}
	dependents := map[Name]*GraphNode{}
	for name, gNode := range g.nodes.All() {
		if g.transitiveClosureMatrix[name][node] != 0 {
			dependents[name] = gNode
		}
	}
	return dependents, nil
}

// subGraphFrom returns a Sub-Graph containing all linked dependencies starting with node
// [Name]. This method is NOT threadsafe: A client must read-lock [Graph.mu] while calling
// this method.
//
// The sub-graph holds node and its dependents, the same *GraphNode values as g, and the
// edges and transitive closure among them. Every path between two of those nodes stays
// among them, so the closure is the same as g's. It is built from just those nodes rather
// than by removing every other node from a clone of g, which costs a transitive closure
// update per removed edge.
func (g *Graph) subGraphFromWithMutex(node Name) (*Graph, error) {
	dependents, err := g.dependentsWithMutex(node)
	if err != nil {
		return nil, err
	}
	subGraph := &Graph{
		children:                resourceDependencies{},
		nodes:                   g.nodes.Copy(),
		parents:                 resourceDependencies{},
		transitiveClosureMatrix: make(transitiveClosureMatrix, len(dependents)),
		logicalClock:            g.logicalClock,
	}
	for name := range g.nodes.Keys() {
		if _, ok := dependents[name]; !ok {
			subGraph.nodes.Delete(name)
		}
	}
	for name := range dependents {
		// A dependent's children depend on it, so they are all dependents too.
		if children, ok := g.children[name]; ok {
			subGraph.children[name] = copyNodes(children)
		}
		for parent, edge := range g.parents[name] {
			if _, ok := dependents[parent]; ok {
				if subGraph.parents[name] == nil {
					subGraph.parents[name] = graphNodes{}
				}
				subGraph.parents[name][parent] = edge
			}
		}
		row := make(map[Name]int, len(dependents))
		for other := range dependents {
			row[other] = g.transitiveClosureMatrix[name][other]
		}
		subGraph.transitiveClosureMatrix[name] = row
	}
	return subGraph, nil
}

// MarkReachability marks the given remote-machine node and the remote resources depending on it as
// either reachable [true] or unreachable [false]. Local dependents are left alone; they remain callable.
func (g *Graph) MarkReachability(node Name, reachable bool) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	dependents, err := g.dependentsWithMutex(node)
	if err != nil {
		return err
	}
	for name, gNode := range dependents {
		if name == node || name.Remote != "" {
			gNode.markReachability(reachable)
		}
	}
	return nil
}

func reachability(n *GraphNode) Reachability {
	if n.IsReachable() {
		return ReachabilityReachable
	}
	return ReachabilityRemoteDisconnected
}

// Status returns a slice of all graph node statuses.
func (g *Graph) Status() []NodeStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visible := make(map[Name]bool)
	for _, n := range g.simpleNamesWhere(func(Name, *GraphNode) bool { return true }) {
		visible[n] = true
	}
	localNames := make(map[string]bool)
	for k, v := range g.nodes.simpleNames.all() {
		if v.local != nil && (k.api.IsComponent() || k.api.IsService()) {
			localNames[k.name] = true
		}
	}

	var result []NodeStatus
	// emit appends one status per API the node serves, so a composite yields one per co-equal API
	// (mirroring ExpandCompositeNames), all sharing the node's status and reachability. An ordinary
	// resource yields one.
	emit := func(name Name, node *GraphNode, reach Reachability) {
		status := node.Status()
		status.Reachability = reach
		apis := compositeAPIsForNode(node)
		if len(apis) < 2 {
			status.Name = name
			result = append(result, status)
			return
		}
		for _, api := range apis {
			s := status
			s.Name = Name{API: api, Name: name.Name, Remote: name.Remote}
			result = append(result, s)
		}
	}

	for k, v := range g.nodes.simpleNames.all() {
		if v.local != nil {
			emit(Name{API: k.api, Name: k.name}, v.local, reachability(v.local))
		}
		for remote, n := range v.remote {
			remoteResName := Name{API: k.api, Name: k.name, Remote: remote}
			var reach Reachability
			switch {
			case visible[remoteResName]:
				reach = reachability(n)
			case localNames[k.name]:
				reach = ReachabilityShadowedByLocal
			default:
				reach = ReachabilityAmbiguousRemote
			}
			emit(remoteResName, n, reach)
		}
	}
	return result
}
