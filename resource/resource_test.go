package resource_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

func TestResourceType(t *testing.T) {
	for _, tc := range []struct {
		TestName  string
		Namespace resource.APINamespace
		Type      string
		Expected  resource.APIType
		Err       string
	}{
		{
			"missing namespace",
			"",
			resource.APITypeComponentName,
			resource.APIType{Name: resource.APITypeComponentName},
			"namespace field for resource missing or invalid",
		},
		{
			"missing type",
			resource.APINamespaceRDK,
			"",
			resource.APIType{Namespace: resource.APINamespaceRDK},
			"type field for resource missing or invalid",
		},

		{
			"reserved character in resource type",
			"rd:k",
			resource.APITypeComponentName,
			resource.APIType{Namespace: "rd:k", Name: resource.APITypeComponentName},
			"reserved character : used",
		},
		{
			"reserved charater in namespace",
			resource.APINamespaceRDK,
			"compon:ent",
			resource.APIType{Namespace: resource.APINamespaceRDK, Name: "compon:ent"},
			"reserved character : used",
		},
		{
			"all fields included",
			resource.APINamespaceRDK,
			resource.APITypeComponentName,
			resource.APIType{Namespace: resource.APINamespaceRDK, Name: resource.APITypeComponentName},
			"",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := tc.Namespace.WithType(tc.Type)
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

func TestResourceAPI(t *testing.T) {
	for _, tc := range []struct {
		TestName    string
		Namespace   resource.APINamespace
		Type        string
		SubtypeName string
		Expected    resource.API
		Err         string
	}{
		{
			"missing namespace",
			"",
			resource.APITypeComponentName,
			arm.SubtypeName,
			resource.API{
				Type: resource.APIType{
					Name: resource.APITypeComponentName,
				},
				SubtypeName: arm.SubtypeName,
			},
			"namespace field for resource missing or invalid",
		},
		{
			"missing type",
			resource.APINamespaceRDK,
			"",
			arm.SubtypeName,
			resource.API{
				Type: resource.APIType{
					Namespace: resource.APINamespaceRDK,
				},
				SubtypeName: arm.SubtypeName,
			},
			"type field for resource missing or invalid",
		},
		{
			"missing subtype",
			resource.APINamespaceRDK,
			resource.APITypeComponentName,
			"",
			resource.API{
				Type: resource.APIType{
					Namespace: resource.APINamespaceRDK,
					Name:      resource.APITypeComponentName,
				},
			},
			"subtype field for resource missing or invalid",
		},
		{
			"reserved character in subtype name",
			resource.APINamespaceRDK,
			resource.APITypeComponentName,
			"sub:type",
			resource.API{
				Type: resource.APIType{
					Namespace: resource.APINamespaceRDK,
					Name:      resource.APITypeComponentName,
				},
				SubtypeName: "sub:type",
			},
			"reserved character : used",
		},
		{
			"all fields included",
			resource.APINamespaceRDK,
			resource.APITypeComponentName,
			arm.SubtypeName,
			resource.API{
				Type: resource.APIType{
					Namespace: resource.APINamespaceRDK,
					Name:      resource.APITypeComponentName,
				},
				SubtypeName: arm.SubtypeName,
			},
			"",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := tc.Namespace.WithType(tc.Type).WithSubtype(tc.SubtypeName)
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
		TestName    string
		Namespace   resource.APINamespace
		Type        string
		SubtypeName string
		Name        string
		Expected    resource.Name
	}{
		{
			"missing name",
			resource.APINamespaceRDK,
			resource.APITypeComponentName,
			arm.SubtypeName,
			"",
			resource.Name{
				API: resource.API{
					Type:        resource.APIType{Namespace: resource.APINamespaceRDK, Name: resource.APITypeComponentName},
					SubtypeName: arm.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			resource.APINamespaceRDK,
			resource.APITypeComponentName,
			arm.SubtypeName,
			"arm1",
			resource.Name{
				API: resource.API{
					Type:        resource.APIType{Namespace: resource.APINamespaceRDK, Name: resource.APITypeComponentName},
					SubtypeName: arm.SubtypeName,
				},
				Name: "arm1",
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := resource.NewName(tc.Namespace.WithType(tc.Type).WithSubtype(tc.SubtypeName), tc.Name)
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
				API: resource.API{
					Type: resource.APIType{
						Namespace: resource.APINamespaceRDK,
						Name:      resource.APITypeComponentName,
					},
					SubtypeName: arm.SubtypeName,
				},
				Name: "",
			},
			"",
		},
		{
			"all fields included",
			arm.Named("arm1").String(),
			resource.Name{
				API: resource.API{
					Type: resource.APIType{
						Namespace: resource.APINamespaceRDK,
						Name:      resource.APITypeComponentName,
					},
					SubtypeName: arm.SubtypeName,
				},
				Name: "arm1",
			},
			"",
		},
		{
			"all fields included 2",
			"rdk:component:movement_sensor/movementsensor1",
			resource.Name{
				API: resource.API{
					Type: resource.APIType{
						Namespace: resource.APINamespaceRDK,
						Name:      resource.APITypeComponentName,
					},
					SubtypeName: movementsensor.SubtypeName,
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
				API: resource.API{
					Type: resource.APIType{
						Namespace: resource.APINamespaceRDK,
						Name:      resource.APITypeComponentName,
					},
					SubtypeName: movementsensor.SubtypeName,
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
				API: resource.API{
					Type: resource.APIType{
						Namespace: resource.APINamespaceRDK,
						Name:      resource.APITypeComponentName,
					},
					SubtypeName: movementsensor.SubtypeName,
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
				API: resource.API{
					Type: resource.APIType{
						Namespace: resource.APINamespaceRDK,
						Name:      resource.APITypeComponentName,
					},
					SubtypeName: arm.SubtypeName,
				},
				Name: "arm1",
			},
			arm.Named("arm1").String(),
		},
		{
			"missing subtype",
			resource.Name{
				API: resource.API{
					Type: resource.APIType{
						Namespace: resource.APINamespaceRDK,
						Name:      resource.APITypeComponentName,
					},
				},
				Name: "arm1",
			},
			"rdk:component:/arm1",
		},
		{
			"missing name",
			resource.Name{
				API: resource.API{
					Type: resource.APIType{
						Namespace: resource.APINamespaceRDK,
						Name:      resource.APITypeComponentName,
					},
					SubtypeName: arm.SubtypeName,
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
				API: resource.API{
					Type: resource.APIType{
						Name: resource.APITypeComponentName,
					},
					SubtypeName: arm.SubtypeName,
				},
				Name: "arm1",
			},
			"namespace field for resource missing or invalid",
		},
		{
			"missing type",
			resource.Name{
				API: resource.API{
					Type: resource.APIType{
						Namespace: resource.APINamespaceRDK,
					},
					SubtypeName: arm.SubtypeName,
				},
				Name: "arm1",
			},
			"type field for resource missing or invalid",
		},
		{
			"missing subtype",
			resource.Name{
				API: resource.API{
					Type: resource.APIType{
						Namespace: resource.APINamespaceRDK,
						Name:      resource.APITypeComponentName,
					},
				},
				Name: "arm1",
			},
			"subtype field for resource missing or invalid",
		},
		{
			"missing name",
			resource.Name{
				API: resource.API{
					Type: resource.APIType{
						Namespace: resource.APINamespaceRDK,
						Name:      resource.APITypeComponentName,
					},
					SubtypeName: arm.SubtypeName,
				},
			},
			"name field for resource is empty",
		},
		{
			"all fields included",
			resource.Name{
				API: resource.API{
					Type: resource.APIType{
						Namespace: resource.APINamespaceRDK,
						Name:      resource.APITypeComponentName,
					},
					SubtypeName: arm.SubtypeName,
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
		API: resource.API{
			Type: resource.APIType{
				Namespace: resource.APINamespaceRDK,
				Name:      resource.APITypeComponentName,
			},
			SubtypeName: movementsensor.SubtypeName,
		},
		Name: "movementsensor1",
	})

	test.That(t, n.ContainsRemoteNames(), test.ShouldBeFalse)

	n1 := n.PrependRemote("remote1")

	test.That(t, n1.ContainsRemoteNames(), test.ShouldBeTrue)
	test.That(t, n1.Remote, test.ShouldResemble, "remote1")
	test.That(t, n1.String(), test.ShouldResemble, "rdk:component:movement_sensor/remote1:movementsensor1")

	test.That(t, n1, test.ShouldNotResemble, n)

	n2 := n1.PrependRemote("remote2")

	test.That(t, n2.ContainsRemoteNames(), test.ShouldBeTrue)
	test.That(t, n2.Remote, test.ShouldResemble, "remote2:remote1")
	test.That(t, n2.String(), test.ShouldResemble, "rdk:component:movement_sensor/remote2:remote1:movementsensor1")

	n3 := n2.PopRemote()
	test.That(t, n3.ContainsRemoteNames(), test.ShouldBeTrue)
	test.That(t, n3.Remote, test.ShouldResemble, "remote1")
	test.That(t, n3, test.ShouldResemble, n1)
	test.That(t, n3.String(), test.ShouldResemble, "rdk:component:movement_sensor/remote1:movementsensor1")

	n4 := n3.PopRemote()
	test.That(t, n4.ContainsRemoteNames(), test.ShouldBeFalse)
	test.That(t, n4.Remote, test.ShouldResemble, "")
	test.That(t, n4, test.ShouldResemble, n)
	test.That(t, n4.String(), test.ShouldResemble, "rdk:component:movement_sensor/movementsensor1")

	resourceAPI := resource.APINamespace("test").WithComponentType("mycomponent")
	n5 := resource.NewName(resourceAPI, "test")
	test.That(t, n5.String(), test.ShouldResemble, "test:component:mycomponent/test")
	n5 = resource.NewName(resourceAPI, "")
	test.That(t, n5.String(), test.ShouldResemble, "test:component:mycomponent/")
	n5 = resource.NewName(resourceAPI, "remote1:test")
	test.That(t, n5.String(), test.ShouldResemble, "test:component:mycomponent/remote1:test")
	n5 = resource.NewName(resourceAPI, "remote2:remote1:test")
	test.That(t, n5.String(), test.ShouldResemble, "test:component:mycomponent/remote2:remote1:test")
	n5 = resource.NewName(resourceAPI, "remote1:")
	test.That(t, n5.String(), test.ShouldResemble, "test:component:mycomponent/remote1:")
	n5 = resource.NewName(resourceAPI, "remote2:remote1:")
	test.That(t, n5.String(), test.ShouldResemble, "test:component:mycomponent/remote2:remote1:")
}

func TestNewPossibleRDKServiceAPIFromString(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		StrAPI   string
		Expected resource.API
		Err      string
	}{
		{
			"valid",
			"rdk:component:arm",
			arm.API,
			"",
		},
		{
			"valid with special characters and numbers",
			"acme_corp1:test-collection99:api_a2",
			resource.API{
				Type:        resource.APIType{Namespace: "acme_corp1", Name: "test-collection99"},
				SubtypeName: "api_a2",
			},
			"",
		},
		{
			"invalid with slash",
			"acme/corp:test:subtypeA",
			resource.API{},
			"not a valid api name",
		},
		{
			"invalid with caret",
			"acme:test:subtype^A",
			resource.API{},
			"not a valid api name",
		},
		{
			"missing field",
			"acme:test",
			resource.API{},
			"not a valid api name",
		},
		{
			"empty namespace",
			":test:subtypeA",
			resource.API{},
			"not a valid api name",
		},
		{
			"empty family",
			"acme::subtypeA",
			resource.API{},
			"not a valid api name",
		},
		{
			"empty name",
			"acme:test::",
			resource.API{},
			"not a valid api name",
		},
		{
			"extra field",
			"acme:test:subtypeA:fail",
			resource.API{},
			"not a valid api name",
		},
		{
			"mistaken resource name",
			"acme:test:subtypeA/fail",
			resource.API{},
			"not a valid api name",
		},
		{
			"valid nested json",
			`{"namespace": "acme", "type": "test", "subtype": "subtypeB"}`,
			resource.API{
				Type:        resource.APIType{Namespace: "acme", Name: "test"},
				SubtypeName: "subtypeB",
			},
			"not a valid api name",
		},
		{
			"invalid nested json type",
			`{"namespace": "acme", "type": "te^st", "subtype": "subtypeB"}`,
			resource.API{},
			"not a valid api name",
		},
		{
			"invalid nested json namespace",
			`{"namespace": "$acme", "type": "test", "subtype": "subtypeB"}`,
			resource.API{},
			"not a valid api name",
		},
		{
			"invalid nested json subtype",
			`{"namespace": "acme", "type": "test", "subtype": "subtype#B"}`,
			resource.API{},
			"not a valid api name",
		},
		{
			"missing nested json field",
			`{"namespace": "acme", "name": "subtype#B"}`,
			resource.API{},
			"not a valid api name",
		},
		{
			"single name",
			`hello`,
			resource.APINamespaceRDK.WithServiceType("hello"),
			"",
		},
		{
			"double name",
			`uh:hello`,
			resource.API{},
			"not a valid api name",
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed, err := resource.NewPossibleRDKServiceAPIFromString(tc.StrAPI)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, observed.Validate(), test.ShouldBeNil)
				test.That(t, observed, test.ShouldResemble, tc.Expected)
				test.That(t, observed.String(), test.ShouldResemble, tc.Expected.String())
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}

func TestDependenciesLookup(t *testing.T) {
	deps := resource.Dependencies{}

	armName := arm.Named("foo")
	_, err := deps.Lookup(armName)
	test.That(t, err, test.ShouldBeError, resource.DependencyNotFoundError(armName))
	remoteArmName := arm.Named("robot1:foo")
	_, err = deps.Lookup(remoteArmName)
	test.That(t, err, test.ShouldBeError, resource.DependencyNotFoundError(remoteArmName))

	logger := golog.NewTestLogger(t)
	someArm, err := fake.NewArm(context.Background(), nil, resource.Config{ConvertedAttributes: &fake.Config{}}, logger)
	test.That(t, err, test.ShouldBeNil)
	deps[armName] = someArm

	t.Log("adding an arm by just its name should allow it to be looked up by that same name")
	res, err := deps.Lookup(armName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, someArm)

	t.Log("but not the remote name since its too specific")
	_, err = deps.Lookup(remoteArmName)
	test.That(t, err, test.ShouldBeError, resource.DependencyNotFoundError(remoteArmName))

	deps = resource.Dependencies{}
	deps[remoteArmName] = someArm

	t.Log("adding an arm by its remote name should allow it to be looked up by the same remote name")
	res, err = deps.Lookup(remoteArmName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, someArm)

	t.Log("as well as just the arm name since it is not specific")
	res, err = deps.Lookup(armName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, someArm)

	remoteArmName2 := arm.Named("robot2:foo")
	deps[remoteArmName2] = someArm
	t.Log("but not if there are two remote names with the same naked name")
	_, err = deps.Lookup(armName)
	test.That(t, err, test.ShouldBeError, utils.NewRemoteResourceClashError(armName.Name))

	sensorName := movementsensor.Named("foo")
	_, err = deps.Lookup(sensorName)
	test.That(t, err, test.ShouldBeError, resource.DependencyNotFoundError(sensorName))

	remoteSensorName := movementsensor.Named("robot1:foo")
	_, err = deps.Lookup(remoteSensorName)
	test.That(t, err, test.ShouldBeError, resource.DependencyNotFoundError(remoteSensorName))
}
