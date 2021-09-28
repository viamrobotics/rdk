package resource_test

import (
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/core/resource"
)

func TestResourceNameNew(t *testing.T) {
	for _, tc := range []struct {
		TestName  string
		Namespace string
		Type      string
		Subtype   string
		Name      string
		Err       string
	}{
		{
			"missing namespace",
			"",
			resource.ResourceTypeComponent,
			"arm",
			"arm1",
			"namespace parameter missing or invalid",
		},
		{
			"missing type",
			resource.ResourceNamespaceCore,
			"",
			"arm",
			"arm1",
			"type parameter missing or invalid",
		},
		{
			"missing subtype",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			"",
			"arm1",
			"subtype parameter missing or invalid",
		},
		{
			"missing name",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			"arm",
			"",
			"",
		},
		{
			"all fields included",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			"arm",
			"arm1",
			"",
		},
		{
			"all fields included 2",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeService,
			"metadata",
			"",
			"",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			observed, err := resource.New(tc.Namespace, tc.Type, tc.Subtype, tc.Name)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				expected, _ := resource.New(tc.Namespace, tc.Type, tc.Subtype, tc.Name)
				test.That(t, expected, test.ShouldResemble, observed)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}

func TestResourceNameValidate(t *testing.T) {
	for _, tc := range []struct {
		Name        string
		NewResource resource.Name
		Err         string
	}{
		{
			"missing uuid",
			resource.Name{
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Subtype:   "arm",
				Name:      "arm1",
			},
			"uuid field for resource missing or invalid",
		},
		{
			"invalid uuid",
			resource.Name{
				UUID:      "abcd",
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Subtype:   "arm",
				Name:      "arm1",
			},
			"uuid field for resource missing or invalid",
		},
		{
			"missing namespace",
			resource.Name{
				UUID:    uuid.NewString(),
				Type:    resource.ResourceTypeComponent,
				Subtype: "arm",
				Name:    "arm1",
			},
			"namespace field for resource missing or invalid",
		},
		{
			"missing type",
			resource.Name{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Subtype:   "arm",
				Name:      "arm1",
			},
			"type field for resource missing or invalid",
		},
		{
			"missing subtype",
			resource.Name{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Name:      "arm1",
			},
			"subtype field for resource missing or invalid",
		},
		{
			"missing name",
			resource.Name{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Subtype:   "arm",
			},
			"",
		},
		{
			"all fields included",
			resource.Name{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Subtype:   "arm",
				Name:      "arm1",
			},
			"",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			err := tc.NewResource.Validate()
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}
