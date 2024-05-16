package utils

import (
	"flag"
	"math/rand"
)

// AssertType attempts to assert that the given interface argument is
// the given type parameter.
func AssertType[T any](from interface{}) (T, error) {
	var zero T
	asserted, ok := from.(T)
	if !ok {
		return zero, NewUnexpectedTypeError[T](from)
	}
	return asserted, nil
}

// FilterMap is a helper that returns a new map based on k,v pairs that pass predicate.
func FilterMap[K comparable, V any](orig map[K]V, predicate func(K, V) bool) map[K]V {
	ret := make(map[K]V)
	for key, val := range orig {
		if predicate(key, val) {
			ret[key] = val
		}
	}
	return ret
}

// FilterSlice returns a new slice with elements of `orig` which match `predicate`.
func FilterSlice[T any](orig []T, predicate func(T) bool) []T {
	ret := make([]T, 0)
	for _, item := range orig {
		if predicate(item) {
			ret = append(ret, item)
		}
	}
	return ret
}

// Rand is a wrapper for either a rand.Rand or a pass-through to the shared rand.x functions.
type Rand interface {
	Float64() float64
}

// Testing returns true when you are running in test suite.
func Testing() bool {
	// TODO switch to official testing.Testing method when we are on go 1.21
	return flag.Lookup("test.v") != nil
}

// randWrapper is a pass-through to the shared math/rand functions.
type randWrapper struct{}

func (randWrapper) Float64() float64 {
	return rand.Float64() //nolint:gosec
}

// SafeTestingRand returns a wrapper around the shared math/rand source in prod,
// and a deterministic rand.Rand seeded with 0 in test.
func SafeTestingRand() Rand {
	if Testing() {
		return rand.New(rand.NewSource(0)) //nolint:gosec
	}
	return randWrapper{}
}

// FindInSlice returns the first item in items which satisfies predicate, or nil.
func FindInSlice[T any](items []T, predicate func(T) bool) *T {
	for _, item := range items {
		if predicate(item) {
			return &item
		}
	}
	return nil
}

// MapOver applies fn() to a slice of items and returns a slice of the return values.
// TODO(golang/go#61898): use stdlib iter if it lands.
func MapOver[T, U any](items []T, fn func(T) (U, error)) ([]U, error) {
	ret := make([]U, 0, len(items))
	for _, item := range items {
		newItem, err := fn(item)
		if err != nil {
			return nil, err
		}
		ret = append(ret, newItem)
	}
	return ret, nil
}
