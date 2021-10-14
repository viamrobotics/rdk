package config

import (
	"go.viam.com/core/testutils"
	"go.viam.com/test"
	"testing"
)

var sampleAttributeMap = AttributeMap{
	"ok_boolean_false": false,
	"ok_boolean_true": true,
	"bad_boolean_false": 0,
	"bad_boolean_true": "true",
	"good_int_slice": []interface{}{1,2,3},
	"bad_int_slice": "this is not an int slice",
	"bad_int_slice_2": []interface{}{1,2,"3"},
	"good_string_slice": []interface{}{"1","2","3"},
	"bad_string_slice": 123,
	"bad_string_slice_2": []interface{}{"1","2",3},
}

func TestAttributeMap(t *testing.T) {
	// TODO: As a general revisit, perhaps AttributeMap should return
	// errors rather than panicking?

	// AttributeMap.Bool tests

	// AttributeMap.Bool properly returns for boolean value of True
	b := sampleAttributeMap.Bool("ok_boolean_true", false)
	test.That(t, b, test.ShouldBeTrue)
	// AttributeMap.Bool properly returns for boolean value of False
	b = sampleAttributeMap.Bool("ok_boolean_false", false)
	test.That(t, b, test.ShouldBeFalse)
	// AttributeMap.Bool panics for non-boolean values
	badTrueGetter := func () {
		sampleAttributeMap.Bool("bad_boolean_true", false)
	}
	badFalseGetter := func () {
		sampleAttributeMap.Bool("bad_boolean_false", false)
	}
	didPanic, err := testutils.RunForPanic(badTrueGetter)
	test.That(t, didPanic, test.ShouldBeTrue)
	test.That(t, err.Error(), test.ShouldContainSubstring, "wanted a bool")
	didPanic, err = testutils.RunForPanic(badFalseGetter)
	test.That(t, didPanic, test.ShouldBeTrue)
	test.That(t, err.Error(), test.ShouldContainSubstring, "wanted a bool")
	// AttributeMap.Bool provides default boolean value when key is missing
	b = sampleAttributeMap.Bool("junk_key", false)
	test.That(t, b, test.ShouldBeFalse)
	b = sampleAttributeMap.Bool("junk_key", true)

	// TODO: write tests for below functions
	// AttributeMap.Float64
	// AttributeMap.Int
	// AttributeMap.String

	// AttributeMap.IntSlice
	// AttributeMap.IntSlice properly returns an int slice
	iSlice := sampleAttributeMap.IntSlice("good_int_slice")
	test.That(t, iSlice, test.ShouldResemble, []int{1,2,3})
	// AttributeMap.IntSlice panics when corresponding value is
	// not a slice of all integers
	badIntSliceGetter1 := func () {
		sampleAttributeMap.IntSlice("bad_int_slice")
	}
	badIntSliceGetter2 := func () {
		sampleAttributeMap.IntSlice("bad_int_slice_2")
	}
	didPanic, err = testutils.RunForPanic(badIntSliceGetter1)
	test.That(t, didPanic, test.ShouldBeTrue)
	test.That(t, err.Error(), test.ShouldContainSubstring, "wanted a []int")
	didPanic, err = testutils.RunForPanic(badIntSliceGetter2)
	test.That(t, didPanic, test.ShouldBeTrue)
	test.That(t, err.Error(), test.ShouldContainSubstring, "values in (bad_int_slice_2) need to be ints")

	// AttributeMap.IntSlice
	// AttributeMap.IntSlice properly returns an int slice
	sSlice := sampleAttributeMap.StringSlice("good_string_slice")
	test.That(t, sSlice, test.ShouldResemble, []string{"1","2","3"})
	// AttributeMap.IntSlice panics when corresponding value is
	// not a slice of all integers
	badStringSliceGetter1 := func () {
		sampleAttributeMap.StringSlice("bad_string_slice")
	}
	badStringSliceGetter2 := func () {
		sampleAttributeMap.StringSlice("bad_string_slice_2")
	}
	didPanic, err = testutils.RunForPanic(badStringSliceGetter1)
	test.That(t, didPanic, test.ShouldBeTrue)
	test.That(t, err.Error(), test.ShouldContainSubstring, "wanted a []string")
	didPanic, err = testutils.RunForPanic(badStringSliceGetter2)
	test.That(t, didPanic, test.ShouldBeTrue)
	test.That(t, err.Error(), test.ShouldContainSubstring, "values in (bad_string_slice_2) need to be strings")

}
