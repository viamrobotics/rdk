package resource_test

import (
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/core/resource"
)

func TestResourceValidate(t *testing.T) {
	for _, tc := range []struct {
		Name        string
		NewResource resource.ResourceName
		Err         string
	}{
		{
			"missing uuid",
			resource.ResourceName{
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Subtype:   "arm",
				Name:      "arm1",
			},
			"uuid field for resource missing or invalid",
		},
		{
			"invalid uuid",
			resource.ResourceName{
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
			resource.ResourceName{
				UUID:    uuid.NewString(),
				Type:    resource.ResourceTypeComponent,
				Subtype: "arm",
				Name:    "arm1",
			},
			"namespace field for resource missing or invalid",
		},
		{
			"missing type",
			resource.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Subtype:   "arm",
				Name:      "arm1",
			},
			"type field for resource missing or invalid",
		},
		{
			"missing subtype",
			resource.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Name:      "arm1",
			},
			"subtype field for resource missing or invalid",
		},
		{
			"missing name",
			resource.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Subtype:   "arm",
			},
			"",
		},
		{
			"all fields included",
			resource.ResourceName{
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
