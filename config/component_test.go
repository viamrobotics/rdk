package config_test

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
)

func TestComponentValidate(t *testing.T) {
	var emptyConfig config.Component
	err := emptyConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig := config.Component{
		Name: "foo",
		Type: "arm",
	}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)
}

func TestComponentResourceName(t *testing.T) {
	for _, tc := range []struct {
		Name            string
		Config          config.Component
		ExpectedSubtype resource.Subtype
		ExpectedName    resource.Name
	}{
		{
			"all fields included",
			config.Component{
				Type: "arm",
				Name: "foo",
			},
			arm.Subtype,
			arm.Named("foo"),
		},
		{
			"missing subtype",
			config.Component{
				Name: "foo",
			},
			resource.Subtype{
				Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
				ResourceSubtype: resource.SubtypeName(""),
			},
			resource.Name{
				UUID: "51782993-c1f4-5e87-9fd8-be561f2444a2",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: resource.SubtypeName(""),
				},
				Name: "foo",
			},
		},
		{
			"sensor with no subtype",
			config.Component{
				Type: "sensor",
				Name: "foo",
			},
			resource.Subtype{
				Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
				ResourceSubtype: resource.SubtypeName(""),
			},
			resource.Name{
				UUID: "51782993-c1f4-5e87-9fd8-be561f2444a2",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: resource.SubtypeName(""),
				},
				Name: "foo",
			},
		},
		{
			"sensor with subtype",
			config.Component{
				Type:    "sensor",
				SubType: "compass",
				Name:    "foo",
			},
			resource.Subtype{
				Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
				ResourceSubtype: resource.ResourceSubtypeCompass,
			},
			resource.Name{
				UUID: "bd405f3f-da99-5adb-8637-1f914454da88",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: resource.ResourceSubtypeCompass,
				},
				Name: "foo",
			},
		},
		{
			"sensor missing name",
			config.Component{
				Type:    "sensor",
				SubType: "compass",
				Name:    "",
			},
			resource.Subtype{
				Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
				ResourceSubtype: resource.ResourceSubtypeCompass,
			},
			resource.Name{
				UUID: "3c4145b6-aff8-52b9-9b06-778abc940d0f",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: resource.ResourceSubtypeCompass,
				},
				Name: "",
			},
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			rName := tc.Config.ResourceName()
			test.That(t, rName.Subtype, test.ShouldResemble, tc.ExpectedSubtype)
			test.That(t, rName, test.ShouldResemble, tc.ExpectedName)
		})
	}
}

func TestComponentFlag(t *testing.T) {
	type MyStruct struct {
		Comp  config.Component `flag:"comp"`
		Comp2 config.Component `flag:"0"`
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
	test.That(t, myStruct.Comp.Type, test.ShouldEqual, config.ComponentType("foo"))
	test.That(t, myStruct.Comp.Host, test.ShouldEqual, "bar")
	test.That(t, myStruct.Comp.Attributes, test.ShouldResemble, config.AttributeMap{
		"wee": "woo",
	})

	err = utils.ParseFlags([]string{"main", "type=foo,host=bar,attr=wee:woo"}, &myStruct)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myStruct.Comp2.Type, test.ShouldEqual, config.ComponentType("foo"))
	test.That(t, myStruct.Comp2.Host, test.ShouldEqual, "bar")
	test.That(t, myStruct.Comp2.Attributes, test.ShouldResemble, config.AttributeMap{
		"wee": "woo",
	})
}

func TestParseComponentFlag(t *testing.T) {
	_, err := config.ParseComponentFlag("foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

	_, err = config.ParseComponentFlag("host=foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "required")

	_, err = config.ParseComponentFlag("port=foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

	comp, err := config.ParseComponentFlag("name=baz,type=foo,depends_on=")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, comp.DependsOn, test.ShouldResemble, []string(nil))

	comp, err = config.ParseComponentFlag("name=baz,type=foo,depends_on=|")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, comp.DependsOn, test.ShouldResemble, []string(nil))

	comp, err = config.ParseComponentFlag("name=baz,type=foo,depends_on=foo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, comp.DependsOn, test.ShouldResemble, []string{"foo"})

	comp, err = config.ParseComponentFlag("name=baz,type=foo,depends_on=foo|")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, comp.DependsOn, test.ShouldResemble, []string{"foo"})

	_, err = config.ParseComponentFlag("depends_on=foo,bar")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

	comp, err = config.ParseComponentFlag(
		"type=foo,host=bar,port=5,model=bar,name=baz,attr=wee:woo,subtype=who,depends_on=foo|bar,attr=one:two")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, comp.Name, test.ShouldEqual, "baz")
	test.That(t, comp.Host, test.ShouldEqual, "bar")
	test.That(t, comp.Port, test.ShouldEqual, 5)
	test.That(t, comp.Type, test.ShouldEqual, config.ComponentType("foo"))
	test.That(t, comp.SubType, test.ShouldEqual, "who")
	test.That(t, comp.Model, test.ShouldEqual, "bar")
	test.That(t, comp.DependsOn, test.ShouldResemble, []string{"foo", "bar"})
	test.That(t, comp.Attributes, test.ShouldResemble, config.AttributeMap{
		"wee": "woo",
		"one": "two",
	})
}
