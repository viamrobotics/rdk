package resource_test

import (
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/core/component/arm"
	"go.viam.com/core/resource"
)

func TestResourceType(t *testing.T) {
	for _, tc := range []struct {
		TestName  string
		Namespace resource.Namespace
		Type      resource.TypeName
		Expected  resource.Type
		Err       string
	}{
		{
			"missing namespace",
			"",
			resource.ResourceTypeComponent,
			resource.Type{ResourceType: resource.ResourceTypeComponent},
			"namespace field for resource missing or invalid",
		},
		{
			"missing type",
			resource.ResourceNamespaceCore,
			"",
			resource.Type{Namespace: resource.ResourceNamespaceCore},
			"type field for resource missing or invalid",
		},
		{
			"all fields included",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
			"",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := resource.NewType(tc.Namespace, tc.Type)
			test.That(t, tc.Expected, test.ShouldResemble, observed)
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

func TestResourceSubtype(t *testing.T) {
	for _, tc := range []struct {
		TestName  string
		Namespace resource.Namespace
		Type      resource.TypeName
		Subtype   resource.SubtypeName
		Expected  resource.Subtype
		Err       string
	}{
		{
			"missing namespace",
			"",
			resource.ResourceTypeComponent,
			arm.SubtypeName,
			resource.Subtype{
				Type: resource.Type{
					ResourceType: resource.ResourceTypeComponent,
				},
				ResourceSubtype: arm.SubtypeName,
			},
			"namespace field for resource missing or invalid",
		},
		{
			"missing type",
			resource.ResourceNamespaceCore,
			"",
			arm.SubtypeName,
			resource.Subtype{
				Type: resource.Type{
					Namespace: resource.ResourceNamespaceCore,
				},
				ResourceSubtype: arm.SubtypeName,
			},
			"type field for resource missing or invalid",
		},
		{
			"missing subtype",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			"",
			resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceCore,
					ResourceType: resource.ResourceTypeComponent,
				},
			},
			"subtype field for resource missing or invalid",
		},
		{
			"all fields included",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			arm.SubtypeName,
			resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceCore,
					ResourceType: resource.ResourceTypeComponent,
				},
				ResourceSubtype: arm.SubtypeName,
			},
			"",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := resource.NewSubtype(tc.Namespace, tc.Type, tc.Subtype)
			test.That(t, tc.Expected, test.ShouldResemble, observed)
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

func TestResourceNameNew(t *testing.T) {
	for _, tc := range []struct {
		TestName  string
		Namespace resource.Namespace
		Type      resource.TypeName
		Subtype   resource.SubtypeName
		Name      string
		Expected  resource.Name
	}{
		{
			"missing name",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			arm.SubtypeName,
			"",
			resource.Name{
				UUID: "8ad23fcd-7f30-56b9-a7f4-cf37a980b4dd",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: arm.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			arm.SubtypeName,
			"arm1",
			resource.Name{
				UUID: "1ef3fc81-df1d-5ac4-b11d-bc1513e47f06",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: arm.SubtypeName,
				},
				Name: "arm1",
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := resource.NewName(tc.Namespace, tc.Type, tc.Subtype, tc.Name)
			test.That(t, tc.Expected, test.ShouldResemble, observed)
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
				UUID: "8ad23fcd-7f30-56b9-a7f4-cf37a980b4dd",
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceCore,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: arm.SubtypeName,
				},
				Name: "",
			},
			"",
		},
		{
			"all fields included",
			arm.Named("arm1").String(),
			resource.Name{
				UUID: "1ef3fc81-df1d-5ac4-b11d-bc1513e47f06",
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceCore,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: arm.SubtypeName,
				},
				Name: "arm1",
			},
			"",
		},
		{
			"all fields included 2",
			"core:component:compass/compass1",
			resource.Name{
				UUID: "d6358b56-3b43-5626-ab8c-b16e7233a832",
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceCore,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: "compass",
				},
				Name: "compass1",
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
		ExpectedFullName string
	}{
		{
			"all fields included",
			resource.Name{
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceCore,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: arm.SubtypeName,
				},
				Name: "arm1",
			},
			arm.Named("arm1").String(),
		},
		{
			"missing subtype",
			resource.Name{
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceCore,
						ResourceType: resource.ResourceTypeComponent,
					},
				},
				Name: "arm1",
			},
			"core:component:/arm1",
		},
		{
			"missing name",
			resource.Name{
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceCore,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: arm.SubtypeName,
				},
			},
			"core:component:arm",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			test.That(t, tc.Name.String(), test.ShouldEqual, tc.ExpectedFullName)
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
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceCore,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: arm.SubtypeName,
				},
				Name: "arm1",
			},
			"uuid field for resource missing or invalid",
		},
		{
			"invalid uuid",
			resource.Name{
				UUID: "abcd",
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceCore,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: arm.SubtypeName,
				},
				Name: "arm1",
			},
			"uuid field for resource missing or invalid",
		},
		{
			"missing namespace",
			resource.Name{
				UUID: uuid.NewString(),
				Subtype: resource.Subtype{
					Type: resource.Type{
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: arm.SubtypeName,
				},
				Name: "arm1",
			},
			"namespace field for resource missing or invalid",
		},
		{
			"missing type",
			resource.Name{
				UUID: uuid.NewString(),
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace: resource.ResourceNamespaceCore,
					},
					ResourceSubtype: arm.SubtypeName,
				},
				Name: "arm1",
			},
			"type field for resource missing or invalid",
		},
		{
			"missing subtype",
			resource.Name{
				UUID: uuid.NewString(),
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceCore,
						ResourceType: resource.ResourceTypeComponent,
					},
				},
				Name: "arm1",
			},
			"subtype field for resource missing or invalid",
		},
		{
			"missing name",
			resource.Name{
				UUID: uuid.NewString(),
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceCore,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: arm.SubtypeName,
				},
			},
			"",
		},
		{
			"all fields included",
			resource.Name{
				UUID: uuid.NewString(),
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceCore,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: arm.SubtypeName,
				},
				Name: "arm1",
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
