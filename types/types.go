package types

// ExtraParams is a key-value map of unstructured additional arguments.
type ExtraParams = map[string]interface{}

// ZeroExtraParams returns an empty map.
func ZeroExtraParams() ExtraParams {
	return make(ExtraParams)
}
