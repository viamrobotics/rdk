package config

import (
	"github.com/go-errors/errors"
)

// An AttributeMap is a convenience wrapper for pulling out
// typed information from a map.
type AttributeMap map[string]interface{}

// Has returns whether or not the given name is in the map.
func (am AttributeMap) Has(name string) bool {
	_, has := am[name]
	return has
}

// IntSlice attempts to return a slice of ints present in the map with
// the given name; returns an empty slice otherwise.
func (am AttributeMap) IntSlice(name string) []int {
	if am == nil {
		return []int{}
	}
	x := am[name]
	if x == nil {
		return []int{}
	}

	if slice, ok := x.([]interface{}); ok {
		ints := make([]int, 0, len(slice))
		for _, v := range slice {
			if i, ok := v.(int); ok {
				ints = append(ints, i)
			} else if i, ok := v.(float64); ok && i == float64(int64(i)) {
				ints = append(ints, int(i))
			} else {
				panic(errors.Errorf("values in (%s) need to be ints but got %T", name, v))
			}
		}
		return ints
	}

	panic(errors.Errorf("wanted a []int for (%s) but got (%v) %T", name, x, x))
}

// StringSlice attempts to return a slice of strings present in the map with
// the given name; returns an empty slice otherwise.
func (am AttributeMap) StringSlice(name string) []string {
	if am == nil {
		return []string{}
	}
	x := am[name]
	if x == nil {
		return []string{}
	}

	if slice, ok := x.([]interface{}); ok {
		strings := make([]string, 0, len(slice))
		for _, v := range slice {
			if s, ok := v.(string); ok {
				strings = append(strings, s)
			} else {
				panic(errors.Errorf("values in (%s) need to be strings but got %T", name, v))
			}
		}
		return strings
	}

	panic(errors.Errorf("wanted a []string for (%s) but got (%v) %T", name, x, x))
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

	panic(errors.Errorf("wanted a string for (%s) but got (%v) %T", name, x, x))
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

	panic(errors.Errorf("wanted an int for (%s) but got (%v) %T", name, x, x))
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

	panic(errors.Errorf("wanted a float for (%s) but got (%v) %T", name, x, x))
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

	panic(errors.Errorf("wanted a bool for (%s) but got (%v) %T", name, x, x))
}
