package packages

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go.viam.com/test"
)

func TestPackagePathVisitor(t *testing.T) {
	viamDotDir := filepath.Join(os.Getenv("HOME"), ".viam")
	dataDir := ".data"

	testStringNoRef := "some/path/file_name.txt"
	testStringRef := "${packages.custom_package}/file_name.txt"
	testStringMlModel := "${packages.ml_models.custom_package}/file_name.txt"
	testStringModule := "${packages.modules.custom_package}/file_name.txt"
	testBadPlaceholder := "${packages.my_model.ml.big}/file.txt"
	testInt := 17

	packageMap := make(map[string]string)
	packageMap["packages.custom_package"] = filepath.Join(viamDotDir, "packages", "custom_package")
	packageMap["packages.ml_models.custom_package"] = filepath.Join(
		viamDotDir, "packages", dataDir, "ml_models", "orgID-custom_package-latest")
	packageMap["packages.modules.custom_package"] = filepath.Join(
		viamDotDir, "packages", dataDir, "modules", "orgID-custom_package-latest")

	testStringRefOutput := filepath.Join(packageMap["packages.custom_package"], "file_name.txt")
	invalidPackageErr := "invalid package placeholder path"
	testCases := []struct {
		desc        string
		input       interface{}
		expected    interface{}
		errorString *string
	}{
		{
			"visit string with package reference",
			testStringRef,
			testStringRefOutput,
			nil,
		},
		{
			"visit string without package reference",
			testStringNoRef,
			testStringNoRef,
			nil,
		},
		{
			"visit string with package ml model reference",
			testStringMlModel,
			filepath.Join(packageMap["packages.ml_models.custom_package"], "file_name.txt"),
			nil,
		},
		{
			"visit string with package module reference",
			testStringModule,
			filepath.Join(packageMap["packages.modules.custom_package"], "file_name.txt"),
			nil,
		},
		{
			"visit string without package reference",
			testStringNoRef,
			testStringNoRef,
			nil,
		},
		{
			"visit pointer to string with package reference",
			&testStringRef,
			&testStringRefOutput,
			nil,
		},
		{
			"visit pointer to string without package reference",
			&testStringNoRef,
			&testStringNoRef,
			nil,
		},
		{
			"visit non-string type",
			testInt,
			testInt,
			nil,
		},
		{
			"visit pointer to non-string type",
			&testInt,
			&testInt,
			nil,
		},
		{
			"visit placeholder with bad reference",
			testBadPlaceholder,
			testBadPlaceholder,
			&invalidPackageErr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			v := NewPackagePathVisitor(NewNoopManager(), packageMap)
			actual, err := v.Visit(tc.input)
			hasErr := tc.errorString
			if hasErr == nil {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, *hasErr)
			}

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
