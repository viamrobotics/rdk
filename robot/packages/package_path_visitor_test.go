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
	testInt := 17

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
			testInt,
			testInt,
		},
		{
			"visit pointer to non-string type",
			&testInt,
			&testInt,
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

				tc.expected = reflect.Indirect(reflect.ValueOf(tc.expected)).Interface()
				actual = reflect.Indirect(reflect.ValueOf(actual)).Interface()
			}

			test.That(t, actual, test.ShouldEqual, tc.expected)
		})
	}
}
