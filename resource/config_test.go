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
)

var (
	acmeAPINamespace = resource.APINamespace("acme")
	fakeModel        = resource.DefaultModelFamily.WithModel("fake")
	extModel         = resource.ModelNamespace("acme").WithFamily("test").WithModel("model")
	extAPI           = acmeAPINamespace.WithComponentType("gizmo")
	extServiceAPI    = acmeAPINamespace.WithServiceType("gadget")
)

func TestComponentValidate(t *testing.T) {
	t.Run("config invalid", func(t *testing.T) {
		var emptyConf resource.Config
		deps, err := emptyConf.Validate("path", resource.APITypeComponentName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "name")
	})

	t.Run("config invalid name", func(t *testing.T) {
		validConf := resource.Config{
			Name: "foo arm",

			Model: fakeModel,
		}
		deps, err := validConf.Validate("path", resource.APITypeComponentName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
		validConf.Name = "foo.arm"
		deps, err = validConf.Validate("path", resource.APITypeComponentName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
		validConf.Name = "9"
		deps, err = validConf.Validate("path", resource.APITypeComponentName)
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
			Name:  "foo",
			API:   arm.API,
			Model: fakeModel,
		}
		deps, err := validConf.Validate("path", resource.APITypeComponentName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		validConf.Name = "A"
		deps, err = validConf.Validate("path", resource.APITypeComponentName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ConvertedAttributes", func(t *testing.T) {
		t.Run("config invalid", func(t *testing.T) {
			invalidConf := resource.Config{
				Name:                "foo",
				API:                 base.API,
				Model:               fakeModel,
				ConvertedAttributes: &testutils.FakeConvertedAttributes{Thing: ""},
			}
			deps, err := invalidConf.Validate("path", resource.APITypeComponentName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "Thing")
		})

		t.Run("config valid", func(t *testing.T) {
			invalidConf := resource.Config{
				Name:  "foo",
				API:   arm.API,
				Model: fakeModel,
				ConvertedAttributes: &testutils.FakeConvertedAttributes{
					Thing: "i am a thing!",
				},
			}
			deps, err := invalidConf.Validate("path", resource.APITypeComponentName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
		})
	})

	t.Run("no namespace", func(t *testing.T) {
		validConf := resource.Config{
			Name:  "foo",
			API:   resource.APINamespace("").WithComponentType("foo"),
			Model: fakeModel,
		}
		deps, err := validConf.Validate("path", resource.APITypeComponentName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConf.API, test.ShouldResemble, resource.APINamespaceRDK.WithComponentType("foo"))
	})

	t.Run("with namespace", func(t *testing.T) {
		validConf := resource.Config{
			Name:  "foo",
			API:   resource.APINamespace("acme").WithComponentType("foo"),
			Model: fakeModel,
		}
		deps, err := validConf.Validate("path", resource.APITypeComponentName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConf.API, test.ShouldResemble, resource.APINamespace("acme").WithComponentType("foo"))
	})

	t.Run("reserved character in name", func(t *testing.T) {
		invalidConf := resource.Config{
			Name:  "fo:o",
			Model: fakeModel,
		}
		_, err := invalidConf.Validate("path", resource.APITypeComponentName)
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
			Name:  "foo",
			API:   resource.APINamespace("ac:me").WithComponentType("foo"),
			Model: fakeModel,
		}
		_, err := invalidConf.Validate("path", resource.APITypeComponentName)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "reserved character : used")
	})

	t.Run("model variations", func(t *testing.T) {
		t.Run("config valid short model", func(t *testing.T) {
			shortConf := resource.Config{
				Name:  "foo",
				API:   base.API,
				Model: resource.Model{Name: "fake"},
			}
			deps, err := shortConf.Validate("path", resource.APITypeComponentName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.Model.Family, test.ShouldResemble, resource.DefaultModelFamily)
			test.That(t, shortConf.Model.Name, test.ShouldEqual, "fake")
		})

		t.Run("config valid full model", func(t *testing.T) {
			shortConf := resource.Config{
				Name:  "foo",
				API:   base.API,
				Model: fakeModel,
			}
			deps, err := shortConf.Validate("path", resource.APITypeComponentName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.Model.Family, test.ShouldResemble, resource.DefaultModelFamily)
			test.That(t, shortConf.Model.Name, test.ShouldEqual, "fake")
		})

		t.Run("config valid external model", func(t *testing.T) {
			shortConf := resource.Config{
				Name:  "foo",
				API:   base.API,
				Model: extModel,
			}
			deps, err := shortConf.Validate("path", resource.APITypeComponentName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.Model.Family, test.ShouldResemble, resource.NewModelFamily("acme", "test"))
			test.That(t, shortConf.Model.Name, test.ShouldEqual, "model")
		})
	})

	t.Run("api subtype namespace variations", func(t *testing.T) {
		t.Run("empty API and builtin type", func(t *testing.T) {
			shortConf := resource.Config{
				Name:  "foo",
				API:   resource.APINamespace("").WithType("").WithSubtype("base"),
				Model: fakeModel,
			}
			deps, err := shortConf.Validate("path", resource.APITypeComponentName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.API, test.ShouldResemble, base.API)
		})

		t.Run("filled API with builtin type", func(t *testing.T) {
			shortConf := resource.Config{
				Name:  "foo",
				Model: fakeModel,
				API:   base.API,
			}
			deps, err := shortConf.Validate("path", resource.APITypeComponentName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.API, test.ShouldResemble, base.API)
		})

		t.Run("empty API with external type", func(t *testing.T) {
			shortConf := resource.Config{
				Name:  "foo",
				API:   resource.APINamespace("acme").WithType("").WithSubtype("gizmo"),
				Model: fakeModel,
			}
			deps, err := shortConf.Validate("path", resource.APITypeComponentName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.API, test.ShouldResemble, extAPI)
		})

		t.Run("filled API with external type", func(t *testing.T) {
			shortConf := resource.Config{
				Name:  "foo",
				Model: fakeModel,
				API:   extAPI,
			}
			deps, err := shortConf.Validate("path", resource.APITypeComponentName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.API, test.ShouldResemble, extAPI)
		})
	})
}

func TestComponentResourceName(t *testing.T) {
	for _, tc := range []struct {
		Name          string
		Conf          resource.Config
		ExpectedAPI   resource.API
		ExpectedName  resource.Name
		ExpectedError string
	}{
		{
			"all fields included",
			resource.Config{
				Name:  "foo",
				API:   arm.API,
				Model: fakeModel,
			},
			arm.API,
			arm.Named("foo"),
			"",
		},
		{
			"missing subtype",
			resource.Config{
				API:  resource.APINamespaceRDK.WithType("").WithSubtype(""),
				Name: "foo",
			},
			resource.API{
				Type:        resource.APIType{Namespace: resource.APINamespaceRDK, Name: resource.APITypeComponentName},
				SubtypeName: "",
			},
			resource.Name{
				API: resource.API{
					Type:        resource.APIType{Namespace: resource.APINamespaceRDK, Name: resource.APITypeComponentName},
					SubtypeName: "",
				},
				Name: "foo",
			},
			"name",
		},
		{
			"sensor with no subtype",
			resource.Config{
				API:  resource.APINamespaceRDK.WithComponentType("sensor"),
				Name: "foo",
			},
			resource.API{
				Type:        resource.APIType{Namespace: resource.APINamespaceRDK, Name: resource.APITypeComponentName},
				SubtypeName: sensor.SubtypeName,
			},
			resource.Name{
				API: resource.API{
					Type:        resource.APIType{Namespace: resource.APINamespaceRDK, Name: resource.APITypeComponentName},
					SubtypeName: sensor.SubtypeName,
				},
				Name: "foo",
			},
			"name",
		},
		{
			"sensor with subtype",
			resource.Config{
				API:  resource.APINamespaceRDK.WithComponentType("movement_sensor"),
				Name: "foo",
			},
			resource.API{
				Type:        resource.APIType{Namespace: resource.APINamespaceRDK, Name: resource.APITypeComponentName},
				SubtypeName: movementsensor.SubtypeName,
			},
			resource.Name{
				API: resource.API{
					Type:        resource.APIType{Namespace: resource.APINamespaceRDK, Name: resource.APITypeComponentName},
					SubtypeName: movementsensor.SubtypeName,
				},
				Name: "foo",
			},
			"name",
		},
		{
			"sensor missing name",
			resource.Config{
				API:  resource.APINamespaceRDK.WithComponentType("sensor"),
				Name: "",
			},
			resource.API{
				Type:        resource.APIType{Namespace: resource.APINamespaceRDK, Name: resource.APITypeComponentName},
				SubtypeName: movementsensor.SubtypeName,
			},
			resource.Name{
				API: resource.API{
					Type:        resource.APIType{Namespace: resource.APINamespaceRDK, Name: resource.APITypeComponentName},
					SubtypeName: movementsensor.SubtypeName,
				},
				Name: "",
			},
			"name",
		},
		{
			"all fields included with external type",
			resource.Config{
				Name:  "foo",
				API:   extAPI,
				Model: extModel,
			},
			extAPI,
			resource.NewName(extAPI, "foo"),
			"",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := tc.Conf.Validate("", resource.APITypeComponentName)
			if tc.ExpectedError == "" {
				test.That(t, err, test.ShouldBeNil)
				rName := tc.Conf.ResourceName()
				test.That(t, rName.API, test.ShouldResemble, tc.ExpectedAPI)
				test.That(t, rName, test.ShouldResemble, tc.ExpectedName)
				return
			}
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, tc.ExpectedError)
		})
	}
}

func TestServiceValidate(t *testing.T) {
	t.Run("config invalid", func(t *testing.T) {
		var emptyConf resource.Config
		deps, err := emptyConf.Validate("path", resource.APITypeServiceName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `subtype field`)
	})

	t.Run("config valid", func(t *testing.T) {
		validConf := resource.Config{
			Name: "frame1",
			API:  resource.APINamespaceRDK.WithServiceType("frame_system"),
		}
		deps, err := validConf.Validate("path", resource.APITypeServiceName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		validConf.Name = "A"
		deps, err = validConf.Validate("path", resource.APITypeServiceName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("config invalid name", func(t *testing.T) {
		validConf := resource.Config{
			Name: "frame 1",
			API:  resource.APINamespaceRDK.WithServiceType("frame_system"),
		}
		deps, err := validConf.Validate("path", resource.APITypeServiceName)
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
				API:                 resource.APINamespaceRDK.WithServiceType("frame_system"),
				ConvertedAttributes: &testutils.FakeConvertedAttributes{Thing: ""},
			}
			deps, err := invalidConf.Validate("path", resource.APITypeServiceName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `Field: "Thing"`)
		})

		t.Run("config valid", func(t *testing.T) {
			invalidConf := resource.Config{
				Name: "frame1",
				API:  resource.APINamespaceRDK.WithServiceType("frame_system"),
				ConvertedAttributes: &testutils.FakeConvertedAttributes{
					Thing: "i am a thing!",
				},
			}
			deps, err := invalidConf.Validate("path", resource.APITypeServiceName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
		})
	})

	t.Run("no namespace", func(t *testing.T) {
		validConf := resource.Config{
			Name: "foo",
			API:  resource.APINamespace("").WithServiceType("frame_system"),
		}
		deps, err := validConf.Validate("path", resource.APITypeServiceName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("no name", func(t *testing.T) {
		testConfig := resource.Config{
			Name: "",
			API:  resource.APINamespace("").WithServiceType("frame_system"),
		}
		deps, err := testConfig.Validate("path", resource.APITypeServiceName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testConfig.Name, test.ShouldEqual, resource.DefaultServiceName)
	})

	t.Run("with namespace", func(t *testing.T) {
		validConf := resource.Config{
			Name: "foo",
			API:  acmeAPINamespace.WithServiceType("thingy"),
		}
		deps, err := validConf.Validate("path", resource.APITypeServiceName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("reserved character in name", func(t *testing.T) {
		invalidConf := resource.Config{
			Name: "fo:o",
			API:  acmeAPINamespace.WithServiceType("thingy"),
		}
		_, err := invalidConf.Validate("path", resource.APITypeServiceName)
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
			Name: "foo",
			API:  resource.APINamespace("ac:me").WithServiceType("thingy"),
		}
		_, err := invalidConf.Validate("path", resource.APITypeServiceName)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "reserved character : used")
	})

	t.Run("default model to default", func(t *testing.T) {
		validConf := resource.Config{
			Name: "foo",
			API:  acmeAPINamespace.WithServiceType("thingy"),
		}
		deps, err := validConf.Validate("path", resource.APITypeServiceName)
		test.That(t, deps, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, validConf.Model.String(), test.ShouldEqual, "rdk:builtin:builtin")
	})

	t.Run("model variations", func(t *testing.T) {
		t.Run("config valid short model", func(t *testing.T) {
			shortConf := resource.Config{
				Name:  "foo",
				API:   resource.APINamespaceRDK.WithComponentType("bar"),
				Model: resource.Model{Name: "fake"},
			}
			deps, err := shortConf.Validate("path", resource.APITypeServiceName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.Model.Family, test.ShouldResemble, resource.DefaultModelFamily)
			test.That(t, shortConf.Model.Name, test.ShouldEqual, "fake")
		})

		t.Run("config valid full model", func(t *testing.T) {
			shortConf := resource.Config{
				Name:  "foo",
				API:   resource.APINamespaceRDK.WithComponentType("bar"),
				Model: fakeModel,
			}
			deps, err := shortConf.Validate("path", resource.APITypeServiceName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.Model.Family, test.ShouldResemble, resource.DefaultModelFamily)
			test.That(t, shortConf.Model.Name, test.ShouldEqual, "fake")
		})

		t.Run("config valid external model", func(t *testing.T) {
			shortConf := resource.Config{
				Name:  "foo",
				API:   resource.APINamespaceRDK.WithComponentType("bar"),
				Model: extModel,
			}
			deps, err := shortConf.Validate("path", resource.APITypeServiceName)
			test.That(t, deps, test.ShouldBeNil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, shortConf.Model.Family, test.ShouldResemble, resource.NewModelFamily("acme", "test"))
			test.That(t, shortConf.Model.Name, test.ShouldEqual, "model")
		})
	})
}

func TestServiceResourceName(t *testing.T) {
	for _, tc := range []struct {
		Name         string
		Conf         resource.Config
		ExpectedAPI  resource.API
		ExpectedName resource.Name
	}{
		{
			"all fields included",
			resource.Config{
				Name: "motion1",
				API:  motion.API,
			},
			motion.API,
			resource.NewName(motion.API, "motion1"),
		},
		{
			"all fields included with external type",
			resource.Config{
				Name:  "foo",
				API:   extServiceAPI,
				Model: extModel,
			},
			extServiceAPI,
			resource.NewName(extServiceAPI, "foo"),
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := tc.Conf.Validate("", resource.APITypeServiceName)
			test.That(t, err, test.ShouldBeNil)
			rName := tc.Conf.ResourceName()
			test.That(t, rName.API, test.ShouldResemble, tc.ExpectedAPI)
			test.That(t, rName, test.ShouldResemble, tc.ExpectedName)
		})
	}
}
