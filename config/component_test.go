package config

import (
	"testing"

	"go.viam.com/robotcore/utils"

	"go.viam.com/test"
)

func TestComponentValidate(t *testing.T) {
	var emptyConfig Component
	err := emptyConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig := Component{
		Name: "foo",
	}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)
}

func TestComponentFlag(t *testing.T) {
	type MyStruct struct {
		Comp  Component `flag:"comp"`
		Comp2 Component `flag:"0"`
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

func TestParseComponentFlag(t *testing.T) {
	_, err := ParseComponentFlag("foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

	_, err = ParseComponentFlag("host=foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "required")

	_, err = ParseComponentFlag("port=foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

	comp, err := ParseComponentFlag("type=foo,host=bar,port=5,model=bar,name=baz,attr=wee:woo,subtype=who,attr=one:two")
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
