// Package operation manageds operation ids
package operation

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type opidKeyType string

const opidKey = opidKeyType("opid")

// Operation is an operation happening on the server.
type Operation struct {
	ID        uuid.UUID
	Method    string
	Arguments interface{}
	Started   time.Time

	cancel context.CancelFunc
	labels []string
}

// Cancel cancel the context associated with an operation.
func (o *Operation) Cancel() {
	o.cancel()
}

// HasLabel returns true if this oepration has a speficic lable.
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
	all := CurrentOps()
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
	theGlobal.remove(o.ID)
}

var theGlobal = &global{ops: map[string]*Operation{}}

type global struct {
	ops  map[string]*Operation
	lock sync.Mutex
}

func (g *global) remove(id uuid.UUID) {
	g.lock.Lock()
	defer g.lock.Unlock()
	delete(g.ops, id.String())
}

func (g *global) add(op *Operation) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.ops[op.ID.String()] = op
}

func (g *global) all() []*Operation {
	g.lock.Lock()
	defer g.lock.Unlock()
	a := make([]*Operation, 0, len(g.ops))
	for _, o := range g.ops {
		a = append(a, o)
	}
	return a
}

func (g *global) find(id uuid.UUID) *Operation {
	g.lock.Lock()
	defer g.lock.Unlock()
	return g.ops[id.String()]
}

func (g *global) findString(id string) *Operation {
	g.lock.Lock()
	defer g.lock.Unlock()
	return g.ops[id]
}

// CurrentOps returns all of the currently running operations.
func CurrentOps() []*Operation {
	return theGlobal.all()
}

// FindOp finds an op by id, could return nil.
func FindOp(id uuid.UUID) *Operation {
	return theGlobal.find(id)
}

// FindOpString finds an op by id, could return nil.
func FindOpString(id string) *Operation {
	return theGlobal.findString(id)
}

// Create puts an operation on this context.
func Create(ctx context.Context, method string, args interface{}) (context.Context, func()) {
	if ctx.Value(opidKey) != nil {
		panic("operations cannot be nested")
	}

	op := &Operation{
		ID:        uuid.New(),
		Method:    method,
		Arguments: args,
		Started:   time.Now(),
	}
	ctx = context.WithValue(ctx, opidKey, op)
	ctx, op.cancel = context.WithCancel(ctx)

	theGlobal.add(op)

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

