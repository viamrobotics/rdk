package ssync

import "sync"

// Map is an alias to [sync.Map] with generic type parameters.
type Map[K comparable, V any] sync.Map

// Clear is an alias to [sync.Map.Clear].
func (m *Map[K, V]) Clear() {
	(*sync.Map)(m).Clear()
}

// CompareAndDelete is an alias to [sync.Map.CompareAndDelete].
func (m *Map[K, V]) CompareAndDelete(key K, old V) bool {
	return (*sync.Map)(m).CompareAndDelete(key, old)
}

// CompareAndSwap is an alias to [sync.Map.CompareAndSwap].
func (m *Map[K, V]) CompareAndSwap(key K, old, new V) bool { //nolint: revive
	return (*sync.Map)(m).CompareAndSwap(key, old, new)
}

// Delete is an alias to [sync.Map.Delete].
func (m *Map[K, V]) Delete(key K) {
	(*sync.Map)(m).Delete(key)
}

// Load is an alias to [sync.Map.Load].
func (m *Map[K, V]) Load(key K) (V, bool) {
	v, ok := (*sync.Map)(m).Load(key)
	if !ok {
		var zero V
		return zero, ok
	}
	return v.(V), ok
}

// LoadOrStore is an alias to [sync.Map.LoadOrStore].
func (m *Map[K, V]) LoadOrStore(key K, value V) (V, bool) {
	v, stored := (*sync.Map)(m).LoadOrStore(key, value)
	return v.(V), stored
}

// Range is an alias to [sync.Map.Range].
func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	(*sync.Map)(m).Range(func(key, value any) bool {
		return f(key.(K), value.(V))
	})
}

// Store is an alias to [sync.Map.Store].
func (m *Map[K, V]) Store(key K, value V) {
	(*sync.Map)(m).Store(key, value)
}

// Swap is an alias to [sync.Map.Swap].
func (m *Map[K, V]) Swap(key K, value V) (V, bool) {
	v, ok := (*sync.Map)(m).Swap(key, value)
	if !ok {
		var zero V
		return zero, ok
	}
	return v.(V), ok
}
