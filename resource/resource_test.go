package resource_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gps"
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
			"string is not a valid resource name",
		},
		{
			"too many colons",
			"rdk::component::arm/arm1",
			resource.Name{},
			"string is not a valid resource name",
		},
		{
			"too few colons",
			"rdk.component.arm/arm1",
			resource.Name{},
			"string is not a valid resource name",
		},
		{
			"missing name",
			"rdk:component:arm",
			resource.Name{
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
			"rdk:component:gps/gps1",
			resource.Name{
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceRDK,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: gps.SubtypeName,
				},
				Name: "gps1",
			},
			"",
		},
		{
			"with remotes",
			"remote1:rdk:component:gps/gps1",
			resource.Name{
				Remote: resource.Remote{
					Remote: "remote1",
				},
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceRDK,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: gps.SubtypeName,
				},
				Name: "gps1",
			},
			"",
		},
		{
			"with remotes 2",
			"remote1:remote2:rdk:component:gps/gps1",
			resource.Name{
				Remote: resource.Remote{
					Remote: "remote1:remote2",
				},
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceRDK,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: gps.SubtypeName,
				},
				Name: "gps1",
			},
			"",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed, err := resource.NewFromString(tc.Name)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, observed, test.ShouldResemble, tc.Expected)
				test.That(t, observed.String(), test.ShouldResemble, tc.Name)
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
			"missing namespace",
			resource.Name{
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

func TestRemoteResource(t *testing.T) {
	n, err := resource.NewFromString("rdk:component:gps/gps1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, n, test.ShouldResemble, resource.Name{
		Subtype: resource.Subtype{
			Type: resource.Type{
				Namespace:    resource.ResourceNamespaceRDK,
				ResourceType: resource.ResourceTypeComponent,
			},
			ResourceSubtype: gps.SubtypeName,
		},
		Name: "gps1",
	})

	test.That(t, n.IsRemoteResource(), test.ShouldBeFalse)

	n1 := n.PrependRemote("remote1")

	test.That(t, n1.IsRemoteResource(), test.ShouldBeTrue)
	test.That(t, n1.Remote.Remote, test.ShouldResemble, resource.RemoteName("remote1"))
	test.That(t, n1.String(), test.ShouldResemble, "remote1:rdk:component:gps/gps1")

	test.That(t, n1, test.ShouldNotResemble, n)

	n2 := n1.PrependRemote("remote2")

	test.That(t, n2.IsRemoteResource(), test.ShouldBeTrue)
	test.That(t, n2.Remote.Remote, test.ShouldResemble, resource.RemoteName("remote2:remote1"))
	test.That(t, n2.String(), test.ShouldResemble, "remote2:remote1:rdk:component:gps/gps1")

	n3 := n2.PopRemote()
	test.That(t, n3.IsRemoteResource(), test.ShouldBeTrue)
	test.That(t, n3.Remote.Remote, test.ShouldResemble, resource.RemoteName("remote1"))
	test.That(t, n3, test.ShouldResemble, n1)
	test.That(t, n3.String(), test.ShouldResemble, "remote1:rdk:component:gps/gps1")

	n4 := n3.PopRemote()
	test.That(t, n4.IsRemoteResource(), test.ShouldBeFalse)
	test.That(t, n4.Remote.Remote, test.ShouldResemble, resource.RemoteName(""))
	test.That(t, n4, test.ShouldResemble, n)
	test.That(t, n4.String(), test.ShouldResemble, "rdk:component:gps/gps1")

}
