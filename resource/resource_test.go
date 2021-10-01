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
		t.Run(tc.TestName, func(t *testing.T) {
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

func TestResourceNameNewFromString(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
		Err      string
	}{
		{
			"malformed name",
			"core/component/arm/arm1",
			resource.Name{},
			"there is more than one backslash",
		},
		{
			"too many colons",
			"core::component::arm/arm1",
			resource.Name{},
			"there are more than 2 colons",
		},
		{
			"too few colons",
			"core.component.arm/arm1",
			resource.Name{},
			"there are less than 2 colons",
		},
		{
			"missing name",
			"core:component:arm",
			resource.Name{
				UUID:      "8ad23fcd-7f30-56b9-a7f4-cf37a980b4dd",
				Namespace: "core",
				Type:      "component",
				Subtype:   "arm",
				Name:      "",
			},
			"",
		},
		{
			"all fields included",
			"core:component:arm/arm1",
			resource.Name{
				UUID:      "1ef3fc81-df1d-5ac4-b11d-bc1513e47f06",
				Namespace: "core",
				Type:      "component",
				Subtype:   "arm",
				Name:      "arm1",
			},
			"",
		},
		{
			"all fields included 2",
			"core:component:compass/compass1",
			resource.Name{
				UUID:      "d6358b56-3b43-5626-ab8c-b16e7233a832",
				Namespace: "core",
				Type:      "component",
				Subtype:   "compass",
				Name:      "compass1",
			},
			"",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed, err := resource.NewFromString(tc.Name)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, observed, test.ShouldResemble, tc.Expected)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}

func TestResourceNameStrings(t *testing.T) {
	for _, tc := range []struct {
		TestName         string
		Name             resource.Name
		ExpectedSubtype  string
		ExpectedFullName string
	}{
		{
			"all fields included",
			resource.Name{
				Namespace: "core",
				Type:      "component",
				Subtype:   "arm",
				Name:      "arm1",
			},
			"core:component:arm",
			"core:component:arm/arm1",
		},
		{
			"missing subtype",
			resource.Name{
				Namespace: "core",
				Type:      "component",
				Name:      "arm1",
			},
			"core:component:",
			"core:component:/arm1",
		},
		{
			"missing name",
			resource.Name{
				Namespace: "core",
				Type:      "component",
				Subtype:   "arm",
			},
			"core:component:arm",
			"core:component:arm",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			test.That(t, tc.Name.ResourceSubtype(), test.ShouldEqual, tc.ExpectedSubtype)
			test.That(t, tc.Name.FullyQualifiedName(), test.ShouldEqual, tc.ExpectedFullName)
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
