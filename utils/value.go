package utils

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
