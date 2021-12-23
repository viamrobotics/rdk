package resource_test

import (
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/resource"
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
			resource.ResourceNamespaceRDK,
			"",
			resource.Type{Namespace: resource.ResourceNamespaceRDK},
			"type field for resource missing or invalid",
		},
		{
			"all fields included",
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
			"",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := resource.NewType(tc.Namespace, tc.Type)
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
			resource.ResourceNamespaceRDK,
			"",
			arm.SubtypeName,
			resource.Subtype{
				Type: resource.Type{
					Namespace: resource.ResourceNamespaceRDK,
				},
				ResourceSubtype: arm.SubtypeName,
			},
			"type field for resource missing or invalid",
		},
		{
			"missing subtype",
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			"",
			resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceRDK,
					ResourceType: resource.ResourceTypeComponent,
				},
			},
			"subtype field for resource missing or invalid",
		},
		{
			"all fields included",
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			arm.SubtypeName,
			resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceRDK,
					ResourceType: resource.ResourceTypeComponent,
				},
				ResourceSubtype: arm.SubtypeName,
			},
			"",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := resource.NewSubtype(tc.Namespace, tc.Type, tc.Subtype)
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
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			arm.SubtypeName,
			"",
			resource.Name{
				UUID: "a5b161b9-dfa9-5eef-93d1-58431fd91212",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: arm.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			arm.SubtypeName,
			"arm1",
			resource.Name{
				UUID: "ded8a90b-0c77-5bda-baf5-b7e79bbdb28a",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: arm.SubtypeName,
				},
				Name: "arm1",
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := resource.NewName(tc.Namespace, tc.Type, tc.Subtype, tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
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
			"rdk/component/arm/arm1",
			resource.Name{},
			"there is more than one backslash",
		},
		{
			"too many colons",
			"rdk::component::arm/arm1",
			resource.Name{},
			"there are more than 2 colons",
		},
		{
			"too few colons",
			"rdk.component.arm/arm1",
			resource.Name{},
			"there are less than 2 colons",
		},
		{
			"missing name",
			"rdk:component:arm",
			resource.Name{
				UUID: "a5b161b9-dfa9-5eef-93d1-58431fd91212",
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceRDK,
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
				UUID: "ded8a90b-0c77-5bda-baf5-b7e79bbdb28a",
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceRDK,
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
			"rdk:component:compass/compass1",
			resource.Name{
				UUID: "286ec871-7aa7-5eba-98c0-6c3da28cdccb",
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceRDK,
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
						Namespace:    resource.ResourceNamespaceRDK,
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
						Namespace:    resource.ResourceNamespaceRDK,
						ResourceType: resource.ResourceTypeComponent,
					},
				},
				Name: "arm1",
			},
			"rdk:component:/arm1",
		},
		{
			"missing name",
			resource.Name{
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceRDK,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: arm.SubtypeName,
				},
			},
			"rdk:component:arm",
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
						Namespace:    resource.ResourceNamespaceRDK,
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
						Namespace:    resource.ResourceNamespaceRDK,
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
						Namespace: resource.ResourceNamespaceRDK,
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
						Namespace:    resource.ResourceNamespaceRDK,
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
						Namespace:    resource.ResourceNamespaceRDK,
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
						Namespace:    resource.ResourceNamespaceRDK,
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
