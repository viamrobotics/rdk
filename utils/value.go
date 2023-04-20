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
