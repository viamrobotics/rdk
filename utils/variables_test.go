package utils

import (
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
	// validNameRegex is the pattern that matches to a valid name.
	// The name must begin with a letter i.e. [a-zA-Z],
	// and the body can only contain 0 or more numbers, letters, dashes and underscores i.e. [-\w]*.
	name := "justLetters"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeTrue)
	name = "numbersAndLetters1"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeTrue)
	name = "letters-and-dashes"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeTrue)
	name = "letters_and_underscores"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeTrue)

	name = "1number"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeFalse)
	name = "a!"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeFalse)
	name = "s p a c e s"
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeFalse)
	name = "period."
	test.That(t, ValidNameRegex.MatchString(name), test.ShouldBeFalse)
}
