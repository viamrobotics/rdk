package utils

import (
	"fmt"
	"strings"
	"testing"

	"go.viam.com/test"
)

func TestJSONTags(t *testing.T) {
	testStruct := struct {
		FirstVar   int     `json:"int_var"`
		SecondVar  float64 `json:"float_var"`
		ThirdVar   string  `json:"string_var"`
		FourthVar  bool    `json:"bool_var,omitempty"`
		FifthVar   int     `json:"-"`
		SixthVar   float64
		SeventhVar string `json:""`
		EigthVar   string `json:",omitempty"`
	}{}
	expectedNames := []TypedName{
		{"int_var", "int"},
		{"float_var", "float64"},
		{"string_var", "string"},
		{"bool_var", "bool"},
		{"SixthVar", "float64"},
		{"SeventhVar", "string"},
		{"EigthVar", "string"},
	}
	tagNames := JSONTags(testStruct)
	test.That(t, tagNames, test.ShouldHaveLength, 7)
	test.That(t, tagNames, test.ShouldResemble, expectedNames)
}

func TestNameValidations(t *testing.T) {
	tests := []struct {
		name             string
		shouldContainErr string
	}{
		{name: "a"},
		{name: "1"},
		{name: "justLetters"},
		{name: "numbersAndLetters1"},
		{name: "letters-and-dashes"},
		{name: "letters_and_underscores"},
		{name: "1number"},
		{name: "a!", shouldContainErr: "must only contain"},
		{name: "s p a c e s", shouldContainErr: "must only contain"},
		{name: "period.", shouldContainErr: "must only contain"},
		{name: "emoji👿", shouldContainErr: "must only contain"},
		{name: "-dashstart", shouldContainErr: "must only contain"},
		{name: strings.Repeat("a", 201), shouldContainErr: "or fewer"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.shouldContainErr == "" {
				test.That(t, ValidateResourceName(tc.name), test.ShouldBeNil)
				test.That(t, ValidateModuleName(tc.name), test.ShouldBeNil)
				test.That(t, ValidatePackageName(tc.name), test.ShouldBeNil)
				test.That(t, ValidateRemoteName(tc.name), test.ShouldBeNil)
			} else {
				test.That(t, fmt.Sprint(ValidateResourceName(tc.name)), test.ShouldContainSubstring, tc.shouldContainErr)
				test.That(t, fmt.Sprint(ValidateModuleName(tc.name)), test.ShouldContainSubstring, tc.shouldContainErr)
				test.That(t, fmt.Sprint(ValidatePackageName(tc.name)), test.ShouldContainSubstring, tc.shouldContainErr)
				test.That(t, fmt.Sprint(ValidateRemoteName(tc.name)), test.ShouldContainSubstring, tc.shouldContainErr)
			}
		})
	}
	// test differences between the validation functions
	name := strings.Repeat("a", 61)
	test.That(t, fmt.Sprint(ValidateResourceName(name)), test.ShouldContainSubstring, "or fewer")
	test.That(t, fmt.Sprint(ValidateRemoteName(name)), test.ShouldContainSubstring, "or fewer")
	test.That(t, ValidateModuleName(name), test.ShouldBeNil)
	test.That(t, ValidatePackageName(name), test.ShouldBeNil)
}
