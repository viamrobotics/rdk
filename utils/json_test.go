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
	}{}
	expectedNames := []string{"int_var", "float_var", "string_var", "bool_var", "SixthVar", "SeventhVar"}
	tagNames := JSONTags(testStruct)
	test.That(t, tagNames, test.ShouldHaveLength, 6)
	test.That(t, tagNames, test.ShouldResemble, expectedNames)
}
