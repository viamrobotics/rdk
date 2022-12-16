package resource_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/resource"
)

func TestModel(t *testing.T) {
	for _, tc := range []struct {
		TestName  string
		Namespace resource.Namespace
		Family    resource.ModelFamilyName
		Model     resource.ModelName
		Expected  resource.Model
		Err       string
	}{
		{
			"missing namespace",
			"",
			"test",
			"modelA",
			resource.Model{
				ModelFamily: resource.ModelFamily{Namespace: "", Family: "test"},
				Name:        "modelA",
			},
			"namespace field for model missing",
		},
		{
			"missing family",
			"acme",
			"",
			"modelA",
			resource.Model{
				ModelFamily: resource.ModelFamily{Namespace: "acme", Family: ""},
				Name:        "modelA",
			},
			"model_family field for model missing",
		},
		{
			"missing name",
			"acme",
			"test",
			"",
			resource.Model{
				ModelFamily: resource.ModelFamily{Namespace: "acme", Family: "test"},
				Name:        "",
			},
			"name field for model missing",
		},
		{
			"reserved character in model namespace",
			"ac:me",
			"test",
			"modelA",
			resource.Model{
				ModelFamily: resource.ModelFamily{Namespace: "ac:me", Family: "test"},
				Name:        "modelA",
			},
			"reserved character : used",
		},
		{
			"reserved character in model family",
			"acme",
			"te:st",
			"modelA",
			resource.Model{
				ModelFamily: resource.ModelFamily{Namespace: "acme", Family: "te:st"},
				Name:        "modelA",
			},
			"reserved character : used",
		},
		{
			"reserved character in model name",
			"acme",
			"test",
			"model:A",
			resource.Model{
				ModelFamily: resource.ModelFamily{Namespace: "acme", Family: "test"},
				Name:        "model:A",
			},
			"reserved character : used",
		},
		{
			"valid model",
			"acme",
			"test",
			"modelA",
			resource.Model{
				ModelFamily: resource.ModelFamily{Namespace: "acme", Family: "test"},
				Name:        "modelA",
			},
			"",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := resource.NewModel(tc.Namespace, tc.Family, tc.Model)
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
		Expected resource.Model
		Err      string
		ErrJSON  string
	}{
		{
			"valid",
			"acme:test:modelA",
			resource.Model{
				ModelFamily: resource.ModelFamily{Namespace: "acme", Family: "test"},
				Name:        "modelA",
			},
			"",
			"",
		},
		{
			"valid with special characters and numbers",
			"acme_corp1:test-collection99:model_a2",
			resource.Model{
				ModelFamily: resource.ModelFamily{Namespace: "acme_corp1", Family: "test-collection99"},
				Name:        "model_a2",
			},
			"",
			"",
		},
		{
			"invalid with slash",
			"acme/corp:test:modelA",
			resource.Model{},
			"not a valid model name",
			"invalid character",
		},
		{
			"invalid with caret",
			"acme:test:model^A",
			resource.Model{},
			"not a valid model name",
			"invalid character",
		},
		{
			"missing field",
			"acme:test",
			resource.Model{},
			"not a valid model name",
			"invalid character",
		},
		{
			"empty namespace",
			":test:modelA",
			resource.Model{},
			"not a valid model name",
			"invalid character",
		},
		{
			"empty family",
			"acme::modelA",
			resource.Model{},
			"not a valid model name",
			"invalid character",
		},
		{
			"empty name",
			"acme:test::",
			resource.Model{},
			"not a valid model name",
			"invalid character",
		},
		{
			"extra field",
			"acme:test:modelA:fail",
			resource.Model{},
			"not a valid model name",
			"invalid character",
		},
		{
			"mistaken resource name",
			"acme:test:modelA/fail",
			resource.Model{},
			"not a valid model name",
			"invalid character",
		},
		{
			"short form",
			"modelB",
			resource.Model{
				ModelFamily: resource.DefaultModelFamily,
				Name:        "modelB",
			},
			"",
			"",
		},
		{
			"invalid short form",
			"model^B",
			resource.Model{},
			"not a valid model name",
			"invalid character",
		},
		{
			"valid nested json",
			`{"namespace": "acme", "model_family": "test", "name": "modelB"}`,
			resource.Model{
				ModelFamily: resource.ModelFamily{Namespace: "acme", Family: "test"},
				Name:        "modelB",
			},
			"not a valid model name",
			"",
		},
		{
			"invalid nested json family",
			`{"namespace": "acme", "model_family": "te^st", "name": "modelB"}`,
			resource.Model{},
			"not a valid model name",
			"not a valid model family",
		},
		{
			"invalid nested json namespace",
			`{"namespace": "$acme", "model_family": "test", "name": "modelB"}`,
			resource.Model{},
			"not a valid model name",
			"not a valid model namespace",
		},
		{
			"invalid nested json name",
			`{"namespace": "acme", "model_family": "test", "name": "model#B"}`,
			resource.Model{},
			"not a valid model name",
			"not a valid model name",
		},
		{
			"missing nested json field",
			`{"namespace": "acme", "name": "model#B"}`,
			resource.Model{},
			"not a valid model name",
			"field for model missing",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed, err := resource.NewModelFromString(tc.StrModel)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, observed.Validate(), test.ShouldBeNil)
				test.That(t, observed, test.ShouldResemble, tc.Expected)
				test.That(t, observed.String(), test.ShouldResemble, tc.Expected.String())
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}

			fromJSON := &resource.Model{}
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
