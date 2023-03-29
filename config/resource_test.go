package config_test

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/testutils"
)

var (
	extAPI        = resource.NewSubtype("acme", "component", "gizmo")
	extServiceAPI = resource.NewSubtype("acme", "service", "gadget")
)

func TestComponentValidate(t *testing.T) {
	t.Run("config invalid", func(t *testing.T) {
		var emptyConfig config.Component
		deps, err := emptyConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	})

	t.Run("config invalid name", func(t *testing.T) {
		validConfig := config.Component{
			Namespace: resource.ResourceNamespaceRDK,
			Name:      "foo arm",
			Type:      "arm",
			Model:     fakeModel,
		}
		deps, err := validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
		validConfig.Name = "foo.arm"
		deps, err = validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
		validConfig.Name = "9"
		deps, err = validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
	})
	t.Run("config valid", func(t *testing.T) {
		validConfig := config.Component{
			Namespace: resource.ResourceNamespaceRDK,
			Name:      "foo",
			Type:      "arm",
			Model:     fakeModel,
		}
		deps, err := validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		validConfig.Name = "A"
		deps, err = validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ConvertedAttributes", func(t *testing.T) {
		t.Run("config invalid", func(t *testing.T) {
			invalidConfig := config.Component{
				Namespace:           resource.ResourceNamespaceRDK,
				Name:                "foo",
				Type:                "base",
				Model:               fakeModel,
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
				Type:      "base",
				Model:     fakeModel,
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
			Name:  "foo",
			Type:  "arm",
			Model: fakeModel,
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
			Model:     fakeModel,
		}
		deps, err := validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConfig.Namespace, test.ShouldEqual, "acme")
	})

	t.Run("reserved character in name", func(t *testing.T) {
		invalidConfig := config.Component{
			Namespace: "acme",
			Name:      "fo:o",
			Type:      "arm",
			Model:     fakeModel,
		}
		_, err := invalidConfig.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
	})

	t.Run("reserved character in namespace", func(t *testing.T) {
		invalidConfig := config.Component{
			Namespace: "ac:me",
			Name:      "foo",
			Type:      "arm",
			Model:     fakeModel,
		}
		_, err := invalidConfig.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "reserved character : used")
	})

	//nolint:dupl
	t.Run("model variations", func(t *testing.T) {
		t.Run("config valid short model", func(t *testing.T) {
			shortConfig := config.Component{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "foo",
				Type:      "base",
				Model:     resource.Model{Name: "fake"},
			}
			deps, err := shortConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConfig.Model.Namespace, test.ShouldEqual, resource.ResourceNamespaceRDK)
			test.That(t, shortConfig.Model.Family, test.ShouldEqual, resource.DefaultModelFamilyName)
			test.That(t, shortConfig.Model.Name, test.ShouldEqual, resource.ModelName("fake"))
		})

		t.Run("config valid full model", func(t *testing.T) {
			shortConfig := config.Component{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "foo",
				Type:      "base",
				Model:     fakeModel,
			}
			deps, err := shortConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConfig.Model.Namespace, test.ShouldEqual, resource.ResourceNamespaceRDK)
			test.That(t, shortConfig.Model.Family, test.ShouldEqual, resource.DefaultModelFamilyName)
			test.That(t, shortConfig.Model.Name, test.ShouldEqual, resource.ModelName("fake"))
		})

		t.Run("config valid external model", func(t *testing.T) {
			shortConfig := config.Component{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "foo",
				Type:      "base",
				Model:     extModel,
			}
			deps, err := shortConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConfig.Model.Namespace, test.ShouldEqual, resource.Namespace("acme"))
			test.That(t, shortConfig.Model.Family, test.ShouldEqual, resource.ModelFamilyName("test"))
			test.That(t, shortConfig.Model.Name, test.ShouldEqual, resource.ModelName("model"))
		})
	})

	t.Run("api subtype namespace variations", func(t *testing.T) {
		t.Run("empty API and builtin type", func(t *testing.T) {
			shortConfig := config.Component{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "foo",
				Type:      "base",
				Model:     fakeModel,
			}
			deps, err := shortConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConfig.API, test.ShouldResemble, base.Subtype)
		})

		t.Run("filled API with builtin type", func(t *testing.T) {
			shortConfig := config.Component{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "foo",
				Type:      "base",
				Model:     fakeModel,
				API:       base.Subtype,
			}
			deps, err := shortConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConfig.API, test.ShouldResemble, base.Subtype)
		})

		t.Run("mismatched API", func(t *testing.T) {
			shortConfig := config.Component{
				Namespace: "acme",
				Name:      "foo",
				Type:      "gizmo",
				Model:     fakeModel,
				API:       base.Subtype,
			}
			deps, err := shortConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "do not match component api field")
		})

		t.Run("empty API with external type", func(t *testing.T) {
			shortConfig := config.Component{
				Namespace: "acme",
				Name:      "foo",
				Type:      "gizmo",
				Model:     fakeModel,
			}
			deps, err := shortConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConfig.API, test.ShouldResemble, extAPI)
		})

		t.Run("filled API with external type", func(t *testing.T) {
			shortConfig := config.Component{
				Namespace: "acme",
				Name:      "foo",
				Type:      "gizmo",
				Model:     fakeModel,
				API:       extAPI,
			}
			deps, err := shortConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConfig.API, test.ShouldResemble, extAPI)
		})

		t.Run("mismatched API with external type", func(t *testing.T) {
			shortConfig := config.Component{
				Namespace: "acme",
				Name:      "foo",
				Type:      "gizmo",
				Model:     fakeModel,
				API:       resource.NewDefaultSubtype("nada", resource.ResourceTypeComponent),
			}
			deps, err := shortConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "do not match component api field")
		})
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
				Model:     fakeModel,
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
		{
			"all fields included with external type",
			config.Component{
				Namespace: "acme",
				Type:      "gizmo",
				Name:      "foo",
				Model:     extModel,
			},
			extAPI,
			resource.NameFromSubtype(extAPI, "foo"),
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

	comp, err = config.ParseComponentFlag("type=foo,model=bar,name=baz,attr=wee:woo,depends_on=foo|bar,attr=one:two")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, comp.Name, test.ShouldEqual, "baz")
	test.That(t, comp.Type, test.ShouldEqual, resource.SubtypeName("foo"))
	test.That(t, comp.Model, test.ShouldResemble, resource.NewDefaultModel("bar"))
	test.That(t, comp.DependsOn, test.ShouldResemble, []string{"foo", "bar"})
	test.That(t, comp.Attributes, test.ShouldResemble, config.AttributeMap{
		"wee": "woo",
		"one": "two",
	})
}

func TestServiceValidate(t *testing.T) {
	t.Run("config invalid", func(t *testing.T) {
		var emptyConfig config.Service
		deps, err := emptyConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `"type" is required`)
	})

	t.Run("config valid", func(t *testing.T) {
		validConfig := config.Service{
			Name: "frame1",
			Type: "frame_system",
		}
		deps, err := validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		validConfig.Name = "A"
		deps, err = validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("config invalid name", func(t *testing.T) {
		validConfig := config.Service{
			Name: "frame 1",
			Type: "frame_system",
		}
		deps, err := validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
		validConfig.Name = "frame.1"
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
		validConfig.Name = "3"
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
	})

	t.Run("ConvertedAttributes", func(t *testing.T) {
		t.Run("config invalid", func(t *testing.T) {
			invalidConfig := config.Service{
				Name:                "frame1",
				Type:                "frame_system",
				ConvertedAttributes: &testutils.FakeConvertedAttributes{Thing: ""},
			}
			deps, err := invalidConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `"Thing" is required`)
		})

		t.Run("config valid", func(t *testing.T) {
			invalidConfig := config.Service{
				Name: "frame1",
				Type: "frame_system",
				ConvertedAttributes: &testutils.FakeConvertedAttributes{
					Thing: "i am a thing!",
				},
			}
			deps, err := invalidConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
		})
	})

	t.Run("Attributes", func(t *testing.T) {
		t.Run("config invalid", func(t *testing.T) {
			invalidConfig := config.Service{
				Name:       "frame1",
				Type:       "frame_system",
				Attributes: config.AttributeMap{"attr": &testutils.FakeConvertedAttributes{Thing: ""}},
			}
			_, err := invalidConfig.Validate("path")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `"Thing" is required`)
		})

		t.Run("config valid", func(t *testing.T) {
			invalidConfig := config.Service{
				Name: "frame1",
				Type: "frame_system",
				Attributes: config.AttributeMap{
					"attr": testutils.FakeConvertedAttributes{
						Thing: "i am a thing!",
					},
					"attr2": "boop",
				},
			}
			_, err := invalidConfig.Validate("path")
			test.That(t, err, test.ShouldBeNil)
		})
	})

	t.Run("no namespace", func(t *testing.T) {
		validConfig := config.Service{
			Name: "foo",
			Type: "thingy",
		}
		deps, err := validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConfig.Namespace, test.ShouldEqual, resource.ResourceNamespaceRDK)
	})

	t.Run("no name", func(t *testing.T) {
		testConfig := config.Service{
			Name: "",
			Type: "thingy",
		}
		deps, err := testConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testConfig.Name, test.ShouldEqual, resource.DefaultServiceName)
	})

	t.Run("with namespace", func(t *testing.T) {
		validConfig := config.Service{
			Namespace: "acme",
			Name:      "foo",
			Type:      "thingy",
		}
		deps, err := validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConfig.Namespace, test.ShouldEqual, "acme")
	})

	t.Run("reserved character in name", func(t *testing.T) {
		invalidConfig := config.Service{
			Namespace: "acme",
			Name:      "fo:o",
			Type:      "thingy",
		}
		_, err := invalidConfig.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
	})

	t.Run("reserved character in namespace", func(t *testing.T) {
		invalidConfig := config.Service{
			Namespace: "ac:me",
			Name:      "foo",
			Type:      "thingy",
		}
		_, err := invalidConfig.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "reserved character : used")
	})

	t.Run("default model to default", func(t *testing.T) {
		validConfig := config.Service{
			Namespace: "acme",
			Name:      "foo",
			Type:      "thingy",
		}

		deps, err := validConfig.Validate("path")
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConfig.Model.String(), test.ShouldEqual, "rdk:builtin:builtin")
	})

	//nolint:dupl
	t.Run("model variations", func(t *testing.T) {
		t.Run("config valid short model", func(t *testing.T) {
			shortConfig := config.Service{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "foo",
				Type:      "bar",
				Model:     resource.Model{Name: "fake"},
			}
			deps, err := shortConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConfig.Model.Namespace, test.ShouldEqual, resource.ResourceNamespaceRDK)
			test.That(t, shortConfig.Model.Family, test.ShouldEqual, resource.DefaultModelFamilyName)
			test.That(t, shortConfig.Model.Name, test.ShouldEqual, resource.ModelName("fake"))
		})

		t.Run("config valid full model", func(t *testing.T) {
			shortConfig := config.Service{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "foo",
				Type:      "bar",
				Model:     fakeModel,
			}
			deps, err := shortConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConfig.Model.Namespace, test.ShouldEqual, resource.ResourceNamespaceRDK)
			test.That(t, shortConfig.Model.Family, test.ShouldEqual, resource.DefaultModelFamilyName)
			test.That(t, shortConfig.Model.Name, test.ShouldEqual, resource.ModelName("fake"))
		})

		t.Run("config valid external model", func(t *testing.T) {
			shortConfig := config.Service{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "foo",
				Type:      "bar",
				Model:     extModel,
			}
			deps, err := shortConfig.Validate("path")
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConfig.Model.Namespace, test.ShouldEqual, resource.Namespace("acme"))
			test.That(t, shortConfig.Model.Family, test.ShouldEqual, resource.ModelFamilyName("test"))
			test.That(t, shortConfig.Model.Name, test.ShouldEqual, resource.ModelName("model"))
		})
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
				Name:      "motion1",
			},
			motion.Subtype,
			resource.NameFromSubtype(motion.Subtype, "motion1"),
		},
		{
			"all fields included with external type",
			config.Service{
				Namespace: "acme",
				Type:      "gadget",
				Name:      "foo",
				Model:     extModel,
			},
			extServiceAPI,
			resource.NameFromSubtype(extServiceAPI, "foo"),
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

	err = conf.Set("type=foo,model=bar,name=baz,attr=wee:woo,depends_on=foo|bar,attr=one:two")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conf.Type, test.ShouldEqual, resource.SubtypeName("foo"))
	test.That(t, conf.Attributes, test.ShouldResemble, config.AttributeMap{
		"wee": "woo",
		"one": "two",
	})
}
