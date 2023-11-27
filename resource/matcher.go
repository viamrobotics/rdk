package resource

// Matcher describes whether a given resource matches its specified criteria.
type Matcher interface {
	IsMatch(Name) bool
}

// TypeMatcher matches resource names that have the given Type.
type TypeMatcher struct {
	Type string
}

// IsMatch returns true if the given name has a Type that matches the TypeMatcher's Type.
func (tm TypeMatcher) IsMatch(name Name) bool {
	return name.API.Type.Name == tm.Type
}

// SubtypeMatcher matches resource names that have the given Subtype.
type SubtypeMatcher struct {
	Subtype string
}

// IsMatch returns true if the given name has a Subtype that matches the SubtypeMatcher's Subtype.
func (sm SubtypeMatcher) IsMatch(name Name) bool {
	return name.API.SubtypeName == sm.Subtype
}
