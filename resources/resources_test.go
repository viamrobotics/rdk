package resources_test

import (
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/core/resources"
)

func TestValidateResourceName(t *testing.T) {}

func TestAddResource(t *testing.T) {
	r := resources.New()

	metadata_r := r.GetResources()[0]
	arm_r := resources.Resource{
		Uuid:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "arm",
		Name:      "arm1",
	}
	sensor_r := resources.Resource{
		Uuid:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "sensor",
		Name:      "sensor1",
	}

	for _, tc := range []struct {
		Name        string
		NewResource resources.Resource
		Expected    []resources.Resource
		Err         string
	}{
		{
			"invalid addition",
			resources.Resource{
				Uuid:      "abcd",
				Namespace: resources.ResourceNamespaceCore,
				Type:      resources.ResourceTypeComponent,
				Subtype:   "arm",
				Name:      "arm1",
			},
			nil,
			"uuid field for resource missing or invalid",
		},
		{
			"invalid addition 2",
			resources.Resource{
				Uuid:    uuid.NewString(),
				Type:    resources.ResourceTypeComponent,
				Subtype: "arm",
				Name:    "arm1",
			},
			nil,
			"namespace field for resource missing or invalid",
		},
		{
			"one addition",
			arm_r,
			[]resources.Resource{metadata_r, arm_r},
			"",
		},
		{
			"duplicate addition",
			arm_r,
			nil,
			"already exists",
		},
		{
			"another addition",
			sensor_r,
			[]resources.Resource{metadata_r, arm_r, sensor_r},
			"",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			err := r.AddResource(tc.NewResource)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, r.GetResources(), test.ShouldResemble, tc.Expected)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}
