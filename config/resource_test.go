package config_test

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/movementsensor"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/testutils"
)

func TestComponentValidate(t *testing.T) {
	t.Run("config invalid", func(t *testing.T) {
		var emptyConfig config.Component
		deps, err := emptyConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	})

	t.Run("config valid", func(t *testing.T) {
		validConfig := config.Component{
			Namespace: resource.ResourceNamespaceRDK,
			Name:      "foo",
			Type:      "arm",
		}
		deps, err := validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ConvertedAttributes", func(t *testing.T) {
		t.Run("config invalid", func(t *testing.T) {
			invalidConfig := config.Component{
				Namespace:           resource.ResourceNamespaceRDK,
				Name:                "foo",
				ConvertedAttributes: &testutils.FakeConvertedAttributes{Thing: ""},
			}
			deps, err := invalidConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `"Thing" is required`)
		})

		t.Run("config valid", func(t *testing.T) {
			invalidConfig := config.Component{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "foo",
				ConvertedAttributes: &testutils.FakeConvertedAttributes{
					Thing: "i am a thing!",
				},
			}
			deps, err := invalidConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
		})
	})

	t.Run("no namespace", func(t *testing.T) {
		validConfig := config.Component{
			Name: "foo",
			Type: "arm",
		}
		deps, err := validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConfig.Namespace, test.ShouldEqual, resource.ResourceNamespaceRDK)
	})

	t.Run("with namespace", func(t *testing.T) {
		validConfig := config.Component{
			Namespace: "acme",
			Name:      "foo",
			Type:      "arm",
		}
		deps, err := validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConfig.Namespace, test.ShouldEqual, "acme")
	})
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
				Namespace: resource.ResourceNamespaceRDK,
				Type:      "arm",
				Name:      "foo",
			},
			arm.Subtype,
			arm.Named("foo"),
		},
		{
			"missing subtype",
			config.Component{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "foo",
			},
			resource.Subtype{
				Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
				ResourceSubtype: resource.SubtypeName(""),
			},
			resource.Name{
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
				Namespace: resource.ResourceNamespaceRDK,
				Type:      "sensor",
				Name:      "foo",
			},
			resource.Subtype{
				Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
				ResourceSubtype: sensor.SubtypeName,
			},
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: sensor.SubtypeName,
				},
				Name: "foo",
			},
		},
		{
			"sensor with subtype",
			config.Component{
				Namespace: resource.ResourceNamespaceRDK,
				Type:      "movement_sensor",
				Name:      "foo",
			},
			resource.Subtype{
				Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
				ResourceSubtype: movementsensor.SubtypeName,
			},
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: movementsensor.SubtypeName,
				},
				Name: "foo",
			},
		},
		{
			"sensor missing name",
			config.Component{
				Namespace: resource.ResourceNamespaceRDK,
				Type:      "movement_sensor",
				Name:      "",
			},
			resource.Subtype{
				Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
				ResourceSubtype: movementsensor.SubtypeName,
			},
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: movementsensor.SubtypeName,
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

	err = utils.ParseFlags([]string{"main", "--comp=type=foo,attr=wee:woo"}, &myStruct)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myStruct.Comp.Type, test.ShouldEqual, resource.SubtypeName("foo"))
	test.That(t, myStruct.Comp.Attributes, test.ShouldResemble, config.AttributeMap{
		"wee": "woo",
	})

	err = utils.ParseFlags([]string{"main", "type=foo,attr=wee:woo"}, &myStruct)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myStruct.Comp2.Type, test.ShouldEqual, resource.SubtypeName("foo"))
	test.That(t, myStruct.Comp2.Attributes, test.ShouldResemble, config.AttributeMap{
		"wee": "woo",
	})
}

func TestParseComponentFlag(t *testing.T) {
	_, err := config.ParseComponentFlag("foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

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

	comp, err = config.ParseComponentFlag("type=foo,model=bar,name=baz,attr=wee:woo,subtype=who,depends_on=foo|bar,attr=one:two")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, comp.Name, test.ShouldEqual, "baz")
	test.That(t, comp.Type, test.ShouldEqual, resource.SubtypeName("foo"))
	test.That(t, comp.SubType, test.ShouldEqual, "who")
	test.That(t, comp.Model, test.ShouldEqual, "bar")
	test.That(t, comp.DependsOn, test.ShouldResemble, []string{"foo", "bar"})
	test.That(t, comp.Attributes, test.ShouldResemble, config.AttributeMap{
		"wee": "woo",
		"one": "two",
	})
}

func TestServiceValidate(t *testing.T) {
	t.Run("config invalid", func(t *testing.T) {
		var emptyConfig config.Service
		err := emptyConfig.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `"type" is required`)
	})

	t.Run("config valid", func(t *testing.T) {
		validConfig := config.Service{
			Type: "frame_system",
		}
		test.That(t, validConfig.Validate("path"), test.ShouldBeNil)
	})

	t.Run("ConvertedAttributes", func(t *testing.T) {
		t.Run("config invalid", func(t *testing.T) {
			invalidConfig := config.Service{
				Type:                "frame_system",
				ConvertedAttributes: &testutils.FakeConvertedAttributes{Thing: ""},
			}
			err := invalidConfig.Validate("path")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `"Thing" is required`)
		})

		t.Run("config valid", func(t *testing.T) {
			invalidConfig := config.Service{
				Type: "frame_system",
				ConvertedAttributes: &testutils.FakeConvertedAttributes{
					Thing: "i am a thing!",
				},
			}
			err := invalidConfig.Validate("path")
			test.That(t, err, test.ShouldBeNil)
		})
	})

	t.Run("Attributes", func(t *testing.T) {
		t.Run("config invalid", func(t *testing.T) {
			invalidConfig := config.Service{
				Type:       "frame_system",
				Attributes: config.AttributeMap{"attr": &testutils.FakeConvertedAttributes{Thing: ""}},
			}
			err := invalidConfig.Validate("path")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `"Thing" is required`)
		})

		t.Run("config valid", func(t *testing.T) {
			invalidConfig := config.Service{
				Type: "frame_system",
				Attributes: config.AttributeMap{
					"attr": testutils.FakeConvertedAttributes{
						Thing: "i am a thing!",
					},
					"attr2": "boop",
				},
			}
			err := invalidConfig.Validate("path")
			test.That(t, err, test.ShouldBeNil)
		})
	})

	t.Run("no namespace", func(t *testing.T) {
		validConfig := config.Service{
			Name: "foo",
			Type: "thingy",
		}
		test.That(t, validConfig.Validate("path"), test.ShouldBeNil)
		test.That(t, validConfig.Namespace, test.ShouldEqual, resource.ResourceNamespaceRDK)
	})

	t.Run("with namespace", func(t *testing.T) {
		validConfig := config.Service{
			Namespace: "acme",
			Name:      "foo",
			Type:      "thingy",
		}
		test.That(t, validConfig.Validate("path"), test.ShouldBeNil)
		test.That(t, validConfig.Namespace, test.ShouldEqual, "acme")
	})
}

func TestServiceResourceName(t *testing.T) {
	for _, tc := range []struct {
		Name            string
		Config          config.Service
		ExpectedSubtype resource.Subtype
		ExpectedName    resource.Name
	}{
		{
			"all fields included",
			config.Service{
				Namespace: resource.ResourceNamespaceRDK,
				Type:      "motion",
			},
			motion.Subtype,
			resource.NameFromSubtype(motion.Subtype, ""),
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			rName := tc.Config.ResourceName()
			test.That(t, rName.Subtype, test.ShouldResemble, tc.ExpectedSubtype)
			test.That(t, rName, test.ShouldResemble, tc.ExpectedName)
		})
	}
}

func TestSet(t *testing.T) {
	conf := &config.Service{}
	err := conf.Set("foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

	err = conf.Set("type=foo,model=bar,name=baz,attr=wee")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

	err = conf.Set("type=foo,model=bar,name=baz,attr=wee:woo,subtype=who,depends_on=foo|bar,attr=one:two")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conf.Type, test.ShouldEqual, config.ServiceType("foo"))
	test.That(t, conf.Attributes, test.ShouldResemble, config.AttributeMap{
		"wee": "woo",
		"one": "two",
	})
}
