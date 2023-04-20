// Package internal is used only within this package and all code contained within
// is not supported and should be considered experimetnal and subject to change.
package internal

// ResourceMatcher matches resource expressions against resources.
// TODO(PRODUCT-460): right now this is just simple builtin strings and there is no real
// matching system.
type ResourceMatcher interface {
	notActuallyImplementedYet()
}

// ComponentDependencyWildcardMatcher is used internally right now for lack of a better way to
// "select" resources that another resource is dependency on. Usage of this is an
// anti-pattern and a better matcher system should exist.
var ComponentDependencyWildcardMatcher = ResourceMatcher(componentDependencyWildcardMatcher("*:component:*/*:*"))

type componentDependencyWildcardMatcher string

func (c componentDependencyWildcardMatcher) notActuallyImplementedYet() {}
