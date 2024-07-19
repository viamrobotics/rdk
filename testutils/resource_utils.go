package testutils

import (
	"cmp"
	"context"
	"slices"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/resource"
)

// NewUnimplementedResource returns a resource that has all methods
// unimplemented.
func NewUnimplementedResource(name resource.Name) resource.Resource {
	return &unimplResource{Named: name.AsNamed()}
}

type unimplResource struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
}

var (
	// EchoFunc is a helper to echo out the say command passsed in a Do.
	EchoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return cmd, nil
	}

	// TestCommand is a dummy command to send for a DoCommand.
	TestCommand = map[string]interface{}{"command": "test", "data": 500}
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

// newSortedResourceNames returns a new slice of resources names sorted by each
// resource's fully-qualified names for the purposes of comparison in automated tests.
func newSortedResourceNames(resourceNames []resource.Name) []resource.Name {
	sorted := make([]resource.Name, len(resourceNames))
	copy(sorted, resourceNames)
	slices.SortStableFunc(sorted, func(r1, r2 resource.Name) int {
		return cmp.Compare(r1.String(), r2.String())
	})
	return sorted
}

// VerifySameResourceNames asserts that two slices of resource.Names contain the same
// elements without considering order.
func VerifySameResourceNames(tb testing.TB, actual, expected []resource.Name) {
	tb.Helper()

	test.That(tb, newSortedResourceNames(actual), test.ShouldResemble, newSortedResourceNames(expected))
}

// VerifySameResourceStatuses asserts that two slices of resource.Status contain the same
// elements without considering order. Does not consider update timestamps when
// comparing.
func VerifySameResourceStatuses(tb testing.TB, actual, expected []resource.Status) {
	tb.Helper()

	sortedActual := newSortedResourceStatuses(actual)
	sortedExpected := newSortedResourceStatuses(expected)

	for i := range sortedActual {
		sortedActual[i].LastUpdated = time.Time{}
	}
	for i := range sortedExpected {
		sortedExpected[i].LastUpdated = time.Time{}
	}

	test.That(tb, sortedActual, test.ShouldResemble, sortedExpected)
}

func newSortedResourceStatuses(resourceStatuses []resource.Status) []resource.Status {
	sorted := make([]resource.Status, len(resourceStatuses))
	copy(sorted, resourceStatuses)
	slices.SortStableFunc(sorted, func(r1, r2 resource.Status) int {
		return cmp.Compare(r1.Name.String(), r2.Name.String())
	})
	return sorted
}

// ExtractNames takes a slice of resource.Name objects
// and returns a slice of name strings for the purposes of comparison
// in automated tests.
func ExtractNames(values ...resource.Name) []string {
	var names []string
	for _, n := range values {
		names = append(names, n.ShortName())
	}
	return names
}

// SubtractNames removes values from the first slice of resource names.
func SubtractNames(from []resource.Name, values ...resource.Name) []resource.Name {
	difference := make([]resource.Name, 0, len(from))
	for _, n := range from {
		var found bool
		for _, v := range values {
			if n == v {
				found = true
				break
			}
		}
		if found {
			continue
		}
		difference = append(difference, n)
	}
	return difference
}

// VerifyTopologicallySortedLevels verifies each topological layer of a sort against the given levels from
// most dependencies to least dependencies.
func VerifyTopologicallySortedLevels(t *testing.T, g *resource.Graph, levels [][]resource.Name, exclusions ...resource.Name) {
	sorted := g.TopologicalSortInLevels()
	sorted = SubtractNamesFromLevels(sorted, exclusions...)
	test.That(t, sorted, test.ShouldHaveLength, len(levels))

	for idx, level := range levels {
		t.Log("checking level", idx)
		test.That(t, sorted[idx], test.ShouldHaveLength, len(level))
		test.That(t, NewResourceNameSet(sorted[idx]...), test.ShouldResemble, NewResourceNameSet(level...))
	}
}

// SubtractNamesFromLevels removes values from each slice of resource names.
func SubtractNamesFromLevels(from [][]resource.Name, values ...resource.Name) [][]resource.Name {
	differences := make([][]resource.Name, 0, len(from))
	for _, names := range from {
		differences = append(differences, SubtractNames(names, values...))
	}
	return differences
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

// ConcatResourceStatus takes a slice of slices of resource.Status objects and returns a
// concatenated slice of resource.Status for the purposes of comparison in automated
// tests.
func ConcatResourceStatuses(values ...[]resource.Status) []resource.Status {
	var rs []resource.Status
	for _, v := range values {
		rs = append(rs, v...)
	}
	return rs
}

// AddSuffixes takes a slice of resource.Name objects and for each suffix,
// adds the suffix to every object, then returns the entire list.
func AddSuffixes(values []resource.Name, suffixes ...string) []resource.Name {
	var rNames []resource.Name

	for _, s := range suffixes {
		for _, v := range values {
			newName := resource.NewName(v.API, v.Name+s)
			rNames = append(rNames, newName)
		}
	}
	return rNames
}

// AddRemotes takes a slice of resource.Name objects and for each remote,
// adds the remote to every object, then returns the entire list.
func AddRemotes(values []resource.Name, remotes ...string) []resource.Name {
	var rNames []resource.Name

	for _, s := range remotes {
		for _, v := range values {
			v = v.PrependRemote(s)
			rNames = append(rNames, v)
		}
	}
	return rNames
}
