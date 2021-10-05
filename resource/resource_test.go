package resource_test

import (
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/core/component/arm"
	"go.viam.com/core/resource"
)

func TestResourceTypeNew(t *testing.T) {
	for _, tc := range []struct {
		TestName  string
		Namespace string
		Type      string
		Str       string
		Err       string
	}{
		{
			"missing namespace",
			"",
			resource.ResourceTypeComponent,
			"",
			"namespace parameter missing or invalid",
		},
		{
			"missing type",
			resource.ResourceNamespaceCore,
			"",
			"",
			"type parameter missing or invalid",
		},
		{
			"all fields included",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			"core:component",
			"",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed, err := resource.NewType(tc.Namespace, tc.Type)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, tc.Str, test.ShouldEqual, observed.String())
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}

func TestResourceSubtypeNew(t *testing.T) {
	for _, tc := range []struct {
		TestName  string
		Namespace string
		Type      string
		Subtype   string
		Str       string
		Err       string
	}{
		{
			"missing namespace",
			"",
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeArm,
			"",
			"namespace parameter missing or invalid",
		},
		{
			"missing type",
			resource.ResourceNamespaceCore,
			"",
			resource.ResourceSubtypeArm,
			"",
			"type parameter missing or invalid",
		},
		{
			"missing subtype",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			"",
			"",
			"subtype parameter missing or invalid",
		},
		{
			"all fields included",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeArm,
			"core:component:arm",
			"",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed, err := resource.NewSubtype(tc.Namespace, tc.Type, tc.Subtype)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, tc.Str, test.ShouldEqual, observed.String())
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
			resource.ResourceSubtypeArm,
			"arm1",
			"namespace parameter missing or invalid",
		},
		{
			"missing type",
			resource.ResourceNamespaceCore,
			"",
			resource.ResourceSubtypeArm,
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
			resource.ResourceSubtypeArm,
			"",
			"",
		},
		{
			"all fields included",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeArm,
			"arm1",
			"",
		},
		{
			"all fields included 2",
			resource.ResourceNamespaceCore,
			resource.ResourceTypeService,
			resource.ResourceSubtypeMetadata,
			"",
			"",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed, err := resource.NewName(tc.Namespace, tc.Type, tc.Subtype, tc.Name)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				expected, _ := resource.NewName(tc.Namespace, tc.Type, tc.Subtype, tc.Name)
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
				UUID: "8ad23fcd-7f30-56b9-a7f4-cf37a980b4dd",
				ResourceSubtype: resource.Subtype{
					ResourceType: resource.Type{
						Namespace: resource.ResourceNamespaceCore,
						Type:      resource.ResourceTypeComponent,
					},
					Subtype: resource.ResourceSubtypeArm,
				},
				Name: "",
			},
			"",
		},
		{
			"all fields included",
			arm.Named("arm1"),
			resource.Name{
				UUID: "1ef3fc81-df1d-5ac4-b11d-bc1513e47f06",
				ResourceSubtype: resource.Subtype{
					ResourceType: resource.Type{
						Namespace: resource.ResourceNamespaceCore,
						Type:      resource.ResourceTypeComponent,
					},
					Subtype: resource.ResourceSubtypeArm,
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
				ResourceSubtype: resource.Subtype{
					ResourceType: resource.Type{
						Namespace: resource.ResourceNamespaceCore,
						Type:      resource.ResourceTypeComponent,
					},
					Subtype: "compass",
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
				ResourceSubtype: resource.Subtype{
					ResourceType: resource.Type{
						Namespace: resource.ResourceNamespaceCore,
						Type:      resource.ResourceTypeComponent,
					},
					Subtype: resource.ResourceSubtypeArm,
				},
				Name: "arm1",
			},
			arm.Named("arm1"),
		},
		{
			"missing subtype",
			resource.Name{
				ResourceSubtype: resource.Subtype{
					ResourceType: resource.Type{
						Namespace: resource.ResourceNamespaceCore,
						Type:      resource.ResourceTypeComponent,
					},
				},
				Name: "arm1",
			},
			"core:component:/arm1",
		},
		{
			"missing name",
			resource.Name{
				ResourceSubtype: resource.Subtype{
					ResourceType: resource.Type{
						Namespace: resource.ResourceNamespaceCore,
						Type:      resource.ResourceTypeComponent,
					},
					Subtype: resource.ResourceSubtypeArm,
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
				ResourceSubtype: resource.Subtype{
					ResourceType: resource.Type{
						Namespace: resource.ResourceNamespaceCore,
						Type:      resource.ResourceTypeComponent,
					},
					Subtype: resource.ResourceSubtypeArm,
				},
				Name: "arm1",
			},
			"uuid field for resource missing or invalid",
		},
		{
			"invalid uuid",
			resource.Name{
				UUID: "abcd",
				ResourceSubtype: resource.Subtype{
					ResourceType: resource.Type{
						Namespace: resource.ResourceNamespaceCore,
						Type:      resource.ResourceTypeComponent,
					},
					Subtype: resource.ResourceSubtypeArm,
				},
				Name: "arm1",
			},
			"uuid field for resource missing or invalid",
		},
		{
			"missing namespace",
			resource.Name{
				UUID: uuid.NewString(),
				ResourceSubtype: resource.Subtype{
					ResourceType: resource.Type{
						Type: resource.ResourceTypeComponent,
					},
					Subtype: resource.ResourceSubtypeArm,
				},
				Name: "arm1",
			},
			"namespace field for resource missing or invalid",
		},
		{
			"missing type",
			resource.Name{
				UUID: uuid.NewString(),
				ResourceSubtype: resource.Subtype{
					ResourceType: resource.Type{
						Namespace: resource.ResourceNamespaceCore,
					},
					Subtype: resource.ResourceSubtypeArm,
				},
				Name: "arm1",
			},
			"type field for resource missing or invalid",
		},
		{
			"missing subtype",
			resource.Name{
				UUID: uuid.NewString(),
				ResourceSubtype: resource.Subtype{
					ResourceType: resource.Type{
						Namespace: resource.ResourceNamespaceCore,
						Type:      resource.ResourceTypeComponent,
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
				ResourceSubtype: resource.Subtype{
					ResourceType: resource.Type{
						Namespace: resource.ResourceNamespaceCore,
						Type:      resource.ResourceTypeComponent,
					},
					Subtype: resource.ResourceSubtypeArm,
				},
			},
			"",
		},
		{
			"all fields included",
			resource.Name{
				UUID: uuid.NewString(),
				ResourceSubtype: resource.Subtype{
					ResourceType: resource.Type{
						Namespace: resource.ResourceNamespaceCore,
						Type:      resource.ResourceTypeComponent,
					},
					Subtype: resource.ResourceSubtypeArm,
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
