package resource

// Matcher matches resource expressions against resources.
type Matcher interface {
	IsMatch(Name) bool
}

type TypeMatcher struct {
	Type string
}

func (am TypeMatcher) IsMatch(name Name) bool {
	return name.API.Type.Name == am.Type
}

type SubtypeMatcher struct {
	Subtype string
}

func (sm SubtypeMatcher) IsMatch(name Name) bool {
	return name.API.Type.Name == sm.Subtype
}
