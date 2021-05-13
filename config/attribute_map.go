package config

import "fmt"

// An AttributeMap is a convenience wrapper for pulling out
// typed information from a map.
type AttributeMap map[string]interface{}

// Has returns whether or not the given naem is in the map.
func (am AttributeMap) Has(name string) bool {
	_, has := am[name]
	return has
}

// String attempts to return a string present in the map with
// the given name; returns an empty string otherwise.
func (am AttributeMap) String(name string) string {
	if am == nil {
		return ""
	}
	x := am[name]
	if x == nil {
		return ""
	}

	if s, ok := x.(string); ok {
		return s
	}

	panic(fmt.Errorf("wanted a string for (%s) but got (%v) %T", name, x, x))
}

// Int attempts to return an integer present in the map with
// the given name; returns the given default otherwise.
func (am AttributeMap) Int(name string, def int) int {
	if am == nil {
		return def
	}
	x, has := am[name]
	if !has {
		return def
	}

	if v, ok := x.(int); ok {
		return v
	}

	if v, ok := x.(float64); ok && v == float64(int64(v)) {
		return int(v)
	}

	panic(fmt.Errorf("wanted an int for (%s) but got (%v) %T", name, x, x))
}

// Float64 attempts to return a float64 present in the map with
// the given name; returns the given default otherwise.
func (am AttributeMap) Float64(name string, def float64) float64 {
	if am == nil {
		return def
	}
	x, has := am[name]
	if !has {
		return def
	}

	if v, ok := x.(float64); ok {
		return v
	}

	panic(fmt.Errorf("wanted an int for (%s) but got (%v) %T", name, x, x))
}

// Bool attempts to return a boolean present in the map with
// the given name; returns the given default otherwise.
func (am AttributeMap) Bool(name string, def bool) bool {
	if am == nil {
		return def
	}
	x, has := am[name]
	if !has {
		return def
	}

	if v, ok := x.(bool); ok {
		return v
	}

	panic(fmt.Errorf("wanted a bool for (%s) but got (%v) %T", name, x, x))
}
