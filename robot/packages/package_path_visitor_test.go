package packages

import (
	"reflect"
	"testing"

	"go.viam.com/test"
)

func TestPackagePathVisitor(t *testing.T) {
	testStringNoRef := "some/path/file_name.txt"
	testStringRef := "${packages.custom_package}/file_name.txt"
	testStringRefReplaced := "custom_package/file_name.txt"

	testCases := []struct {
		desc     string
		input    interface{}
		expected interface{}
	}{
		{
			"visit string with package reference",
			testStringRef,
			testStringRefReplaced,
		},
		{
			"visit string without package reference",
			testStringNoRef,
			testStringNoRef,
		},
		{
			"visit pointer to string with package reference",
			&testStringRef,
			&testStringRefReplaced,
		},
		{
			"visit pointer to string without package reference",
			&testStringNoRef,
			&testStringNoRef,
		},
		{
			"visit non-string type",
			17,
			17,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			v := NewPackagePathVisitor(NewNoopManager())
			actual, err := v.Visit(tc.input)
			test.That(t, err, test.ShouldBeNil)

			if reflect.TypeOf(tc.input).Kind() == reflect.Ptr {
				if reflect.TypeOf(actual).Kind() != reflect.Ptr {
					t.Fatal("input was pointer, but output was not")
				}
				if reflect.TypeOf(tc.input).Elem().Kind() != reflect.String {
					// For now this test doesn't cover comparisons of pointer
					// values for non-string types
					return
				}

				actual = *actual.(*string)
				tc.expected = *tc.expected.(*string)
			}
			if actual != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}
