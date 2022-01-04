package config_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/testutils"
)

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
				Type: "frame_system",
			},
			framesystem.Subtype,
			resource.NameFromSubtype(framesystem.Subtype, ""),
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			rName := tc.Config.ResourceName()
			test.That(t, rName.Subtype, test.ShouldResemble, tc.ExpectedSubtype)
			test.That(t, rName, test.ShouldResemble, tc.ExpectedName)
		})
	}
}

func TestParseServiceFlag(t *testing.T) {
	_, err := config.ParseServiceFlag("foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

	comp, err := config.ParseServiceFlag("type=foo,model=bar,name=baz,attr=wee:woo,subtype=who,depends_on=foo|bar,attr=one:two")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, comp.Type, test.ShouldEqual, config.ServiceType("foo"))
	test.That(t, comp.Attributes, test.ShouldResemble, config.AttributeMap{
		"wee": "woo",
		"one": "two",
	})
}
