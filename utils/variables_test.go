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
