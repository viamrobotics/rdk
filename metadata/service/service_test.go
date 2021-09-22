package service_test

import (
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/core/metadata/service"
	"go.viam.com/core/resource"
)

func TestAdd(t *testing.T) {
	r, err := service.New()
	test.That(t, err, test.ShouldBeNil)
	service := r.All()[0]
	arm := resource.Name{
		UUID:      uuid.NewString(),
		Namespace: resource.ResourceNamespaceCore,
		Type:      resource.ResourceTypeComponent,
		Subtype:   "arm",
		Name:      "arm1",
	}
	sensor := resource.Name{
		UUID:      uuid.NewString(),
		Namespace: resource.ResourceNamespaceCore,
		Type:      resource.ResourceTypeComponent,
		Subtype:   "sensor",
		Name:      "sensor1",
	}

	newMetadata := resource.Name{
		UUID:      uuid.NewString(),
		Namespace: resource.ResourceNamespaceCore,
		Type:      resource.ResourceTypeService,
		Subtype:   resource.ResourceSubtypeMetadata,
	}

	for _, tc := range []struct {
		Name        string
		NewResource resource.Name
		Expected    []resource.Name
		Err         string
	}{
		{
			"invalid addition",
			resource.Name{},
			nil,
			"uuid field for resource missing or invalid",
		},
		{
			"add metadata",
			newMetadata,
			[]resource.Name{service, newMetadata},
			"",
		},
		{
			"one addition",
			arm,
			[]resource.Name{service, newMetadata, arm},
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
			[]resource.Name{service, newMetadata, arm, sensor},
			"",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			err := r.Add(tc.NewResource)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, r.All(), test.ShouldResemble, tc.Expected)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	r, err := service.New()
	test.That(t, err, test.ShouldBeNil)

	service := r.All()[0]
	arm, err := resource.New(
		resource.ResourceNamespaceCore,
		resource.ResourceTypeComponent,
		"arm",
		"arm1",
	)
	test.That(t, err, test.ShouldBeNil)
	sensor, err := resource.New(
		resource.ResourceNamespaceCore,
		resource.ResourceTypeComponent,
		"sensor",
		"sensor1",
	)
	test.That(t, err, test.ShouldBeNil)
	r.Add(arm)
	r.Add(sensor)

	for _, tc := range []struct {
		Name     string
		Remove   resource.Name
		Expected []resource.Name
		Err      string
	}{
		{
			"invalid removal",
			resource.Name{},
			nil,
			"uuid field for resource missing or invalid",
		},
		{
			"remove metadata",
			service,
			[]resource.Name{arm, sensor},
			"",
		},
		{
			"one removal",
			sensor,
			[]resource.Name{arm},
			"",
		},
		{
			"second removal",
			arm,
			[]resource.Name{},
			"",
		},
		{
			"not found",
			sensor,
			nil,
			"unable to find and remove resource",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			err := r.Remove(tc.Remove)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, r.All(), test.ShouldResemble, tc.Expected)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}
