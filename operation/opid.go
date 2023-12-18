// Package operation manages operation ids
package operation

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/session"
)

type opidKeyType string

const opidKey = opidKeyType("opid")

var methodPrefixesToFilter = [...]string{
	"/proto.rpc.webrtc.v1.SignalingService",
	"/viam.robot.v1.RobotService/StreamStatus",
	"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
}

// Operation is an operation happening on the server.
type Operation struct {
	ID        uuid.UUID
	SessionID uuid.UUID
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

// HasLabel returns true if this operation has a specific label.
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
func NewManager(logger logging.Logger) *Manager {
	return &Manager{ops: map[string]*Operation{}, logger: logger}
}

// Manager holds Operations.
type Manager struct {
	ops    map[string]*Operation
	lock   sync.Mutex
	logger logging.Logger
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
	return m.createWithID(ctx, uuid.New(), method, args)
}

func (m *Manager) createWithID(ctx context.Context, id uuid.UUID, method string, args interface{}) (context.Context, func()) {
	if ctx.Value(opidKey) != nil {
		panic("operations cannot be nested")
	}

	for _, val := range methodPrefixesToFilter {
		if strings.HasPrefix(method, val) {
			return ctx, func() {}
		}
	}

	o := m.Find(id)
	if o != nil {
		m.logger.CWarnw(ctx, "attempt to create duplicate operation", "id", id.String(), "method", method)
	}

	op := &Operation{
		ID:        id,
		Method:    method,
		Arguments: args,
		Started:   time.Now(),
		myManager: m,
	}
	if sess, ok := session.FromContext(ctx); ok {
		op.SessionID = sess.ID()
	}
	ctx = context.WithValue(ctx, opidKey, op)
	ctx, op.cancel = context.WithCancel(ctx)
	m.add(op)

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
