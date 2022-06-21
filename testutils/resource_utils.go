package testutils

import (
	"go.viam.com/rdk/resource"
)

// NewResourceNameSet returns a flattened set of name strings from
// a collection of resource.Name objects for the purposes of comparison
// in automated tests.
func NewResourceNameSet(resourceNames ...resource.Name) map[resource.Name]struct{} {
	set := make(map[resource.Name]struct{}, len(resourceNames))
	for _, val := range resourceNames {
		set[val] = struct{}{}
	}
	return set
}

// ExtractNames takes a slice of resource.Name objects
// and returns a slice of name strings for the purposes of comparison
// in automated tests.
func ExtractNames(values ...resource.Name) []string {
	var names []string
	for _, n := range values {
		names = append(names, n.Name)
	}
	return names
}

// ConcatResourceNames takes a slice of slices of resource.Name objects
// and returns a concatenated slice of resource.Name for the purposes of comparison
// in automated tests.
func ConcatResourceNames(values ...[]resource.Name) []resource.Name {
	var rNames []resource.Name
	for _, v := range values {
		rNames = append(rNames, v...)
	}
	return rNames
}

// AddSuffixes takes a slice of resource.Name objects and for each suffix,
// adds the suffix to every object, then returns the entire list.
func AddSuffixes(values []resource.Name, suffixes ...string) []resource.Name {
	var rNames []resource.Name

	for _, s := range suffixes {
		for _, v := range values {
			newName := resource.NameFromSubtype(v.Subtype, v.Name+s)
			rNames = append(rNames, newName)
		}
	}
	return rNames
}

// AddRemote takes a slice of resource.Name objects and for each remote,
// adds the remote to every object, then returns the entire list.
func AddRemotes(values []resource.Name, remotes ...string) []resource.Name {
	var rNames []resource.Name

	for _, s := range remotes {
		for _, v := range values {
			v.PrependRemote(resource.RemoteName(s))
			rNames = append(rNames, v)
		}
	}
	return rNames
}
