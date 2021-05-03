package api

import (
	"testing"

	"go.viam.com/robotcore/utils"

	"github.com/edaniels/test"
)

func TestComponentConfigValidate(t *testing.T) {
	var emptyConfig ComponentConfig
	err := emptyConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig := ComponentConfig{
		Name: "foo",
	}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)
}

func TestComponentConfigFlag(t *testing.T) {
	type MyStruct struct {
		Comp  ComponentConfig `flag:"comp"`
		Comp2 ComponentConfig `flag:"0"`
	}
	var myStruct MyStruct
	err := utils.ParseFlags([]string{"main", "--comp=foo"}, &myStruct)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

	err = utils.ParseFlags([]string{"main", "--comp=host=foo"}, &myStruct)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "required")

	err = utils.ParseFlags([]string{"main", "--comp=type=foo,host=bar,attr=wee:woo"}, &myStruct)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myStruct.Comp.Type, test.ShouldEqual, ComponentType("foo"))
	test.That(t, myStruct.Comp.Host, test.ShouldEqual, "bar")
	test.That(t, myStruct.Comp.Attributes, test.ShouldResemble, AttributeMap{
		"wee": "woo",
	})

	err = utils.ParseFlags([]string{"main", "type=foo,host=bar,attr=wee:woo"}, &myStruct)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myStruct.Comp2.Type, test.ShouldEqual, ComponentType("foo"))
	test.That(t, myStruct.Comp2.Host, test.ShouldEqual, "bar")
	test.That(t, myStruct.Comp2.Attributes, test.ShouldResemble, AttributeMap{
		"wee": "woo",
	})
}

func TestParseComponentConfigFlag(t *testing.T) {
	_, err := ParseComponentConfigFlag("foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

	_, err = ParseComponentConfigFlag("host=foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "required")

	_, err = ParseComponentConfigFlag("port=foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

	comp, err := ParseComponentConfigFlag("type=foo,host=bar,port=5,model=bar,name=baz,attr=wee:woo,subtype=who,attr=one:two")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, comp.Name, test.ShouldEqual, "baz")
	test.That(t, comp.Host, test.ShouldEqual, "bar")
	test.That(t, comp.Port, test.ShouldEqual, 5)
	test.That(t, comp.Type, test.ShouldEqual, ComponentType("foo"))
	test.That(t, comp.SubType, test.ShouldEqual, "who")
	test.That(t, comp.Model, test.ShouldEqual, "bar")
	test.That(t, comp.Attributes, test.ShouldResemble, AttributeMap{
		"wee": "woo",
		"one": "two",
	})
}
