package resource_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/movementsensor"
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
			"reserved character in resource type",
			"rd:k",
			resource.ResourceTypeComponent,
			resource.Type{Namespace: "rd:k", ResourceType: resource.ResourceTypeComponent},
			"reserved character : used",
		},
		{
			"reserved charater in namespace",
			resource.ResourceNamespaceRDK,
			"compon:ent",
			resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: "compon:ent"},
			"reserved character : used",
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
			"reserved character in subtype name",
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			"sub:type",
			resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceRDK,
					ResourceType: resource.ResourceTypeComponent,
				},
				ResourceSubtype: "sub:type",
			},
			"reserved character : used",
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
			"rdk/components/arm/arm1",
			resource.Name{},
			"string \"rdk/components/arm/arm1\" is not a valid resource name",
		},
		{
			"too many colons",
			"rdk::component::arm/arm1",
			resource.Name{},
			"string \"rdk::component::arm/arm1\" is not a valid resource name",
		},
		{
			"too few colons",
			"rdk.component.arm/arm1",
			resource.Name{},
			"string \"rdk.component.arm/arm1\" is not a valid resource name",
		},
		{
			"missing name",
			"rdk:component:arm/",
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
			"rdk:component:movement_sensor/movementsensor1",
			resource.Name{
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceRDK,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: movementsensor.SubtypeName,
				},
				Name: "movementsensor1",
			},
			"",
		},
		{
			"with remotes",
			"rdk:component:movement_sensor/remote1:movementsensor1",
			resource.Name{
				Remote: "remote1",
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceRDK,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: movementsensor.SubtypeName,
				},
				Name: "movementsensor1",
			},
			"",
		},
		{
			"with remotes 2",
			"rdk:component:movement_sensor/remote1:remote2:movementsensor1",
			resource.Name{
				Remote: "remote1:remote2",
				Subtype: resource.Subtype{
					Type: resource.Type{
						Namespace:    resource.ResourceNamespaceRDK,
						ResourceType: resource.ResourceTypeComponent,
					},
					ResourceSubtype: movementsensor.SubtypeName,
				},
				Name: "movementsensor1",
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
			"rdk:component:arm/",
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
			"name field for resource is empty",
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
	n, err := resource.NewFromString("rdk:component:movement_sensor/movementsensor1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, n, test.ShouldResemble, resource.Name{
		Subtype: resource.Subtype{
			Type: resource.Type{
				Namespace:    resource.ResourceNamespaceRDK,
				ResourceType: resource.ResourceTypeComponent,
			},
			ResourceSubtype: movementsensor.SubtypeName,
		},
		Name: "movementsensor1",
	})

	test.That(t, n.ContainsRemoteNames(), test.ShouldBeFalse)

	n1 := n.PrependRemote("remote1")

	test.That(t, n1.ContainsRemoteNames(), test.ShouldBeTrue)
	test.That(t, n1.Remote, test.ShouldResemble, resource.RemoteName("remote1"))
	test.That(t, n1.String(), test.ShouldResemble, "rdk:component:movement_sensor/remote1:movementsensor1")

	test.That(t, n1, test.ShouldNotResemble, n)

	n2 := n1.PrependRemote("remote2")

	test.That(t, n2.ContainsRemoteNames(), test.ShouldBeTrue)
	test.That(t, n2.Remote, test.ShouldResemble, resource.RemoteName("remote2:remote1"))
	test.That(t, n2.String(), test.ShouldResemble, "rdk:component:movement_sensor/remote2:remote1:movementsensor1")

	n3 := n2.PopRemote()
	test.That(t, n3.ContainsRemoteNames(), test.ShouldBeTrue)
	test.That(t, n3.Remote, test.ShouldResemble, resource.RemoteName("remote1"))
	test.That(t, n3, test.ShouldResemble, n1)
	test.That(t, n3.String(), test.ShouldResemble, "rdk:component:movement_sensor/remote1:movementsensor1")

	n4 := n3.PopRemote()
	test.That(t, n4.ContainsRemoteNames(), test.ShouldBeFalse)
	test.That(t, n4.Remote, test.ShouldResemble, resource.RemoteName(""))
	test.That(t, n4, test.ShouldResemble, n)
	test.That(t, n4.String(), test.ShouldResemble, "rdk:component:movement_sensor/movementsensor1")

	resourceSubtype := resource.NewSubtype(
		"test",
		resource.ResourceTypeComponent,
		resource.SubtypeName("mycomponent"),
	)
	n5 := resource.NameFromSubtype(resourceSubtype, "test")
	test.That(t, n5.String(), test.ShouldResemble, "test:component:mycomponent/test")
	n5 = resource.NameFromSubtype(resourceSubtype, "")
	test.That(t, n5.String(), test.ShouldResemble, "test:component:mycomponent/")
	n5 = resource.NameFromSubtype(resourceSubtype, "remote1:test")
	test.That(t, n5.String(), test.ShouldResemble, "test:component:mycomponent/remote1:test")
	n5 = resource.NameFromSubtype(resourceSubtype, "remote2:remote1:test")
	test.That(t, n5.String(), test.ShouldResemble, "test:component:mycomponent/remote2:remote1:test")
	n5 = resource.NameFromSubtype(resourceSubtype, "remote1:")
	test.That(t, n5.String(), test.ShouldResemble, "test:component:mycomponent/remote1:")
	n5 = resource.NameFromSubtype(resourceSubtype, "remote2:remote1:")
	test.That(t, n5.String(), test.ShouldResemble, "test:component:mycomponent/remote2:remote1:")
}

func TestSubtypeFromString(t *testing.T) {
	//nolint:dupl
	for _, tc := range []struct {
		TestName   string
		StrSubtype string
		Expected   resource.Subtype
		Err        string
		ErrJSON    string
	}{
		{
			"valid",
			"rdk:component:arm",
			arm.Subtype,
			"",
			"",
		},
		{
			"valid with special characters and numbers",
			"acme_corp1:test-collection99:api_a2",
			resource.Subtype{
				Type:            resource.Type{Namespace: "acme_corp1", ResourceType: "test-collection99"},
				ResourceSubtype: "api_a2",
			},
			"",
			"",
		},
		{
			"invalid with slash",
			"acme/corp:test:subtypeA",
			resource.Subtype{},
			"not a valid subtype name",
			"invalid character",
		},
		{
			"invalid with caret",
			"acme:test:subtype^A",
			resource.Subtype{},
			"not a valid subtype name",
			"invalid character",
		},
		{
			"missing field",
			"acme:test",
			resource.Subtype{},
			"not a valid subtype name",
			"invalid character",
		},
		{
			"empty namespace",
			":test:subtypeA",
			resource.Subtype{},
			"not a valid subtype name",
			"invalid character",
		},
		{
			"empty family",
			"acme::subtypeA",
			resource.Subtype{},
			"not a valid subtype name",
			"invalid character",
		},
		{
			"empty name",
			"acme:test::",
			resource.Subtype{},
			"not a valid subtype name",
			"invalid character",
		},
		{
			"extra field",
			"acme:test:subtypeA:fail",
			resource.Subtype{},
			"not a valid subtype name",
			"invalid character",
		},
		{
			"mistaken resource name",
			"acme:test:subtypeA/fail",
			resource.Subtype{},
			"not a valid subtype name",
			"invalid character",
		},
		{
			"valid nested json",
			`{"namespace": "acme", "type": "test", "subtype": "subtypeB"}`,
			resource.Subtype{
				Type:            resource.Type{Namespace: "acme", ResourceType: "test"},
				ResourceSubtype: "subtypeB",
			},
			"not a valid subtype name",
			"",
		},
		{
			"invalid nested json type",
			`{"namespace": "acme", "type": "te^st", "subtype": "subtypeB"}`,
			resource.Subtype{},
			"not a valid subtype name",
			"not a valid type name",
		},
		{
			"invalid nested json namespace",
			`{"namespace": "$acme", "type": "test", "subtype": "subtypeB"}`,
			resource.Subtype{},
			"not a valid subtype name",
			"not a valid type namespace",
		},
		{
			"invalid nested json subtype",
			`{"namespace": "acme", "type": "test", "subtype": "subtype#B"}`,
			resource.Subtype{},
			"not a valid subtype name",
			"not a valid subtype name",
		},
		{
			"missing nested json field",
			`{"namespace": "acme", "name": "subtype#B"}`,
			resource.Subtype{},
			"not a valid subtype name",
			"field for resource missing",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed, err := resource.NewSubtypeFromString(tc.StrSubtype)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, observed.Validate(), test.ShouldBeNil)
				test.That(t, observed, test.ShouldResemble, tc.Expected)
				test.That(t, observed.String(), test.ShouldResemble, tc.Expected.String())
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}

			fromJSON := &resource.Subtype{}
			errJSON := fromJSON.UnmarshalJSON([]byte(tc.StrSubtype))
			if tc.ErrJSON == "" {
				test.That(t, errJSON, test.ShouldBeNil)
				test.That(t, fromJSON.Validate(), test.ShouldBeNil)
				test.That(t, fromJSON, test.ShouldResemble, &tc.Expected)
				test.That(t, fromJSON.String(), test.ShouldResemble, tc.Expected.String())
			} else {
				test.That(t, errJSON, test.ShouldNotBeNil)
				test.That(t, errJSON.Error(), test.ShouldContainSubstring, tc.ErrJSON)
			}
		})
	}
}
