package resource

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"go.viam.com/rdk/ftdc"
	"go.viam.com/rdk/logging"
)

//go:generate stringer -type NodeState -trimprefix NodeState
//
// The directive above configures `go generate ./...` to automatically generate a
// `String` method for the NodeState type.

// NodeState captures the configuration lifecycle state of a resource node.
type NodeState uint8

const (
	// NodeStateUnknown represents an unknown state.
	NodeStateUnknown NodeState = iota

	// NodeStateUnconfigured denotes a newly created resource.
	NodeStateUnconfigured

	// NodeStateConfiguring denotes a resource is being configured.
	NodeStateConfiguring

	// NodeStateReady denotes a resource that has been configured and is healthy.
	NodeStateReady

	// NodeStateRemoving denotes a resource is being removed from the resource graph.
	NodeStateRemoving

	// NodeStateUnhealthy denotes a resource is unhealthy.
	NodeStateUnhealthy
)

// A GraphNode contains the current state of a resource.
// Based on these states, the underlying Resource may or may not be available.
// Additionally, the node can be informed that the resource either needs to be
// updated or eventually removed. During its life, errors may be set on the
// node to indicate that the resource is no longer available to external users.
type GraphNode struct {
	// mu guards all fields below.
	mu sync.RWMutex

	// graphLogicalClock is a pointer to the Graph's logicalClock. It is
	// incremented every time any GraphNode calls SwapResource. SwapResource is called
	// whenever a resource is reconfigured.
	graphLogicalClock *atomic.Int64
	// updatedAt is the value of the graphLogicalClock when it was last
	// incremented by this GraphNode's SwapResource method. It is only referenced
	// in tests.
	updatedAt int64

	current      Resource
	currentModel Model
	config       Config
	// lastReconfigured returns a pointer to the time at which the resource within this
	// GraphNode was constructed or last reconfigured. It returns nil if the GraphNode is
	// unconfigured.
	lastReconfigured          *time.Time
	lastErr                   error
	unresolvedDependencies    []string
	needsDependencyResolution bool

	logger logging.Logger

	// state stores the current lifecycle state for a resource node.
	state NodeState
	// transitionedAt stores the timestamp of when resource entered its current lifecycle
	// state.
	transitionedAt time.Time

	// pendingRevision stores the next revision that will be applied to the graph node
	// once the underlying resource is successfully configured - that revision will be
	// stored the on the revision field.
	pendingRevision string
	revision        string

	// unreachable is an informational field that indicates if a resource on a remote
	// machine is disconnected.
	unreachable bool
}

var (
	errNotInitalized  = errors.New("resource not initialized yet")
	errPendingRemoval = errors.New("resource is pending removal")
)

// NewUninitializedNode returns a node that is brand new and not yet initialized.
func NewUninitializedNode() *GraphNode {
	return &GraphNode{
		state:          NodeStateUnconfigured,
		transitionedAt: time.Now(),
	}
}

// NewUnconfiguredGraphNode returns a node that contains enough information to
// construct the underlying resource.
func NewUnconfiguredGraphNode(config Config, dependencies []string) *GraphNode {
	node := NewUninitializedNode()
	node.SetNewConfig(config, dependencies)
	return node
}

// NewConfiguredGraphNode returns a node that is already configured with
// the supplied config and resource.
func NewConfiguredGraphNode(config Config, res Resource, resModel Model) *GraphNode {
	node := NewUninitializedNode()
	node.SetNewConfig(config, nil)
	node.setDependenciesResolved()
	node.SwapResource(res, resModel, nil)
	return node
}

// UpdatedAt returns the value of the logical clock when SwapResource was last
// called on this GraphNode (the resource was last updated). It's only used
// for tests.
func (w *GraphNode) UpdatedAt() int64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.updatedAt
}

func (w *GraphNode) setGraphLogicalClock(clock *atomic.Int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.graphLogicalClock = clock
}

// LastReconfigured returns a pointer to the time at which the resource within
// this GraphNode was constructed or last reconfigured. It returns nil if the
// GraphNode is unconfigured.
func (w *GraphNode) LastReconfigured() *time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastReconfigured
}

// Resource returns the underlying resource if it is not pending removal,
// has no error on it, and is initialized.
func (w *GraphNode) Resource() (Resource, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.state == NodeStateRemoving {
		return nil, errPendingRemoval
	}
	if w.lastErr != nil {
		return nil, w.lastErr
	}
	if w.current == nil {
		return nil, errNotInitalized
	}
	return w.current, nil
}

// State return the current lifecycle state for a resource node.
func (w *GraphNode) State() NodeState {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state
}

// TransitionedAt return the timestamp of when resource entered its current lifecycle
// state.
func (w *GraphNode) TransitionedAt() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.transitionedAt
}

// InitializeLogger initializes the logger object associated with this resource node.
func (w *GraphNode) InitializeLogger(parent logging.Logger, subname string) {
	logger := parent.Sublogger(subname)
	w.logger = logger
}

// Logger returns the logger object associated with this resource node. This is expected to be the logger
// passed into the `Constructor` when registering resources.
func (w *GraphNode) Logger() logging.Logger {
	return w.logger
}

// UnsafeResource always returns the underlying resource, if
// initialized, even if it is in an error state. This should
// only be called during reconfiguration.
func (w *GraphNode) UnsafeResource() (Resource, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.current == nil {
		return nil, errNotInitalized
	}
	return w.current, nil
}

// ResourceModel returns the current model that this resource is.
// This value should only be assumed to be associated with the current
// resource.
func (w *GraphNode) ResourceModel() Model {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.currentModel
}

// HasResource returns if calling Resource would result in no error.
func (w *GraphNode) HasResource() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state != NodeStateRemoving && w.lastErr == nil && w.current != nil
}

// IsUninitialized returns if this resource is in an uninitialized state.
func (w *GraphNode) IsUninitialized() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.current == nil
}

// UnsetResource unsets the current resource. This function does not clean up
// or close the resource and should be used carefully.
func (w *GraphNode) UnsetResource() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.current = nil
}

// SwapResource emplaces the new resource. It may be the same as before
// and expects the caller to close the old one. This is considered
// to be a working resource and as such we unmark it for removal
// and indicate it no longer needs reconfiguration. SwapResource also
// increments the graphLogicalClock and sets updatedAt for this GraphNode
// to the new value.
//
// The `ftdc` input may be nil (e.g: testing). If present, this will also updates FTDC to
// communicate that the `Stats` method may return different values. As we'll now be calling `Stats`
// on a potentially different underlying `Model`.
func (w *GraphNode) SwapResource(newRes Resource, newModel Model, ftdc *ftdc.FTDC) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.current = newRes
	w.currentModel = newModel
	w.revision = w.pendingRevision
	w.lastErr = nil
	w.transitionTo(NodeStateReady)

	// these should already be set
	w.unresolvedDependencies = nil
	w.needsDependencyResolution = false

	if w.graphLogicalClock != nil {
		w.updatedAt = w.graphLogicalClock.Add(1)
	}
	now := time.Now()
	w.lastReconfigured = &now

	// Calling `Stats` acquires the `GraphNode.mu` (via the `Resource` method). This allows us to
	// safely Remove->Add in one step instead of needing to Remove->SwapResource->Add.
	if ftdc != nil {
		// We call `Remove` followed by `Add` instead of only calling `Add`. Only calling `Add`
		// would result in a warning that we're overriding an existing ftdc "system".
		ftdc.Remove(newRes.Name().String())
		ftdc.Add(newRes.Name().String(), w)
	}
}

// MarkForRemoval marks this node for removal at a later time.
func (w *GraphNode) MarkForRemoval() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.transitionTo(NodeStateRemoving)
}

// MarkedForRemoval returns if this node is marked for removal.
func (w *GraphNode) MarkedForRemoval() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state == NodeStateRemoving
}

// LogAndSetLastError logs and sets the latest error on this node. This will cause the resource to
// become unavailable to external users of the graph. The resource manager may still access the
// underlying resource via UnsafeResource.
//
// The additional `args` should come in key/value pairs for structured logging.
func (w *GraphNode) LogAndSetLastError(err error, args ...any) {
	w.mu.Lock()
	w.lastErr = err
	w.transitionTo(NodeStateUnhealthy)
	w.mu.Unlock()

	if w.logger != nil {
		w.logger.Errorw(err.Error(), args...)
	}
}

// Config returns the current config that this resource is using.
// This value should only be assumed to be associated with the current
// resource.
func (w *GraphNode) Config() Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.config
}

// NeedsReconfigure returns whether or not this node needs reconfiguration
// performed on its underlying resource.
func (w *GraphNode) NeedsReconfigure() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// A resource can only become unhealthy during (re)configuration, so we can
	// assume that an unhealthy node always need to be reconfigured.
	return w.state == NodeStateConfiguring || w.state == NodeStateUnhealthy
}

// hasUnresolvedDependencies returns whether or not this node has any
// dependencies to be resolved (even if they are empty).
func (w *GraphNode) hasUnresolvedDependencies() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.needsDependencyResolution
}

func (w *GraphNode) setNeedsReconfigure(newConfig Config, mustReconfigure bool, dependencies []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !mustReconfigure && w.state == NodeStateRemoving {
		// This is the case where the node is being asked to update
		// with no new config but it was marked for removal otherwise.
		// The current system enforces us to remove since dependencies
		// have not changed.
		return
	}
	if mustReconfigure {
		w.needsDependencyResolution = true
	}
	w.config = newConfig
	w.transitionTo(NodeStateConfiguring)
	w.unresolvedDependencies = dependencies
}

// UpdatePendingRevision sets the next revision to be applied once the node is in a
// [NodeStateReady] state.
func (w *GraphNode) UpdatePendingRevision(revision string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.pendingRevision = revision
}

// UpdateRevision updates the node config revision if the node is in a [NodeStateReady]
// state.
func (w *GraphNode) UpdateRevision(revision string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.state == NodeStateReady {
		w.pendingRevision = revision
		w.revision = revision
	}
}

func (w *GraphNode) markReachability(reachable bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.unreachable = !reachable
}

// SetNewConfig is used to inform the node that it has been modified
// and requires a reconfiguration. If the node was previously marked for removal,
// this unmarks it.
func (w *GraphNode) SetNewConfig(newConfig Config, dependencies []string) {
	w.setNeedsReconfigure(newConfig, true, dependencies)
}

// SetNeedsUpdate is used to inform the node that it should
// reconfigure itself with the same config in order to process
// dependency updates. If the node was previously marked for removal,
// this makes no changes.
func (w *GraphNode) SetNeedsUpdate() {
	// doing two mutex ops here but we assume there's only one caller.
	w.setNeedsReconfigure(w.Config(), false, w.UnresolvedDependencies())
}

// setUnresolvedDependencies sets names that are yet to be resolved as
// dependencies for the node. Note that even an empty list will still
// set needsDependencyResolution to true. If no resolution is needed,
// you would call setDependenciesResolved. The resource graph calls
// these internally.
func (w *GraphNode) setUnresolvedDependencies(names ...string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.unresolvedDependencies = names
	w.needsDependencyResolution = true
}

// setDependenciesResolved sets that all unresolved dependencies have been
// resolved and linked/unlinked from the calling resource graph.
func (w *GraphNode) setDependenciesResolved() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.needsDependencyResolution = false
}

// UnresolvedDependencies returns the set of names that are yet to be resolved as
// dependencies for the node.
func (w *GraphNode) UnresolvedDependencies() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if len(w.unresolvedDependencies) == 0 {
		return nil
	}
	unresolvedDependencies := make([]string, 0, len(w.unresolvedDependencies))
	unresolvedDependencies = append(unresolvedDependencies, w.unresolvedDependencies...)
	return unresolvedDependencies
}

// Close closes the underlying resource of this node.
func (w *GraphNode) Close(ctx context.Context) error {
	w.mu.Lock()
	if w.current == nil {
		w.mu.Unlock()
		return nil
	}
	current := w.current
	w.current = nil

	// Unlock before calling Close() on underlying resource, since Close() behavior can be unpredictable
	// and usage of the graph node should not block on the underlying resource being closed.
	w.mu.Unlock()
	// TODO(RSDK-7928): we might want to make this transition a node to an "unconfigured"
	// or "removing" state.
	return current.Close(ctx)
}

func (w *GraphNode) replace(other *GraphNode) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		return errors.New("may only replace an uninitialized node")
	}
	if w == other {
		return nil
	}

	other.mu.Lock()
	w.updatedAt = other.updatedAt
	if other.graphLogicalClock != nil {
		w.graphLogicalClock = other.graphLogicalClock
	}
	w.lastReconfigured = other.lastReconfigured
	w.current = other.current
	w.currentModel = other.currentModel
	w.config = other.config
	w.lastErr = other.lastErr
	w.unresolvedDependencies = other.unresolvedDependencies
	w.needsDependencyResolution = other.needsDependencyResolution

	w.state = other.state
	w.transitionedAt = other.transitionedAt

	// other is now owned by the graph/node and is invalidated
	other.updatedAt = 0
	other.graphLogicalClock = nil
	other.lastReconfigured = nil
	other.current = nil
	other.currentModel = Model{}
	other.config = Config{}
	other.lastErr = nil
	other.unresolvedDependencies = nil
	other.needsDependencyResolution = false

	other.state = NodeStateUnknown
	other.transitionedAt = time.Time{}

	other.mu.Unlock()
	return nil
}

func (w *GraphNode) canTransitionTo(state NodeState) bool {
	switch w.state {
	case NodeStateUnknown:
	case NodeStateUnconfigured:
		//nolint
		switch state {
		case NodeStateConfiguring, NodeStateRemoving:
			return true
		}
	case NodeStateConfiguring:
		//nolint
		switch state {
		case NodeStateReady, NodeStateUnhealthy:
			return true
		}
	case NodeStateReady:
		//nolint
		switch state {
		case NodeStateConfiguring, NodeStateRemoving:
			return true
		}
	case NodeStateRemoving:
	case NodeStateUnhealthy:
		//nolint
		switch state {
		case NodeStateRemoving, NodeStateUnhealthy:
			return true
		case NodeStateReady:
			// TODO(NEEDS TICKET): ideally, a node should "recover" by the following transition
			// steps: "unhealthy" -> "configuring" -> "ready".
			//
			// However, once a node becomes "unhealthy" it is filtered out from most
			// reconfiguration operations, and is not available to be transitioned to
			// "configuring" when we want it to be.
			//
			// We eventually want to change the reconfiguration system to show unhealthy
			// nodes instead of filtering them out at each step of the process.
			//
			// In the meantime, we allow nodes to "recover" by transitioning directly
			// from "unhealthy" -> "ready".
			//
			// See this discussion for more details: https://github.com/viamrobotics/rdk/pull/4257#discussion_r1712173743
			return true
		}
	}
	return false
}

// transitionTo transitions the GraphNode to a new state. This method will log a warning
// if the state transition is not expected. This method is not thread-safe and must be
// called while holding a write lock on `mu` if accessed concurrently.
func (w *GraphNode) transitionTo(state NodeState) {
	if w.state == state && w.logger != nil {
		w.logger.Debugw("resource state self-transition", "state", w.state.String())
		return
	}

	// if state of a node is [NodeStateRemoving] it cannot transition to [NodeStateUnhealthy] until it is removed.
	// currently this is the only hard blocked transition
	// note this does not block SwapResource from transitioning a removing resource to ready
	if w.state == NodeStateRemoving && state == NodeStateUnhealthy {
		w.logger.Debug("node cannot transition from [NodeStateRemoving] to [NodeStateUnhealthy], blocking transition")
		return
	}

	if !w.canTransitionTo(state) && w.logger != nil {
		w.logger.Warnw("unexpected resource state transition", "from", w.state.String(), "to", state.String())
	}

	w.state = state
	w.transitionedAt = time.Now()
}

// Status returns the current [NodeStatus].
func (w *GraphNode) Status() NodeStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.status()
}

func (w *GraphNode) status() NodeStatus {
	err := w.lastErr
	logger := w.Logger()

	// check invariants between state and error
	switch {
	case w.state == NodeStateUnhealthy && w.lastErr == nil:
		logger.Warnw("an unhealthy node doesn't have an error")
	case w.state == NodeStateReady && w.lastErr != nil:
		logger.Warnw("a ready node still has an error", "error", err)
		// do not return leftover error in status if the node is ready
		err = nil
	}

	// TODO (RSDK-9550): Node should have the correct notion of its name
	return NodeStatus{
		State:       w.state,
		LastUpdated: w.transitionedAt,
		Revision:    w.revision,
		Error:       err,
	}
}

type graphNodeStats struct {
	State    int
	ResStats any
}

// Stats satisfies the FTDC Statser interface.
func (w *GraphNode) Stats() any {
	ret := graphNodeStats{}

	res, err := w.Resource()
	//nolint:errorlint
	switch err {
	case nil:
		ret.State = 0
	case errNotInitalized:
		ret.State = 1
	case errPendingRemoval:
		ret.State = 2
	default:
		// `w.lastErr != nil`
		ret.State = 3
	}

	if statser, isStatser := res.(ftdc.Statser); isStatser && err == nil {
		ret.ResStats = statser.Stats()
	}

	return ret
}

// NodeStatus encapsulates a resource name along with state transition metadata.
type NodeStatus struct {
	Name        Name
	State       NodeState
	LastUpdated time.Time
	Revision    string

	// Error contains any errors on the resource if it currently unhealthy.
	// This field will be nil if the resource is not in the [NodeStateUnhealthy] state.
	Error error
}
