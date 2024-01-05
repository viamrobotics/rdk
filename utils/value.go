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
