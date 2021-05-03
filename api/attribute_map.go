package api

import "fmt"

// An AttributeMap is a convenience wrapper for pulling out
// typed information from a map.
type AttributeMap map[string]interface{}

// Has returns whether or not the given naem is in the map.
func (am AttributeMap) Has(name string) bool {
	_, has := am[name]
	return has
}

// GetString attempts to return a string present in the map with
// the given name; returns an empty string otherwise.
func (am AttributeMap) GetString(name string) string {
	if am == nil {
		return ""
	}
	x := am[name]
	if x == nil {
		return ""
	}

	s, ok := x.(string)
	if ok {
		return s
	}

	panic(fmt.Errorf("wanted a string for (%s) but got (%v) %T", name, x, x))
}

// GetInt attempts to return an integer present in the map with
// the given name; returns the given default otherwise.
func (am AttributeMap) GetInt(name string, def int) int {
	if am == nil {
		return def
	}
	x, has := am[name]
	if !has {
		return def
	}

	v, ok := x.(int)
	if ok {
		return v
	}

	v2, ok := x.(float64)
	if ok {
		// TODO(erh): is this safe? json defaults to float64, so seems nice
		return int(v2)
	}

	panic(fmt.Errorf("wanted an int for (%s) but got (%v) %T", name, x, x))
}

// GetFloat64 attempts to return a float64 present in the map with
// the given name; returns the given default otherwise.
func (am AttributeMap) GetFloat64(name string, def float64) float64 {
	if am == nil {
		return def
	}
	x, has := am[name]
	if !has {
		return def
	}

	v, ok := x.(float64)
	if ok {
		return v
	}

	panic(fmt.Errorf("wanted an int for (%s) but got (%v) %T", name, x, x))
}

// GetBool attempts to return a boolean present in the map with
// the given name; returns the given default otherwise.
func (am AttributeMap) GetBool(name string, def bool) bool {
	if am == nil {
		return def
	}
	x, has := am[name]
	if !has {
		return def
	}

	v, ok := x.(bool)
	if ok {
		return v
	}

	panic(fmt.Errorf("wanted a bool for (%s) but got (%v) %T", name, x, x))
}
