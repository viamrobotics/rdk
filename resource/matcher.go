package resource

import "reflect"

// Matcher describes whether a given resource matches its specified criteria.
type Matcher interface {
	IsMatch(Resource) bool
}

// TypeMatcher matches resources that have the given Type.
type TypeMatcher struct {
	Type string
}

// IsMatch returns true if the given resource has a Type that matches the TypeMatcher's Type.
func (tm TypeMatcher) IsMatch(r Resource) bool {
	return r.Name().API.Type.Name == tm.Type
}

// SubtypeMatcher matches resources that have the given Subtype.
type SubtypeMatcher struct {
	Subtype string
}

// IsMatch returns true if the given resource has a Subtype that matches the SubtypeMatcher's Subtype.
func (sm SubtypeMatcher) IsMatch(r Resource) bool {
	return r.Name().API.SubtypeName == sm.Subtype
}

// InterfaceMatcher matches resources that fulfill the given interface.
type InterfaceMatcher struct {
	Interface interface{}
}

// IsMatch returns true if the given resource fulfills the InterfaceMatcher's Interface.
func (im InterfaceMatcher) IsMatch(r Resource) bool {
	return reflect.TypeOf(r).Implements(reflect.TypeOf(im.Interface).Elem())
}
