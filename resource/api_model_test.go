package resource

import (
	"testing"

	"go.viam.com/test"
)

func TestLegacyAPIParsingWithoutNamespace(t *testing.T) {
	// A legacy API configuration on a component/service does not include the `api` field. But
	// rather a `type` field and optionally a `namespace` field.
	var resConfig Config
	err := resConfig.UnmarshalJSON([]byte(`
{
  "name": "legacy",
  "type": "board",
  "model": "fake"
}`))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resConfig.Name, test.ShouldEqual, "legacy")
	test.That(t, resConfig.API.Type.Namespace, test.ShouldEqual, "")
	test.That(t, resConfig.API.Type.Name, test.ShouldEqual, "")
	test.That(t, resConfig.API.SubtypeName, test.ShouldEqual, "board")
	// Unqualified `model`s (single word rather than colon delimited triplet) automatically acquire
	// the `rdk:builtin` model family at parse time.
	test.That(t, resConfig.Model.Family.Namespace, test.ShouldEqual, "rdk")
	test.That(t, resConfig.Model.Family.Name, test.ShouldEqual, "builtin")
	test.That(t, resConfig.Model.Name, test.ShouldEqual, "fake")

	// After parsing a `resource.Config` as json, fill the API Type in with `rdk:component`.
	resConfig.AdjustPartialNames("component")
	test.That(t, resConfig.API.Type.Namespace, test.ShouldEqual, "rdk")
	test.That(t, resConfig.API.Type.Name, test.ShouldEqual, "component")
}

func TestLegacyAPIParsing(t *testing.T) {
	// A legacy API configuration on a component/service does not include the `api` field. But
	// rather a `type` field and optionally a `namespace` field.
	var resConfig Config
	err := resConfig.UnmarshalJSON([]byte(`
{
  "name": "legacy",
  "namespace": "not_rdk",
  "type": "board",
  "model": "fake"
}`))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resConfig.Name, test.ShouldEqual, "legacy")
	test.That(t, resConfig.API.Type.Namespace, test.ShouldEqual, "not_rdk")
	test.That(t, resConfig.API.Type.Name, test.ShouldEqual, "")
	test.That(t, resConfig.API.SubtypeName, test.ShouldEqual, "board")
	// Unqualified `model`s (single word rather than colon delimited triplet) automatically acquire
	// the `rdk:builtin` model family at parse time.
	test.That(t, resConfig.Model.Family.Namespace, test.ShouldEqual, "rdk")
	test.That(t, resConfig.Model.Family.Name, test.ShouldEqual, "builtin")
	test.That(t, resConfig.Model.Name, test.ShouldEqual, "fake")

	// After parsing a `resource.Config` as json, fill the API Type in with `rdk:component`.
	resConfig.AdjustPartialNames("component")
	test.That(t, resConfig.API.Type.Namespace, test.ShouldEqual, "not_rdk")
	test.That(t, resConfig.API.Type.Name, test.ShouldEqual, "component")
}

func TestServiceAdjustPartialNames(t *testing.T) {
	resConfig := Config{
		API: API{
			SubtypeName: "nav",
		},
	}

	// Running `AdjustPartialNames` on a service should fill in all of the `API` and `Model` fields,
	// with the exception of the `SubtypeName` with a default.
	resConfig.AdjustPartialNames("service")
	test.That(t, resConfig.Name, test.ShouldEqual, "builtin")
	test.That(t, resConfig.API.Type.Namespace, test.ShouldEqual, "rdk")
	test.That(t, resConfig.API.Type.Name, test.ShouldEqual, "service")
	test.That(t, resConfig.API.SubtypeName, test.ShouldEqual, "nav")
	test.That(t, resConfig.Model.Family.Namespace, test.ShouldEqual, "rdk")
	test.That(t, resConfig.Model.Family.Name, test.ShouldEqual, "builtin")
	test.That(t, resConfig.Model.Name, test.ShouldEqual, "builtin")
}

func TestAPIStringParsing(t *testing.T) {
	// Test that a colon-delimited triplet string for the `API` is parsed into its
	// namespace/type/subtype tuple.
	var resConfig Config
	err := resConfig.UnmarshalJSON([]byte(`
{
  "name": "legacy",
  "type": "board",
  "model": "fake",
  "api": "namespace:component_type:subtype"
}`))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, resConfig.API.Type.Namespace, test.ShouldEqual, "namespace")
	test.That(t, resConfig.API.Type.Name, test.ShouldEqual, "component_type")
	test.That(t, resConfig.API.SubtypeName, test.ShouldEqual, "subtype")

	// Test that a malformed string gives a string specific error message.
	err = resConfig.UnmarshalJSON([]byte(`
{
  "name": "legacy",
  "type": "board",
  "model": "fake",
  "api": "double::colons::are_bad"
}`))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not a valid API config string.")
}

func TestAPIObjectParsing(t *testing.T) {
	// Test that an API object with all three types parses correctly.
	var resConfig Config
	err := resConfig.UnmarshalJSON([]byte(`
{
  "name": "legacy",
  "type": "board",
  "model": "fake",
  "api": {
    "namespace": "namespace",
    "type": "component",
    "subtype": "subtype"
  }
}`))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, resConfig.API.Type.Namespace, test.ShouldEqual, "namespace")
	test.That(t, resConfig.API.Type.Name, test.ShouldEqual, "component")
	test.That(t, resConfig.API.SubtypeName, test.ShouldEqual, "subtype")

	// Test that a malformed API object does not parse. Without enumerating all of the ways an API
	// object can be malformed.
	err = resConfig.UnmarshalJSON([]byte(`
{
  "name": "legacy",
  "type": "board",
  "model": "fake",
  "api": {}
}`))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "namespace field for resource missing or invalid")
}
