// Package types exists to facility a refactor - TODO remove
package types

// ExtraParams is a key-value map of unstructured additional arguments.
type ExtraParams = map[string]interface{}

// ZeroExtraParams returns an empty map.
func ZeroExtraParams() ExtraParams {
	return make(ExtraParams)
}

// OneExtraParam returns a map with one entry.
func OneExtraParam(key string, value string) ExtraParams {
	extra := ZeroExtraParams()
	extra[key] = value
	return extra
}
