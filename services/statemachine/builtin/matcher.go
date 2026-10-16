package builtin

import (
	"reflect"
	"strings"
)

// Match reports whether value satisfies the condition. A condition is a
// minimal matcher over a returned map:
//
//	{"pressed": true}                        equality per key
//	{"confidence": {"gt": 0.8}}              comparison operators gt, gte, lt, lte, ne, eq
//	{"state": {"in": ["a", "b"]}}            membership
//	{"name": {"contains": "glass"}}          substring
//	{"pose": {"exists": true}}               key presence
//	{"any": {"class_name": "empty_glass"}}   some element of a list matches
//	{"all": {...}}, {"not": {...}}
//
// Anything richer belongs in a resource that returns a boolean.
func Match(cond, value interface{}) bool {
	switch c := cond.(type) {
	case map[string]interface{}:
		if len(c) == 0 {
			return true
		}
		if isOperatorMap(c) {
			return matchOperators(c, value)
		}
		vm, ok := toMap(value)
		if !ok {
			return false
		}
		for k, sub := range c {
			if !Match(sub, vm[k]) {
				return false
			}
		}
		return true
	case nil:
		return value == nil
	default:
		return equal(cond, value)
	}
}

var operators = map[string]bool{
	"eq": true, "ne": true, "gt": true, "gte": true, "lt": true, "lte": true,
	"in": true, "contains": true, "exists": true, "any": true, "all": true, "not": true,
}

func isOperatorMap(m map[string]interface{}) bool {
	for k := range m {
		if !operators[k] {
			return false
		}
	}
	return true
}

func matchOperators(ops map[string]interface{}, value interface{}) bool {
	for op, arg := range ops {
		if !matchOperator(op, arg, value) {
			return false
		}
	}
	return true
}

func matchOperator(op string, arg, value interface{}) bool {
	switch op {
	case "eq":
		return equal(arg, value)
	case "ne":
		return !equal(arg, value)
	case "gt", "gte", "lt", "lte":
		a, okA := toFloat(arg)
		v, okV := toFloat(value)
		if !okA || !okV {
			return false
		}
		switch op {
		case "gt":
			return v > a
		case "gte":
			return v >= a
		case "lt":
			return v < a
		default:
			return v <= a
		}
	case "in":
		list, ok := toList(arg)
		if !ok {
			return false
		}
		for _, item := range list {
			if equal(item, value) {
				return true
			}
		}
		return false
	case "contains":
		s, ok := value.(string)
		sub, ok2 := arg.(string)
		if ok && ok2 {
			return strings.Contains(s, sub)
		}
		list, ok := toList(value)
		if !ok {
			return false
		}
		for _, item := range list {
			if equal(arg, item) {
				return true
			}
		}
		return false
	case "exists":
		want, _ := arg.(bool)
		return (value != nil) == want
	case "any", "all":
		list, ok := toList(value)
		if !ok {
			if vm, isMap := toMap(value); isMap {
				// Search the first list-valued field: a response like
				// {"detections": [...]} matches against its elements.
				for _, fv := range vm {
					if l, isList := toList(fv); isList {
						list, ok = l, true
						break
					}
				}
			}
			if !ok {
				return false
			}
		}
		for _, item := range list {
			if Match(arg, item) {
				if op == "any" {
					return true
				}
			} else if op == "all" {
				return false
			}
		}
		return op == "all"
	case "not":
		return !Match(arg, value)
	}
	return false
}

func equal(a, b interface{}) bool {
	if fa, ok := toFloat(a); ok {
		if fb, ok := toFloat(b); ok {
			return fa == fb
		}
		return false
	}
	return reflect.DeepEqual(a, b)
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}

func toList(v interface{}) ([]interface{}, bool) {
	if l, ok := v.([]interface{}); ok {
		return l, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}
	out := make([]interface{}, rv.Len())
	for i := range out {
		out[i] = rv.Index(i).Interface()
	}
	return out, true
}

func toMap(v interface{}) (map[string]interface{}, bool) {
	if m, ok := v.(map[string]interface{}); ok {
		return m, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	out := make(map[string]interface{}, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		out[iter.Key().String()] = iter.Value().Interface()
	}
	return out, true
}
