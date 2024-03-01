package utils

import (
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

func TestValidNameRegex(t *testing.T) {
	name := "justLetters"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeTrue)
	name = "numbersAndLetters1"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeTrue)
	name = "letters-and-dashes"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeTrue)
	name = "letters_and_underscores"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeTrue)
	name = "1number"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeTrue)

	name = "a!"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeFalse)
	name = "s p a c e s"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeFalse)
	name = "period."
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeFalse)
	name = strings.Repeat("a", 61)
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeFalse)
}

func TestValidNameErrorMsg(t *testing.T) {
	name := "!"
	test.That(t, ErrInvalidName(name).Error(), test.ShouldContainSubstring, "must start with a letter or number")
	name = strings.Repeat("a", 61)
	test.That(t, ErrInvalidName(name).Error(), test.ShouldContainSubstring, "must be less than 60 characters")
}
