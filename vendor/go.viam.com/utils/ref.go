package utils

import "sync"

// RefCountedValue is a utility to "reference count" values in order
// to destruct them once no one references them.
// If you don't require that kind of logic, just rely on golang's
// garbage collection.
type RefCountedValue interface {
	// Ref increments the reference count and returns the value.
	Ref() interface{}

	// Deref decrements the reference count and returns if this
	// dereference resulted in the value being unreferenced.
	Deref() (unreferenced bool)
}

type refCountedValue struct {
	mu    sync.Mutex
	count int
	val   interface{}
}

// NewRefCountedValue returns a new reference counted value for the given
// value. Its reference count starts at zero but is not released. It is
// assumed the caller of this will reference it at least once.
func NewRefCountedValue(val interface{}) RefCountedValue {
	return &refCountedValue{val: val}
}

func (rcv *refCountedValue) Ref() interface{} {
	rcv.mu.Lock()
	defer rcv.mu.Unlock()
	if rcv.count == -1 {
		panic("already released")
	}
	rcv.count++
	return rcv.val
}

func (rcv *refCountedValue) Deref() bool {
	rcv.mu.Lock()
	defer rcv.mu.Unlock()
	if rcv.count <= 0 {
		panic("deref when count already zero")
	}
	rcv.count--
	if rcv.count == 0 {
		rcv.count = -1
		return true
	}
	return false
}
