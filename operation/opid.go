// Package operation manages operation ids
package operation

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type opidKeyType string

const opidKey = opidKeyType("opid")

var invalidMethods = map[string]bool{
	"proto.rpc.webrtc.v1.SignalingService":          true,
	"/proto.api.robot.v1.RobotService/StreamStatus": true,
}

// Operation is an operation happening on the server.
type Operation struct {
	ID        uuid.UUID
	Method    string
	Arguments interface{}
	Started   time.Time

	myManager *Manager
	cancel    context.CancelFunc
	labels    []string
}

// Cancel cancel the context associated with an operation.
func (o *Operation) Cancel() {
	o.cancel()
}

// HasLabel returns true if this oepration has a speficic label.
func (o *Operation) HasLabel(label string) bool {
	for _, l := range o.labels {
		if l == label {
			return true
		}
	}
	return false
}

// CancelOtherWithLabel will cancel all operations besides this one with this label.
func (o *Operation) CancelOtherWithLabel(label string) {
	all := o.myManager.All()
	for _, op := range all {
		if op == o {
			continue
		}
		if op.HasLabel(label) {
			op.Cancel()
		}
	}

	o.labels = append(o.labels, label)
}

func (o *Operation) cleanup() {
	o.myManager.remove(o.ID)
}

// NewManager creates a new manager for holding Operations.
func NewManager() *Manager {
	return &Manager{ops: map[string]*Operation{}}
}

// Manager holds Operations.
type Manager struct {
	ops  map[string]*Operation
	lock sync.Mutex
}

func (m *Manager) remove(id uuid.UUID) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.ops, id.String())
}

func (m *Manager) add(op *Operation) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.ops[op.ID.String()] = op
}

// All returns all running operations.
func (m *Manager) All() []*Operation {
	m.lock.Lock()
	defer m.lock.Unlock()
	a := make([]*Operation, 0, len(m.ops))
	for _, o := range m.ops {
		a = append(a, o)
	}
	return a
}

// Find an Operation.
func (m *Manager) Find(id uuid.UUID) *Operation {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.ops[id.String()]
}

// FindString an Operation.
func (m *Manager) FindString(id string) *Operation {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.ops[id]
}

// Create puts an operation on this context.
func (m *Manager) Create(ctx context.Context, method string, args interface{}) (context.Context, func()) {
	if ctx.Value(opidKey) != nil {
		panic("operations cannot be nested")
	}

	op := &Operation{
		ID:        uuid.New(),
		Method:    method,
		Arguments: args,
		Started:   time.Now(),
		myManager: m,
	}
	ctx = context.WithValue(ctx, opidKey, op)
	ctx, op.cancel = context.WithCancel(ctx)

	// Add method to manager if not in invalid map
	if _, ok := invalidMethods[op.Method]; !ok {
		m.add(op)
	}
	return ctx, func() { op.cleanup() }
}

// Get returns the current Operation. This can be nil.
func Get(ctx context.Context) *Operation {
	o := ctx.Value(opidKey)
	if o == nil {
		return nil
	}
	return o.(*Operation)
}

// CancelOtherWithLabel will cancel all operations besides this one with this label.
// if no Operation is set, will do nothing.
func CancelOtherWithLabel(ctx context.Context, label string) {
	if o := Get(ctx); o != nil {
		o.CancelOtherWithLabel(label)
	}
}
