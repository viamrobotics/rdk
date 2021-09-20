package resources_test

import (
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/core/resources"
)

func TestResourceValidate(t *testing.T) {
	for _, tc := range []struct {
		Name        string
		NewResource resources.Resource
		Err         string
	}{
		{
			"missing uuid",
			resources.Resource{
				Namespace: resources.ResourceNamespaceCore,
				Type:      resources.ResourceTypeComponent,
				Subtype:   "arm",
				Name:      "arm1",
			},
			"uuid field for resource missing or invalid",
		},
		{
			"invalid uuid",
			resources.Resource{
				UUID:      "abcd",
				Namespace: resources.ResourceNamespaceCore,
				Type:      resources.ResourceTypeComponent,
				Subtype:   "arm",
				Name:      "arm1",
			},
			"uuid field for resource missing or invalid",
		},
		{
			"missing namespace",
			resources.Resource{
				UUID:    uuid.NewString(),
				Type:    resources.ResourceTypeComponent,
				Subtype: "arm",
				Name:    "arm1",
			},
			"namespace field for resource missing or invalid",
		},
		{
			"missing type",
			resources.Resource{
				UUID:      uuid.NewString(),
				Namespace: resources.ResourceNamespaceCore,
				Subtype:   "arm",
				Name:      "arm1",
			},
			"type field for resource missing or invalid",
		},
		{
			"missing subtype",
			resources.Resource{
				UUID:      uuid.NewString(),
				Namespace: resources.ResourceNamespaceCore,
				Type:      resources.ResourceTypeComponent,
				Name:      "arm1",
			},
			"subtype field for resource missing or invalid",
		},
		{
			"missing name",
			resources.Resource{
				UUID:      uuid.NewString(),
				Namespace: resources.ResourceNamespaceCore,
				Type:      resources.ResourceTypeComponent,
				Subtype:   "arm",
			},
			"",
		},
		{
			"all fields included",
			resources.Resource{
				UUID:      uuid.NewString(),
				Namespace: resources.ResourceNamespaceCore,
				Type:      resources.ResourceTypeComponent,
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

func TestAddResource(t *testing.T) {
	r := resources.New()

	metadata := r.GetResources()[0]
	arm := resources.Resource{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "arm",
		Name:      "arm1",
	}
	sensor := resources.Resource{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "sensor",
		Name:      "sensor1",
	}

	newMetadata := resources.Resource{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeService,
		Subtype:   resources.ResourceSubtypeMetadata,
	}

	for _, tc := range []struct {
		Name        string
		NewResource resources.Resource
		Expected    []resources.Resource
		Err         string
	}{
		{
			"invalid addition",
			resources.Resource{},
			nil,
			"uuid field for resource missing or invalid",
		},
		{
			"invalid addition 2",
			newMetadata,
			nil,
			"Unable to add a resource with a metadata subtype",
		},
		{
			"one addition",
			arm,
			[]resources.Resource{metadata, arm},
			"",
		},
		{
			"duplicate addition",
			arm,
			nil,
			"already exists",
		},
		{
			"another addition",
			sensor,
			[]resources.Resource{metadata, arm, sensor},
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

func TestRemoveResource(t *testing.T) {
	r := resources.New()

	metadata := r.GetResources()[0]
	arm := resources.Resource{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "arm",
		Name:      "arm1",
	}
	sensor := resources.Resource{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "sensor",
		Name:      "sensor1",
	}

	r.AddResource(arm)
	r.AddResource(sensor)

	for _, tc := range []struct {
		Name           string
		RemoveResource resources.Resource
		Expected       []resources.Resource
		Err            string
	}{
		{
			"invalid removal",
			resources.Resource{},
			nil,
			"uuid field for resource missing or invalid",
		},
		{
			"invalid metadata removal",
			metadata,
			nil,
			"Unable to remove resource with a metadata subtype",
		},
		{
			"one removal",
			sensor,
			[]resources.Resource{metadata, arm},
			"",
		},
		{
			"second removal",
			arm,
			[]resources.Resource{metadata},
			"",
		},
		{
			"not found",
			sensor,
			nil,
			"Unable to find and remove resource",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			err := r.RemoveResource(tc.RemoveResource)
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

func TestClearResource(t *testing.T) {
	r := resources.New()

	metadata := r.GetResources()[0]
	arm := resources.Resource{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "arm",
		Name:      "arm1",
	}
	sensor := resources.Resource{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "sensor",
		Name:      "sensor1",
	}

	r.AddResource(arm)
	r.AddResource(sensor)

	err := r.ClearResources()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.GetResources(), test.ShouldResemble, []resources.Resource{metadata})
}
