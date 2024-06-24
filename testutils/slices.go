package testutils

import (
	"cmp"
	"slices"
	"testing"

	"go.viam.com/test"
)

// VerifySameElements asserts that two slices contain the same elements without
// considering order.
func VerifySameElements[E cmp.Ordered](tb testing.TB, actual, expected []E) {
	tb.Helper()

	actualSorted := make([]E, len(actual))
	copy(actualSorted, actual)
	expectedSorted := make([]E, len(expected))
	copy(expectedSorted, expected)

	slices.Sort(actualSorted)
	slices.Sort(expectedSorted)

	test.That(tb, actualSorted, test.ShouldResemble, expectedSorted)
}
