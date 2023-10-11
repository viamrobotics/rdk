package resource

import (
	"testing"

	"go.viam.com/test"
)

func TestLegacyAPIParsingWithoutNamespace(t *testing.T) {
	//nolint:dupl

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
	//nolint:dupl

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

	// Running `AdjustPartialNames` on a service should fill in all of the remaining `API` and
	// `Model` fields.
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
	//nolint:dupl

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
	//nolint:dupl

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

func TestModel(t *testing.T) {
	//nolint:dupl
	for _, tc := range []struct {
		TestName  string
		Namespace ModelNamespace
		Family    string
		Model     string
		Expected  Model
		Err       string
	}{
		{
			"missing namespace",
			"",
			"test",
			"modelA",
			Model{
				Family: ModelFamily{Namespace: "", Name: "test"},
				Name:   "modelA",
			},
			"namespace field for model missing",
		},
		{
			"missing family",
			"acme",
			"",
			"modelA",
			Model{
				Family: ModelFamily{Namespace: "acme", Name: ""},
				Name:   "modelA",
			},
			"model_family field for model missing",
		},
		{
			"missing name",
			"acme",
			"test",
			"",
			Model{
				Family: ModelFamily{Namespace: "acme", Name: "test"},
				Name:   "",
			},
			"name field for model missing",
		},
		{
			"reserved character in model namespace",
			"ac:me",
			"test",
			"modelA",
			Model{
				Family: ModelFamily{Namespace: "ac:me", Name: "test"},
				Name:   "modelA",
			},
			"reserved character : used",
		},
		{
			"reserved character in model family",
			"acme",
			"te:st",
			"modelA",
			Model{
				Family: ModelFamily{Namespace: "acme", Name: "te:st"},
				Name:   "modelA",
			},
			"reserved character : used",
		},
		{
			"reserved character in model name",
			"acme",
			"test",
			"model:A",
			Model{
				Family: ModelFamily{Namespace: "acme", Name: "test"},
				Name:   "model:A",
			},
			"reserved character : used",
		},
		{
			"valid model",
			"acme",
			"test",
			"modelA",
			Model{
				Family: ModelFamily{Namespace: "acme", Name: "test"},
				Name:   "modelA",
			},
			"",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := tc.Namespace.WithFamily(tc.Family).WithModel(tc.Model)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
			err := observed.Validate()
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}

func TestModelFromString(t *testing.T) {
	//nolint:dupl
	for _, tc := range []struct {
		TestName string
		StrModel string
		Expected Model
		Err      string
	}{
		{
			"valid",
			"acme:test:modelA",
			Model{
				Family: ModelFamily{Namespace: "acme", Name: "test"},
				Name:   "modelA",
			},
			"",
		},
		{
			"valid with special characters and numbers",
			"acme_corp1:test-collection99:model_a2",
			Model{
				Family: ModelFamily{Namespace: "acme_corp1", Name: "test-collection99"},
				Name:   "model_a2",
			},
			"",
		},
		{
			"invalid with slash",
			"acme/corp:test:modelA",
			Model{},
			"not a valid model name",
		},
		{
			"invalid with caret",
			"acme:test:model^A",
			Model{},
			"not a valid model name",
		},
		{
			"missing field",
			"acme:test",
			Model{},
			"not a valid model name",
		},
		{
			"empty namespace",
			":test:modelA",
			Model{},
			"not a valid model name",
		},
		{
			"empty family",
			"acme::modelA",
			Model{},
			"not a valid model name",
		},
		{
			"empty name",
			"acme:test::",
			Model{},
			"not a valid model name",
		},
		{
			"extra field",
			"acme:test:modelA:fail",
			Model{},
			"not a valid model name",
		},
		{
			"mistaken resource name",
			"acme:test:modelA/fail",
			Model{},
			"not a valid model name",
		},
		{
			"short form",
			"modelB",
			Model{
				Family: DefaultModelFamily,
				Name:   "modelB",
			},
			"",
		},
		{
			"invalid short form",
			"model^B",
			Model{},
			"not a valid model name",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed, err := NewModelFromString(tc.StrModel)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, observed.Validate(), test.ShouldBeNil)
				test.That(t, observed, test.ShouldResemble, tc.Expected)
				test.That(t, observed.String(), test.ShouldResemble, tc.Expected.String())
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}

func TestModelFromJSONObject(t *testing.T) {
	//nolint:dupl
	for _, tc := range []struct {
		TestName string
		StrModel string
		Expected Model
		ErrJSON  string
	}{
		{
			"valid nested json",
			`{"namespace": "acme", "model_family": "test", "name": "modelB"}`,
			Model{
				Family: ModelFamily{Namespace: "acme", Name: "test"},
				Name:   "modelB",
			},
			"",
		},
		{
			"invalid nested json family",
			`{"namespace": "acme", "model_family": "te^st", "name": "modelB"}`,
			Model{},
			"not a valid model family",
		},
		{
			"invalid nested json namespace",
			`{"namespace": "$acme", "model_family": "test", "name": "modelB"}`,
			Model{},
			"not a valid model namespace",
		},
		{
			"invalid nested json name",
			`{"namespace": "acme", "model_family": "test", "name": "model#B"}`,
			Model{},
			"not a valid model name",
		},
		{
			"missing nested json field",
			`{"namespace": "acme", "name": "model#B"}`,
			Model{},
			"field for model missing",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			fromJSON := &Model{}
			errJSON := fromJSON.UnmarshalJSON([]byte(tc.StrModel))
			if tc.ErrJSON == "" {
				test.That(t, errJSON, test.ShouldBeNil)
				test.That(t, fromJSON.Validate(), test.ShouldBeNil)
				test.That(t, fromJSON, test.ShouldResemble, &tc.Expected)
				test.That(t, fromJSON.String(), test.ShouldResemble, tc.Expected.String())
			} else {
				test.That(t, errJSON, test.ShouldNotBeNil)
				test.That(t, errJSON.Error(), test.ShouldContainSubstring, tc.ErrJSON)
			}
		})
	}
}

func TestAPIFromString(t *testing.T) {
	//nolint:dupl
	for _, tc := range []struct {
		TestName string
		StrAPI   string
		Expected API
		Err      string
	}{
		{
			"valid",
			"rdk:component:arm",
			APINamespaceRDK.WithComponentType("arm"),
			"",
		},
		{
			"valid with special characters and numbers",
			"acme_corp1:test-collection99:api_a2",
			API{
				Type:        APIType{Namespace: "acme_corp1", Name: "test-collection99"},
				SubtypeName: "api_a2",
			},
			"",
		},
		{
			"invalid with slash",
			"acme/corp:test:subtypeA",
			API{},
			"not a valid api name",
		},
		{
			"invalid with caret",
			"acme:test:subtype^A",
			API{},
			"not a valid api name",
		},
		{
			"missing field",
			"acme:test",
			API{},
			"not a valid api name",
		},
		{
			"empty namespace",
			":test:subtypeA",
			API{},
			"not a valid api name",
		},
		{
			"empty family",
			"acme::subtypeA",
			API{},
			"not a valid api name",
		},
		{
			"empty name",
			"acme:test::",
			API{},
			"not a valid api name",
		},
		{
			"extra field",
			"acme:test:subtypeA:fail",
			API{},
			"not a valid api name",
		},
		{
			"mistaken resource name",
			"acme:test:subtypeA/fail",
			API{},
			"not a valid api name",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed, err := NewAPIFromString(tc.StrAPI)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, observed.Validate(), test.ShouldBeNil)
				test.That(t, observed, test.ShouldResemble, tc.Expected)
				test.That(t, observed.String(), test.ShouldResemble, tc.Expected.String())
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}

func TestAPIFromJSONObject(t *testing.T) {
	//nolint:dupl
	for _, tc := range []struct {
		TestName string
		StrAPI   string
		Expected API
		ErrJSON  string
	}{
		{
			"valid nested json",
			`{"namespace": "acme", "type": "test", "subtype": "subtypeB"}`,
			API{
				Type:        APIType{Namespace: "acme", Name: "test"},
				SubtypeName: "subtypeB",
			},
			"",
		},
		{
			"invalid nested json type",
			`{"namespace": "acme", "type": "te^st", "subtype": "subtypeB"}`,
			API{},
			"not a valid type name",
		},
		{
			"invalid nested json namespace",
			`{"namespace": "$acme", "type": "test", "subtype": "subtypeB"}`,
			API{},
			"not a valid type namespace",
		},
		{
			"invalid nested json subtype",
			`{"namespace": "acme", "type": "test", "subtype": "subtype#B"}`,
			API{},
			"not a valid subtype name",
		},
		{
			"missing nested json field",
			`{"namespace": "acme", "name": "subtype#B"}`,
			API{},
			"field for resource missing",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			fromJSON := &API{}
			errJSON := fromJSON.UnmarshalJSON([]byte(tc.StrAPI))
			if tc.ErrJSON == "" {
				test.That(t, errJSON, test.ShouldBeNil)
				test.That(t, fromJSON.Validate(), test.ShouldBeNil)
				test.That(t, fromJSON, test.ShouldResemble, &tc.Expected)
				test.That(t, fromJSON.String(), test.ShouldResemble, tc.Expected.String())
			} else {
				test.That(t, errJSON, test.ShouldNotBeNil)
				test.That(t, errJSON.Error(), test.ShouldContainSubstring, tc.ErrJSON)
			}
		})
	}
}
