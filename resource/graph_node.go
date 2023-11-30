package resource

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

// A GraphNode contains the current state of a resource.
// It starts out as either uninitialized, unconfigured, or configured.
// Based on these states, the underlying Resource may or may not be available.
// Additionally, the node can be informed that the resource either needs to be
// updated or eventually removed. During its life, errors may be set on the
// node to indicate that the resource is no longer available to external users.
type GraphNode struct {
	// mu guards all fields below.
	mu sync.RWMutex

	// graphLogicalClock is a pointer to the Graph's logicalClock. It is
	// incremented every time any GraphNode calls SwapResource.
	graphLogicalClock *atomic.Int64
	// updatedAt is the value of the graphLogicalClock when it was last
	// incremented by this GraphNode's SwapResource method. It is only referenced
	// in tests.
	updatedAt int64

	current                   Resource
	currentModel              Model
	config                    Config
	needsReconfigure          bool
	lastReconfigured          *time.Time
	lastErr                   error
	markedForRemoval          bool
	unresolvedDependencies    []string
	needsDependencyResolution bool

	logger logging.Logger
}

var (
	errNotInitalized  = errors.New("resource not initialized yet")
	errPendingRemoval = errors.New("resource is pending removal")
)

// NewUninitializedNode returns a node that is brand new and not yet initialized.
func NewUninitializedNode() *GraphNode {
	return &GraphNode{}
}

// NewUnconfiguredGraphNode returns a node that contains enough information to
// construct the underlying resource.
func NewUnconfiguredGraphNode(config Config, dependencies []string) *GraphNode {
	node := &GraphNode{}
	node.SetNewConfig(config, dependencies)
	return node
}

// NewConfiguredGraphNode returns a node that is already configured with
// the supplied config and resource.
func NewConfiguredGraphNode(config Config, res Resource, resModel Model) *GraphNode {
	node := &GraphNode{}
	node.SetNewConfig(config, nil)
	node.setDependenciesResolved()
	node.SwapResource(res, resModel)
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
// GraphNode is uninitialized or unconfigured.
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
	if w.markedForRemoval {
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

// SetLogger associates a logger object with this resource node. This is expected to be the logger
// passed into the `Constructor` when registering component resources.
func (w *GraphNode) SetLogger(logger logging.Logger) {
	w.logger = logger
}

// SetLogLevel changes the log level of the logger (if available). Processing configs is the main
// entry point for changing log levels. Which will affect whether models making log calls are
// suppressed or not.
func (w *GraphNode) SetLogLevel(level logging.Level) {
	if w.logger != nil {
		w.logger.SetLevel(level)
	}
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
	return !w.markedForRemoval && w.lastErr == nil && w.current != nil
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
func (w *GraphNode) SwapResource(newRes Resource, newModel Model) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.current = newRes
	w.currentModel = newModel
	w.lastErr = nil
	w.needsReconfigure = false
	w.markedForRemoval = false

	// these should already be set
	w.unresolvedDependencies = nil
	w.needsDependencyResolution = false

	if w.graphLogicalClock != nil {
		w.updatedAt = w.graphLogicalClock.Add(1)
	}
	now := time.Now()
	w.lastReconfigured = &now
}

// MarkForRemoval marks this node for removal at a later time.
func (w *GraphNode) MarkForRemoval() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.markedForRemoval = true
}

// MarkedForRemoval returns if this node is marked for removal.
func (w *GraphNode) MarkedForRemoval() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.markedForRemoval
}

// LogAndSetLastError logs and sets the latest error on this node. This will cause the resource to
// become unavailable to external users of the graph. The resource manager may still access the
// underlying resource via UnsafeResource.
//
// The additional `args` should come in key/value pairs for structured logging.
func (w *GraphNode) LogAndSetLastError(err error, args ...any) {
	w.mu.Lock()
	w.lastErr = err
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
	return !w.markedForRemoval && w.needsReconfigure
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
	if !mustReconfigure && w.markedForRemoval {
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
	w.needsReconfigure = true
	w.markedForRemoval = false
	w.unresolvedDependencies = dependencies
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
	defer w.mu.Unlock()

	if w.current == nil {
		return nil
	}
	current := w.current
	w.current = nil
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
	w.needsReconfigure = other.needsReconfigure
	w.lastErr = other.lastErr
	w.markedForRemoval = other.markedForRemoval
	w.unresolvedDependencies = other.unresolvedDependencies
	w.needsDependencyResolution = other.needsDependencyResolution

	// other is now owned by the graph/node and is invalidated
	other.updatedAt = 0
	other.graphLogicalClock = nil
	other.lastReconfigured = nil
	other.current = nil
	other.currentModel = Model{}
	other.config = Config{}
	other.needsReconfigure = false
	other.lastErr = nil
	other.markedForRemoval = false
	other.unresolvedDependencies = nil
	other.needsDependencyResolution = false
	other.mu.Unlock()
	return nil
}
