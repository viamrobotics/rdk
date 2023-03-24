package resource_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/utils"
)

var (
	fakeModel     = resource.NewDefaultModel("fake")
	extModel      = resource.NewModel("acme", "test", "model")
	extAPI        = resource.NewSubtype("acme", "component", "gizmo")
	extServiceAPI = resource.NewSubtype("acme", "service", "gadget")
)

func TestComponentValidate(t *testing.T) {
	t.Run("config invalid", func(t *testing.T) {
		var emptyConf resource.Config
		deps, err := emptyConf.Validate("path", resource.ResourceTypeComponent)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	})

	t.Run("config invalid name", func(t *testing.T) {
		validConf := resource.Config{
			DeprecatedNamespace: resource.ResourceNamespaceRDK,
			Name:                "foo arm",
			DeprecatedSubtype:   "arm",
			Model:               fakeModel,
		}
		deps, err := validConf.Validate("path", resource.ResourceTypeComponent)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
		validConf.Name = "foo.arm"
		deps, err = validConf.Validate("path", resource.ResourceTypeComponent)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
		validConf.Name = "9"
		deps, err = validConf.Validate("path", resource.ResourceTypeComponent)
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
		validConf := resource.Config{
			DeprecatedNamespace: resource.ResourceNamespaceRDK,
			Name:                "foo",
			DeprecatedSubtype:   "arm",
			Model:               fakeModel,
		}
		deps, err := validConf.Validate("path", resource.ResourceTypeComponent)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		validConf.Name = "A"
		deps, err = validConf.Validate("path", resource.ResourceTypeComponent)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ConvertedAttributes", func(t *testing.T) {
		t.Run("config invalid", func(t *testing.T) {
			invalidConf := resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "foo",
				DeprecatedSubtype:   "base",
				Model:               fakeModel,
				ConvertedAttributes: &testutils.FakeConvertedAttributes{Thing: ""},
			}
			deps, err := invalidConf.Validate("path", resource.ResourceTypeComponent)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `"Thing" is required`)
		})

		t.Run("config valid", func(t *testing.T) {
			invalidConf := resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "foo",
				DeprecatedSubtype:   "base",
				Model:               fakeModel,
				ConvertedAttributes: &testutils.FakeConvertedAttributes{
					Thing: "i am a thing!",
				},
			}
			deps, err := invalidConf.Validate("path", resource.ResourceTypeComponent)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
		})
	})

	t.Run("no namespace", func(t *testing.T) {
		validConf := resource.Config{
			Name:              "foo",
			DeprecatedSubtype: "arm",
			Model:             fakeModel,
		}
		deps, err := validConf.Validate("path", resource.ResourceTypeComponent)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConf.DeprecatedNamespace, test.ShouldEqual, resource.ResourceNamespaceRDK)
	})

	t.Run("with namespace", func(t *testing.T) {
		validConf := resource.Config{
			DeprecatedNamespace: "acme",
			Name:                "foo",
			DeprecatedSubtype:   "arm",
			Model:               fakeModel,
		}
		deps, err := validConf.Validate("path", resource.ResourceTypeComponent)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConf.DeprecatedNamespace, test.ShouldEqual, "acme")
	})

	t.Run("reserved character in name", func(t *testing.T) {
		invalidConf := resource.Config{
			DeprecatedNamespace: "acme",
			Name:                "fo:o",
			DeprecatedSubtype:   "arm",
			Model:               fakeModel,
		}
		_, err := invalidConf.Validate("path", resource.ResourceTypeComponent)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
	})

	t.Run("reserved character in namespace", func(t *testing.T) {
		invalidConf := resource.Config{
			DeprecatedNamespace: "ac:me",
			Name:                "foo",
			DeprecatedSubtype:   "arm",
			Model:               fakeModel,
		}
		_, err := invalidConf.Validate("path", resource.ResourceTypeComponent)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "reserved character : used")
	})

	//nolint:dupl
	t.Run("model variations", func(t *testing.T) {
		t.Run("config valid short model", func(t *testing.T) {
			shortConf := resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "foo",
				DeprecatedSubtype:   "base",
				Model:               resource.Model{Name: "fake"},
			}
			deps, err := shortConf.Validate("path", resource.ResourceTypeComponent)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.Model.Namespace, test.ShouldEqual, resource.ResourceNamespaceRDK)
			test.That(t, shortConf.Model.Family, test.ShouldEqual, resource.DefaultModelFamilyName)
			test.That(t, shortConf.Model.Name, test.ShouldEqual, resource.ModelName("fake"))
		})

		t.Run("config valid full model", func(t *testing.T) {
			shortConf := resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "foo",
				DeprecatedSubtype:   "base",
				Model:               fakeModel,
			}
			deps, err := shortConf.Validate("path", resource.ResourceTypeComponent)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.Model.Namespace, test.ShouldEqual, resource.ResourceNamespaceRDK)
			test.That(t, shortConf.Model.Family, test.ShouldEqual, resource.DefaultModelFamilyName)
			test.That(t, shortConf.Model.Name, test.ShouldEqual, resource.ModelName("fake"))
		})

		t.Run("config valid external model", func(t *testing.T) {
			shortConf := resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "foo",
				DeprecatedSubtype:   "base",
				Model:               extModel,
			}
			deps, err := shortConf.Validate("path", resource.ResourceTypeComponent)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.Model.Namespace, test.ShouldEqual, resource.Namespace("acme"))
			test.That(t, shortConf.Model.Family, test.ShouldEqual, resource.ModelFamilyName("test"))
			test.That(t, shortConf.Model.Name, test.ShouldEqual, resource.ModelName("model"))
		})
	})

	t.Run("api subtype namespace variations", func(t *testing.T) {
		t.Run("empty API and builtin type", func(t *testing.T) {
			shortConf := resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "foo",
				DeprecatedSubtype:   "base",
				Model:               fakeModel,
			}
			deps, err := shortConf.Validate("path", resource.ResourceTypeComponent)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.API, test.ShouldResemble, base.Subtype)
		})

		t.Run("filled API with builtin type", func(t *testing.T) {
			shortConf := resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "foo",
				DeprecatedSubtype:   "base",
				Model:               fakeModel,
				API:                 base.Subtype,
			}
			deps, err := shortConf.Validate("path", resource.ResourceTypeComponent)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.API, test.ShouldResemble, base.Subtype)
		})

		t.Run("empty API with external type", func(t *testing.T) {
			shortConf := resource.Config{
				DeprecatedNamespace: "acme",
				Name:                "foo",
				DeprecatedSubtype:   "gizmo",
				Model:               fakeModel,
			}
			deps, err := shortConf.Validate("path", resource.ResourceTypeComponent)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.API, test.ShouldResemble, extAPI)
		})

		t.Run("filled API with external type", func(t *testing.T) {
			shortConf := resource.Config{
				DeprecatedNamespace: "acme",
				Name:                "foo",
				DeprecatedSubtype:   "gizmo",
				Model:               fakeModel,
				API:                 extAPI,
			}
			deps, err := shortConf.Validate("path", resource.ResourceTypeComponent)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.API, test.ShouldResemble, extAPI)
		})
	})
}

func TestComponentResourceName(t *testing.T) {
	for _, tc := range []struct {
		Name            string
		Conf            resource.Config
		ExpectedSubtype resource.Subtype
		ExpectedName    resource.Name
		ExpectedError   string
	}{
		{
			"all fields included",
			resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				DeprecatedSubtype:   "arm",
				Name:                "foo",
				Model:               fakeModel,
			},
			arm.Subtype,
			arm.Named("foo"),
			"",
		},
		{
			"missing subtype",
			resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "foo",
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
			"name field for model missing",
		},
		{
			"sensor with no subtype",
			resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				DeprecatedSubtype:   "sensor",
				Name:                "foo",
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
			"name field for model missing",
		},
		{
			"sensor with subtype",
			resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				DeprecatedSubtype:   "movement_sensor",
				Name:                "foo",
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
			"name field for model missing",
		},
		{
			"sensor missing name",
			resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				DeprecatedSubtype:   "movement_sensor",
				Name:                "",
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
			`"name" is required`,
		},
		{
			"all fields included with external type",
			resource.Config{
				DeprecatedNamespace: "acme",
				DeprecatedSubtype:   "gizmo",
				Name:                "foo",
				Model:               extModel,
			},
			extAPI,
			resource.NameFromSubtype(extAPI, "foo"),
			"",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := tc.Conf.Validate("", resource.ResourceTypeComponent)
			if tc.ExpectedError == "" {
				test.That(t, err, test.ShouldBeNil)
				rName := tc.Conf.ResourceName()
				test.That(t, rName.Subtype, test.ShouldResemble, tc.ExpectedSubtype)
				test.That(t, rName, test.ShouldResemble, tc.ExpectedName)
				return
			}
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.ExpectedError)
		})
	}
}

func TestServiceValidate(t *testing.T) {
	t.Run("config invalid", func(t *testing.T) {
		var emptyConf resource.Config
		deps, err := emptyConf.Validate("path", resource.ResourceTypeService)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `subtype field`)
	})

	t.Run("config valid", func(t *testing.T) {
		validConf := resource.Config{
			Name:              "frame1",
			DeprecatedSubtype: "frame_system",
		}
		deps, err := validConf.Validate("path", resource.ResourceTypeService)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		validConf.Name = "A"
		deps, err = validConf.Validate("path", resource.ResourceTypeService)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("config invalid name", func(t *testing.T) {
		validConf := resource.Config{
			Name:              "frame 1",
			DeprecatedSubtype: "frame_system",
		}
		deps, err := validConf.Validate("path", resource.ResourceTypeService)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
		validConf.Name = "frame.1"
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
		validConf.Name = "3"
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
			invalidConf := resource.Config{
				Name:                "frame1",
				DeprecatedSubtype:   "frame_system",
				ConvertedAttributes: &testutils.FakeConvertedAttributes{Thing: ""},
			}
			deps, err := invalidConf.Validate("path", resource.ResourceTypeService)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `"Thing" is required`)
		})

		t.Run("config valid", func(t *testing.T) {
			invalidConf := resource.Config{
				Name:              "frame1",
				DeprecatedSubtype: "frame_system",
				ConvertedAttributes: &testutils.FakeConvertedAttributes{
					Thing: "i am a thing!",
				},
			}
			deps, err := invalidConf.Validate("path", resource.ResourceTypeService)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
		})
	})

	t.Run("Attributes", func(t *testing.T) {
		t.Run("config invalid", func(t *testing.T) {
			invalidConf := resource.Config{
				Name:              "frame1",
				DeprecatedSubtype: "frame_system",
				Attributes:        utils.AttributeMap{"attr": &testutils.FakeConvertedAttributes{Thing: ""}},
			}
			_, err := invalidConf.Validate("path", resource.ResourceTypeService)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `"Thing" is required`)
		})

		t.Run("config valid", func(t *testing.T) {
			invalidConf := resource.Config{
				Name:              "frame1",
				DeprecatedSubtype: "frame_system",
				Attributes: utils.AttributeMap{
					"attr": testutils.FakeConvertedAttributes{
						Thing: "i am a thing!",
					},
					"attr2": "boop",
				},
			}
			_, err := invalidConf.Validate("path", resource.ResourceTypeService)
			test.That(t, err, test.ShouldBeNil)
		})
	})

	t.Run("no namespace", func(t *testing.T) {
		validConf := resource.Config{
			Name:              "foo",
			DeprecatedSubtype: "thingy",
		}
		deps, err := validConf.Validate("path", resource.ResourceTypeService)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConf.DeprecatedNamespace, test.ShouldEqual, resource.ResourceNamespaceRDK)
	})

	t.Run("no name", func(t *testing.T) {
		testConfig := resource.Config{
			Name:              "",
			DeprecatedSubtype: "thingy",
		}
		deps, err := testConfig.Validate("path", resource.ResourceTypeService)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testConfig.Name, test.ShouldEqual, resource.DefaultServiceName)
	})

	t.Run("with namespace", func(t *testing.T) {
		validConf := resource.Config{
			DeprecatedNamespace: "acme",
			Name:                "foo",
			DeprecatedSubtype:   "thingy",
		}
		deps, err := validConf.Validate("path", resource.ResourceTypeService)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConf.DeprecatedNamespace, test.ShouldEqual, "acme")
	})

	t.Run("reserved character in name", func(t *testing.T) {
		invalidConf := resource.Config{
			DeprecatedNamespace: "acme",
			Name:                "fo:o",
			DeprecatedSubtype:   "thingy",
		}
		_, err := invalidConf.Validate("path", resource.ResourceTypeService)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
	})

	t.Run("reserved character in namespace", func(t *testing.T) {
		invalidConf := resource.Config{
			DeprecatedNamespace: "ac:me",
			Name:                "foo",
			DeprecatedSubtype:   "thingy",
		}
		_, err := invalidConf.Validate("path", resource.ResourceTypeService)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "reserved character : used")
	})

	t.Run("default model to default", func(t *testing.T) {
		validConf := resource.Config{
			DeprecatedNamespace: "acme",
			Name:                "foo",
			DeprecatedSubtype:   "thingy",
		}

		deps, err := validConf.Validate("path", resource.ResourceTypeService)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConf.Model.String(), test.ShouldEqual, "rdk:builtin:builtin")
	})

	//nolint:dupl
	t.Run("model variations", func(t *testing.T) {
		t.Run("config valid short model", func(t *testing.T) {
			shortConf := resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "foo",
				DeprecatedSubtype:   "bar",
				Model:               resource.Model{Name: "fake"},
			}
			deps, err := shortConf.Validate("path", resource.ResourceTypeService)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.Model.Namespace, test.ShouldEqual, resource.ResourceNamespaceRDK)
			test.That(t, shortConf.Model.Family, test.ShouldEqual, resource.DefaultModelFamilyName)
			test.That(t, shortConf.Model.Name, test.ShouldEqual, resource.ModelName("fake"))
		})

		t.Run("config valid full model", func(t *testing.T) {
			shortConf := resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "foo",
				DeprecatedSubtype:   "bar",
				Model:               fakeModel,
			}
			deps, err := shortConf.Validate("path", resource.ResourceTypeService)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.Model.Namespace, test.ShouldEqual, resource.ResourceNamespaceRDK)
			test.That(t, shortConf.Model.Family, test.ShouldEqual, resource.DefaultModelFamilyName)
			test.That(t, shortConf.Model.Name, test.ShouldEqual, resource.ModelName("fake"))
		})

		t.Run("config valid external model", func(t *testing.T) {
			shortConf := resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "foo",
				DeprecatedSubtype:   "bar",
				Model:               extModel,
			}
			deps, err := shortConf.Validate("path", resource.ResourceTypeService)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.Model.Namespace, test.ShouldEqual, resource.Namespace("acme"))
			test.That(t, shortConf.Model.Family, test.ShouldEqual, resource.ModelFamilyName("test"))
			test.That(t, shortConf.Model.Name, test.ShouldEqual, resource.ModelName("model"))
		})
	})
}

func TestServiceResourceName(t *testing.T) {
	for _, tc := range []struct {
		Name            string
		Conf            resource.Config
		ExpectedSubtype resource.Subtype
		ExpectedName    resource.Name
	}{
		{
			"all fields included",
			resource.Config{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				DeprecatedSubtype:   "motion",
				Name:                "motion1",
			},
			motion.Subtype,
			resource.NameFromSubtype(motion.Subtype, "motion1"),
		},
		{
			"all fields included with external type",
			resource.Config{
				DeprecatedNamespace: "acme",
				DeprecatedSubtype:   "gadget",
				Name:                "foo",
				Model:               extModel,
			},
			extServiceAPI,
			resource.NameFromSubtype(extServiceAPI, "foo"),
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := tc.Conf.Validate("", resource.ResourceTypeService)
			test.That(t, err, test.ShouldBeNil)
			rName := tc.Conf.ResourceName()
			test.That(t, rName.Subtype, test.ShouldResemble, tc.ExpectedSubtype)
			test.That(t, rName, test.ShouldResemble, tc.ExpectedName)
		})
	}
}
