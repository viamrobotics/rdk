package utils

import (
	"reflect"

	"github.com/pkg/errors"
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

	panic(errors.Errorf("wanted a []float64 for (%s) but got (%v) %T", name, x, x))
}

// Float64Slice attempts to return a slice of ints present in the map with
// the given name; returns an empty slice otherwise.
func (am AttributeMap) Float64Slice(name string) []float64 {
	if am == nil {
		return []float64{}
	}
	x := am[name]
	if x == nil {
		return []float64{}
	}

	if slice, ok := x.([]interface{}); ok {
		float64s := make([]float64, 0, len(slice))
		for _, v := range slice {
			if i, ok := v.(float64); ok {
				float64s = append(float64s, i)
			} else {
				panic(errors.Errorf("values in (%s) need to be float64 but got %T", name, v))
			}
		}
		return float64s
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

	if slice, ok := x.([]string); ok {
		return slice
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

// BoolSlice attempts to return a slice of bools present in the map with
// the given name; returns an empty slice otherwise.
func (am AttributeMap) BoolSlice(name string, def bool) []bool {
	if am == nil {
		return []bool{}
	}
	x := am[name]
	if x == nil {
		return []bool{}
	}

	if slice, ok := x.([]interface{}); ok {
		bools := make([]bool, 0, len(slice))
		for _, v := range slice {
			if b, ok := v.(bool); ok {
				bools = append(bools, b)
			} else {
				panic(errors.Errorf("values in (%s) need to be bools but got %T", name, v))
			}
		}
		return bools
	}

	panic(errors.Errorf("wanted a []bool for (%s) but got (%v) %T", name, x, x))
}

// Walk implements the Walker interface.
func (am AttributeMap) Walk(visitor Visitor) (interface{}, error) {
	w := attrWalker{visitor: visitor}
	m, err := w.walkMap(am)
	if err != nil {
		return nil, err
	}

	return AttributeMap(m), nil
}

type attrWalker struct {
	visitor Visitor
}

func (w *attrWalker) walkMap(data interface{}) (map[string]interface{}, error) {
	s := reflect.ValueOf(data)
	if s.Kind() != reflect.Map {
		return nil, errors.Errorf("data of type %T is not a map", data)
	}

	iter := reflect.ValueOf(data).MapRange()
	result := map[string]interface{}{}
	var err error
	for iter.Next() {
		k := iter.Key()
		if k.Kind() != reflect.String {
			return nil, errors.Errorf("map keys of type %v are not strings", k.Kind())
		}
		v := iter.Value().Interface()
		result[k.String()], err = w.walkInterface(v)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (w *attrWalker) walkInterface(data interface{}) (interface{}, error) {
	if data == nil {
		return data, nil
	}

	t := reflect.TypeOf(data)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var newData interface{}
	var err error
	switch t.Kind() {
	case reflect.Struct:
		newData, err = w.walkStruct(data)
		if err != nil {
			return nil, err
		}
	case reflect.Map:
		newData, err = w.walkMap(data)
		if err != nil {
			return nil, err
		}
	case reflect.Slice:
		newData, err = w.walkSlice(data)
		if err != nil {
			return nil, err
		}
	case reflect.String:
		fallthrough
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fallthrough
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fallthrough
	case reflect.Array, reflect.Bool, reflect.Chan, reflect.Complex128, reflect.Complex64, reflect.Float32,
		reflect.Float64, reflect.Func, reflect.Interface, reflect.Invalid, reflect.Pointer,
		reflect.Uintptr, reflect.UnsafePointer:
		fallthrough
	default:
		newData, err = w.visitor.Visit(data)
		if err != nil {
			return nil, err
		}
	}
	return newData, nil
}

func (w *attrWalker) walkSlice(data interface{}) ([]interface{}, error) {
	s := reflect.ValueOf(data)
	if s.Kind() != reflect.Slice {
		return nil, errors.Errorf("data of type %T is not a slice", data)
	}

	newList := make([]interface{}, 0, s.Len())
	for i := 0; i < s.Len(); i++ {
		value := s.Index(i).Interface()
		data, err := w.walkInterface(value)
		if err != nil {
			return nil, err
		}
		newList = append(newList, data)
	}
	return newList, nil
}

func (w *attrWalker) walkStruct(data interface{}) (interface{}, error) {
	t := reflect.TypeOf(data)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, errors.Errorf("data of type %T is not a struct", data)
	}
	res := map[string]interface{}{}
	value := reflect.ValueOf(data)
	if value.Kind() == reflect.Ptr && value.IsNil() {
		return res, nil
	}
	value = reflect.Indirect(value)
	for i := 0; i < t.NumField(); i++ {
		sField := t.Field(i)
		key := sField.Name

		field := value.Field(i).Interface()

		if isEmptyValue(reflect.ValueOf(field)) {
			res[key] = data
			continue
		}

		data, err := w.walkInterface(field)
		if err != nil {
			return nil, err
		}

		res[key] = data
	}
	return res, nil
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Chan, reflect.Complex128, reflect.Complex64, reflect.Func, reflect.Invalid, reflect.Struct,
		reflect.UnsafePointer:
		fallthrough
	default:
		return false
	}
}
