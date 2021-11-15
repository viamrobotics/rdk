package testutils

import "go.viam.com/core/resource"

// NewResourceNameSet returns a flattened set of name strings from
// a collection of resource.Name objects for the purposes of comparison
// in automated tests
func NewResourceNameSet(resourceNames ...resource.Name) map[resource.Name]struct{} {
	set := make(map[resource.Name]struct{}, len(resourceNames))
	for _, val := range resourceNames {
		set[val] = struct{}{}
	}
	return set
}

// ResourceMapToStringSet takes a collection of resource.Name objects bucketed by
// their subtypes and returns a set of name strings for the purposes of comparison
// in automated tests
func ResourceMapToStringSet(resourceMap map[resource.Subtype][]resource.Name) map[resource.Name]struct{} {
	set := make(map[resource.Name]struct{})
	for _, resourceNames := range resourceMap {
		for _, name := range resourceNames {
			set[name] = struct{}{}
		}
	}
	return set
}
