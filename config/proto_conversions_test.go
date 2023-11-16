package config

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"syscall"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/lestrrat-go/jwx/jwk"
	packagespb "go.viam.com/api/app/packages/v1"
	pb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"go.viam.com/utils/jwks"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var testComponent = resource.Config{
	Name:      "some-name",
	API:       resource.NewAPI("some-namespace", "component", "some-type"),
	Model:     resource.DefaultModelFamily.WithModel("some-model"),
	DependsOn: []string{"dep1", "dep2"},
	Attributes: utils.AttributeMap{
		"attr1": 1,
		"attr2": "attr-string",
	},
	AssociatedResourceConfigs: []resource.AssociatedResourceConfig{
		// these will get rewritten in tests to simulate API data
		{
			// will resemble the same but become "foo:bar:baz"
			API: resource.NewAPI("foo", "bar", "baz"),
			Attributes: utils.AttributeMap{
				"attr1": 1,
			},
		},
		{
			// will stay the same
			API: resource.APINamespaceRDK.WithServiceType("some-type-2"),
			Attributes: utils.AttributeMap{
				"attr1": 2,
			},
		},
		{
			// will resemble the same but be just "some-type-3"
			API: resource.APINamespaceRDK.WithServiceType("some-type-3"),
			Attributes: utils.AttributeMap{
				"attr1": 3,
			},
		},
	},
	Frame: &referenceframe.LinkConfig{
		Parent:      "world",
		Translation: r3.Vector{X: 1, Y: 2, Z: 3},
		Orientation: &spatial.OrientationConfig{
			Type:  spatial.OrientationVectorDegreesType,
			Value: json.RawMessage([]byte(`{"th":0,"x":0,"y":0,"z":1}`)),
		},
		Geometry: &spatial.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
	},
}

var testRemote = Remote{
	Name:    "some-name",
	Address: "localohst:8080",
	Frame: &referenceframe.LinkConfig{
		Parent:      "world",
		Translation: r3.Vector{X: 1, Y: 2, Z: 3},
		Orientation: &spatial.OrientationConfig{
			Type:  spatial.OrientationVectorDegreesType,
			Value: json.RawMessage([]byte(`{"th":0,"x":0,"y":0,"z":1}`)),
		},
		Geometry: &spatial.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
	},
	Auth: RemoteAuth{
		Entity: "some-entity",
		Credentials: &rpc.Credentials{
			Type:    rpc.CredentialsTypeAPIKey,
			Payload: "payload",
		},
	},
	ManagedBy:               "managed-by",
	Insecure:                true,
	ConnectionCheckInterval: 1000000000,
	ReconnectInterval:       2000000000,
	AssociatedResourceConfigs: []resource.AssociatedResourceConfig{
		{
			API: resource.APINamespaceRDK.WithServiceType("some-type-1"),
			Attributes: utils.AttributeMap{
				"attr1": 1,
			},
		},
		{
			API: resource.APINamespaceRDK.WithServiceType("some-type-2"),
			Attributes: utils.AttributeMap{
				"attr1": 1,
			},
		},
	},
}

var testService = resource.Config{
	Name:  "some-name",
	API:   resource.NewAPI("some-namespace", "service", "some-type"),
	Model: resource.DefaultModelFamily.WithModel("some-model"),
	Attributes: utils.AttributeMap{
		"attr1": 1,
	},
	DependsOn: []string{"some-depends-on"},
	AssociatedResourceConfigs: []resource.AssociatedResourceConfig{
		{
			API: resource.APINamespaceRDK.WithServiceType("some-type-1"),
			Attributes: utils.AttributeMap{
				"attr1": 1,
			},
		},
	},
}

var testProcessConfig = pexec.ProcessConfig{
	ID:          "Some-id",
	Name:        "Some-name",
	Args:        []string{"arg1", "arg2"},
	CWD:         "/home",
	Environment: map[string]string{"SOME_VAR": "value"},
	OneShot:     true,
	Log:         true,
	StopSignal:  syscall.SIGINT,
	StopTimeout: time.Second,
}

var testNetworkConfig = NetworkConfig{
	NetworkConfigData: NetworkConfigData{
		FQDN:        "some.fqdn",
		BindAddress: "0.0.0.0:1234",
		TLSCertFile: "./cert.pub",
		TLSKeyFile:  "./cert.private",
		Sessions: SessionsConfig{
			HeartbeatWindow: 5 * time.Second,
		},
	},
}

var testAuthConfig = AuthConfig{
	Handlers: []AuthHandlerConfig{
		{
			Type: rpc.CredentialsTypeAPIKey,
			Config: utils.AttributeMap{
				"config-1": 1,
			},
		},
		{
			Type: rpc.CredentialsTypeAPIKey,
			Config: utils.AttributeMap{
				"config-2": 2,
			},
		},
	},
	TLSAuthEntities: []string{"tls1", "tls2"},
}

var testCloudConfig = Cloud{
	ID:             "some-id",
	Secret:         "some-secret",
	LocationSecret: "other-secret",
	LocationSecrets: []LocationSecret{
		{ID: "id1", Secret: "abc1"},
		{ID: "id2", Secret: "abc2"},
	},
	ManagedBy:         "managed-by",
	FQDN:              "some.fqdn",
	LocalFQDN:         "local.fqdn",
	SignalingAddress:  "0.0.0.0:8080",
	SignalingInsecure: true,
}

var testModule = Module{
	Name:        "testmod",
	ExePath:     "/tmp/test.mod",
	LogLevel:    "debug",
	Type:        ModuleTypeLocal,
	ModuleID:    "a:b",
	Environment: map[string]string{"SOME_VAR": "value"},
}

var testModuleWithError = Module{
	Name:        "testmodErr",
	ExePath:     "/tmp/test.mod",
	LogLevel:    "debug",
	Type:        ModuleTypeRegistry,
	ModuleID:    "mod:testmodErr",
	Environment: map[string]string{},
	Status: &AppValidationStatus{
		Error: "i have a bad error ah!",
	},
}

var testPackageConfig = PackageConfig{
	Name:    "package-name",
	Package: "some/package",
	Version: "v1",
	Type:    PackageTypeModule,
}

var (
	testInvalidModule    = Module{}
	testInvalidComponent = resource.Config{
		API: resource.NewAPI("", "component", ""),
	}
	testInvalidRemote        = Remote{}
	testInvalidProcessConfig = pexec.ProcessConfig{}
	testInvalidService       = resource.Config{
		API: resource.NewAPI("", "service", ""),
	}
	testInvalidPackage = PackageConfig{}
)

func init() {
	if _, err := testComponent.Validate("", resource.APITypeComponentName); err != nil {
		panic(err)
	}
	if _, err := testService.Validate("", resource.APITypeServiceName); err != nil {
		panic(err)
	}
}

//nolint:thelper
func validateModule(t *testing.T, actual, expected Module) {
	test.That(t, actual.Name, test.ShouldEqual, expected.Name)
	test.That(t, actual.ExePath, test.ShouldEqual, expected.ExePath)
	test.That(t, actual.LogLevel, test.ShouldEqual, expected.LogLevel)
	test.That(t, actual, test.ShouldResemble, expected)
}

func TestPackageConfigConversions(t *testing.T) {
	proto, err := PackageConfigToProto(&testPackageConfig)
	test.That(t, err, test.ShouldBeNil)

	out, err := PackageConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, testPackageConfig, test.ShouldResemble, *out)

	// test package with error
	pckWithErr := &PackageConfig{
		Name:    "testErr",
		Package: "package/test/me",
		Version: "1.0.0",
		Type:    PackageTypeMlModel,
		Status: &AppValidationStatus{
			Error: "help me error!",
		},
	}

	proto, err = PackageConfigToProto(pckWithErr)
	test.That(t, err, test.ShouldBeNil)

	out, err = PackageConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, pckWithErr, test.ShouldResemble, out)
}

func TestModuleConfigToProto(t *testing.T) {
	proto, err := ModuleConfigToProto(&testModule)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, proto.Status, test.ShouldBeNil)

	out, err := ModuleConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)

	validateModule(t, *out, testModule)

	protoWithErr, err := ModuleConfigToProto(&testModuleWithError)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, protoWithErr.Status, test.ShouldNotBeNil)

	out, err = ModuleConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)

	validateModule(t, *out, testModule)
}

//nolint:thelper
func validateComponent(t *testing.T, actual, expected resource.Config) {
	test.That(t, actual.Name, test.ShouldEqual, expected.Name)
	test.That(t, actual.API, test.ShouldResemble, expected.API)
	test.That(t, actual.Model, test.ShouldResemble, expected.Model)
	test.That(t, actual.DependsOn, test.ShouldResemble, expected.DependsOn)
	test.That(t, actual.Attributes.Int("attr1", 0), test.ShouldEqual, expected.Attributes.Int("attr1", -1))
	test.That(t, actual.Attributes.String("attr2"), test.ShouldEqual, expected.Attributes.String("attr2"))

	test.That(t, actual.AssociatedResourceConfigs, test.ShouldHaveLength, 3)
	test.That(t, actual.AssociatedResourceConfigs[0].API, test.ShouldResemble, expected.AssociatedResourceConfigs[0].API)
	test.That(t,
		actual.AssociatedResourceConfigs[0].Attributes.Int("attr1", 0),
		test.ShouldEqual,
		expected.AssociatedResourceConfigs[0].Attributes.Int("attr1", -1))
	test.That(t,
		actual.AssociatedResourceConfigs[1].API,
		test.ShouldResemble,
		expected.AssociatedResourceConfigs[1].API)
	test.That(t,
		actual.AssociatedResourceConfigs[1].Attributes.Int("attr1", 0),
		test.ShouldEqual,
		expected.AssociatedResourceConfigs[1].Attributes.Int("attr1", -1))
	test.That(t,
		actual.AssociatedResourceConfigs[2].API,
		test.ShouldResemble,
		expected.AssociatedResourceConfigs[2].API)
	test.That(t,
		actual.AssociatedResourceConfigs[2].Attributes.Int("attr1", 0),
		test.ShouldEqual,
		expected.AssociatedResourceConfigs[2].Attributes.Int("attr1", -1))

	f1, err := actual.Frame.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	f2, err := testComponent.Frame.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f1, test.ShouldResemble, f2)
}

func rewriteTestComponentProto(conf *pb.ComponentConfig) {
	conf.ServiceConfigs[0].Type = "foo:bar:baz"
	conf.ServiceConfigs[2].Type = "some-type-3"
}

func TestComponentConfigToProto(t *testing.T) {
	proto, err := ComponentConfigToProto(&testComponent)
	test.That(t, err, test.ShouldBeNil)
	rewriteTestComponentProto(proto)

	out, err := ComponentConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)

	validateComponent(t, *out, testComponent)

	for _, tc := range []struct {
		Name string
		Conf resource.Config
	}{
		{
			Name: "basic component with internal API",
			Conf: resource.Config{
				Name:  "foo",
				API:   resource.APINamespaceRDK.WithComponentType("base"),
				Model: resource.DefaultModelFamily.WithModel("fake"),
			},
		},
		{
			Name: "basic component with external API",
			Conf: resource.Config{
				Name:  "foo",
				API:   resource.NewAPI("acme", "component", "gizmo"),
				Model: resource.DefaultModelFamily.WithModel("fake"),
			},
		},
		{
			Name: "basic component with external model",
			Conf: resource.Config{
				Name:  "foo",
				API:   resource.NewAPI("acme", "component", "gizmo"),
				Model: resource.NewModel("acme", "test", "model"),
			},
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := tc.Conf.Validate("", resource.APITypeComponentName)
			test.That(t, err, test.ShouldBeNil)
			proto, err := ComponentConfigToProto(&tc.Conf)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, proto.Api, test.ShouldEqual, tc.Conf.API.String())
			test.That(t, proto.Model, test.ShouldEqual, tc.Conf.Model.String())
			test.That(t, proto.Namespace, test.ShouldEqual, tc.Conf.API.Type.Namespace)
			test.That(t, proto.Type, test.ShouldEqual, tc.Conf.API.SubtypeName)
			out, err := ComponentConfigFromProto(proto)
			test.That(t, err, test.ShouldBeNil)
			_, err = out.Validate("test", resource.APITypeComponentName)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, out, test.ShouldNotBeNil)
			test.That(t, out, test.ShouldResemble, &tc.Conf)
			test.That(t, out.API, test.ShouldResemble, out.API)
			test.That(t, out.Model, test.ShouldResemble, out.Model)
		})
	}
}

func TestFrameConfigFromProto(t *testing.T) {
	expectedFrameWithOrientation := func(or spatial.Orientation) *referenceframe.LinkConfig {
		orCfg, err := spatial.NewOrientationConfig(or)
		test.That(t, err, test.ShouldBeNil)
		return &referenceframe.LinkConfig{
			Parent:      "world",
			Translation: r3.Vector{X: 1, Y: 2, Z: 3},
			Orientation: orCfg,
		}
	}
	createNewFrame := func(or *pb.Orientation) *pb.Frame {
		return &pb.Frame{
			Parent: "world",
			Translation: &pb.Translation{
				X: 1,
				Y: 2,
				Z: 3,
			},
			Orientation: or,
		}
	}

	orRadians := spatial.NewOrientationVector()
	orRadians.OX = 1
	orRadians.OY = 2
	orRadians.OZ = 3
	orRadians.Theta = 4

	orDegress := spatial.NewOrientationVectorDegrees()
	orDegress.OX = 1
	orDegress.OY = 2
	orDegress.OZ = 3
	orDegress.Theta = 4

	orR4AA := spatial.NewR4AA()
	orR4AA.RX = 1
	orR4AA.RY = 2
	orR4AA.RZ = 3
	orR4AA.Theta = 4

	orEulerAngles := spatial.NewEulerAngles()
	orEulerAngles.Roll = 1
	orEulerAngles.Pitch = 2
	orEulerAngles.Yaw = 3

	testCases := []struct {
		name          string
		expectedFrame *referenceframe.LinkConfig
		inputFrame    *pb.Frame
	}{
		{
			"with orientation vector radians",
			expectedFrameWithOrientation(orRadians),
			createNewFrame(&pb.Orientation{
				Type: &pb.Orientation_VectorRadians{VectorRadians: &pb.Orientation_OrientationVectorRadians{Theta: 4, X: 1, Y: 2, Z: 3}},
			}),
		},
		{
			"with orientation vector degrees",
			expectedFrameWithOrientation(orDegress),
			createNewFrame(&pb.Orientation{
				Type: &pb.Orientation_VectorDegrees{VectorDegrees: &pb.Orientation_OrientationVectorDegrees{Theta: 4, X: 1, Y: 2, Z: 3}},
			}),
		},
		{
			"with orientation R4AA",
			expectedFrameWithOrientation(orR4AA),
			createNewFrame(&pb.Orientation{
				Type: &pb.Orientation_AxisAngles_{AxisAngles: &pb.Orientation_AxisAngles{Theta: 4, X: 1, Y: 2, Z: 3}},
			}),
		},
		{
			"with orientation EulerAngles",
			expectedFrameWithOrientation(orEulerAngles),
			createNewFrame(&pb.Orientation{
				Type: &pb.Orientation_EulerAngles_{EulerAngles: &pb.Orientation_EulerAngles{Roll: 1, Pitch: 2, Yaw: 3}},
			}),
		},
		{
			"with orientation Quaternion",
			expectedFrameWithOrientation(&spatial.Quaternion{Real: 1, Imag: 2, Jmag: 3, Kmag: 4}),
			createNewFrame(&pb.Orientation{
				Type: &pb.Orientation_Quaternion_{Quaternion: &pb.Orientation_Quaternion{W: 1, X: 2, Y: 3, Z: 4}},
			}),
		},
		{
			"with no orientation",
			expectedFrameWithOrientation(nil),
			createNewFrame(nil),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			frameOut, err := FrameConfigFromProto(testCase.inputFrame)
			test.That(t, err, test.ShouldBeNil)
			f1, err := frameOut.ParseConfig()
			test.That(t, err, test.ShouldBeNil)
			f2, err := testCase.expectedFrame.ParseConfig()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, f1, test.ShouldResemble, f2)
		})
	}
}

//nolint:thelper
func validateRemote(t *testing.T, actual, expected Remote) {
	test.That(t, actual.Name, test.ShouldEqual, expected.Name)
	test.That(t, actual.Address, test.ShouldEqual, expected.Address)
	test.That(t, actual.ManagedBy, test.ShouldEqual, expected.ManagedBy)
	test.That(t, actual.Insecure, test.ShouldEqual, expected.Insecure)
	test.That(t, actual.ReconnectInterval, test.ShouldEqual, expected.ReconnectInterval)
	test.That(t, actual.ConnectionCheckInterval, test.ShouldEqual, expected.ConnectionCheckInterval)
	test.That(t, actual.Auth, test.ShouldResemble, expected.Auth)
	f1, err := actual.Frame.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	f2, err := testComponent.Frame.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f1, test.ShouldResemble, f2)

	test.That(t, actual.AssociatedResourceConfigs, test.ShouldHaveLength, 2)
	test.That(t,
		actual.AssociatedResourceConfigs[0].API,
		test.ShouldResemble,
		expected.AssociatedResourceConfigs[0].API)
	test.That(t,
		actual.AssociatedResourceConfigs[0].Attributes.Int("attr1", 0),
		test.ShouldEqual,
		expected.AssociatedResourceConfigs[0].Attributes.Int("attr1", -1))
	test.That(t,
		actual.AssociatedResourceConfigs[1].API,
		test.ShouldResemble,
		expected.AssociatedResourceConfigs[1].API)
	test.That(t,
		actual.AssociatedResourceConfigs[1].Attributes.Int("attr1", 0),
		test.ShouldEqual,
		expected.AssociatedResourceConfigs[1].Attributes.Int("attr1", -1))
}

func TestRemoteConfigToProto(t *testing.T) {
	t.Run("With RemoteAuth", func(t *testing.T) {
		proto, err := RemoteConfigToProto(&testRemote)
		test.That(t, err, test.ShouldBeNil)

		out, err := RemoteConfigFromProto(proto)
		test.That(t, err, test.ShouldBeNil)

		validateRemote(t, *out, testRemote)
	})

	t.Run("Without RemoteAuth", func(t *testing.T) {
		proto := pb.RemoteConfig{
			Name:    "some-name",
			Address: "localohst:8080",
		}

		out, err := RemoteConfigFromProto(&proto)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, out.Name, test.ShouldEqual, proto.Name)
		test.That(t, out.Address, test.ShouldEqual, proto.Address)
		test.That(t, out.Auth, test.ShouldResemble, RemoteAuth{})
	})
}

//nolint:thelper
func validateService(t *testing.T, actual, expected resource.Config) {
	test.That(t, actual.Name, test.ShouldEqual, expected.Name)
	test.That(t, actual.API, test.ShouldResemble, expected.API)
	test.That(t, actual.Model, test.ShouldResemble, expected.Model)
	test.That(t, actual.DependsOn, test.ShouldResemble, expected.DependsOn)
	test.That(t, actual.Attributes.Int("attr1", 0), test.ShouldEqual, expected.Attributes.Int("attr1", -1))
	test.That(t, actual.Attributes.String("attr2"), test.ShouldEqual, expected.Attributes.String("attr2"))
}

func TestServiceConfigToProto(t *testing.T) {
	proto, err := ServiceConfigToProto(&testService)
	test.That(t, err, test.ShouldBeNil)

	out, err := ServiceConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)

	validateService(t, *out, testService)

	for _, tc := range []struct {
		Name string
		Conf resource.Config
	}{
		{
			Name: "basic component with internal API",
			Conf: resource.Config{
				Name:                      "foo",
				API:                       resource.APINamespaceRDK.WithServiceType("base"),
				Model:                     resource.DefaultModelFamily.WithModel("fake"),
				AssociatedResourceConfigs: []resource.AssociatedResourceConfig{},
			},
		},
		{
			Name: "basic component with external API",
			Conf: resource.Config{
				Name:                      "foo",
				API:                       resource.NewAPI("acme", "service", "gizmo"),
				Model:                     resource.DefaultModelFamily.WithModel("fake"),
				AssociatedResourceConfigs: []resource.AssociatedResourceConfig{},
			},
		},
		{
			Name: "basic component with external model",
			Conf: resource.Config{
				Name:                      "foo",
				API:                       resource.NewAPI("acme", "service", "gizmo"),
				Model:                     resource.NewModel("acme", "test", "model"),
				AssociatedResourceConfigs: []resource.AssociatedResourceConfig{},
			},
		},
		{
			Name: "empty model name",
			Conf: resource.Config{
				Name:                      "foo",
				API:                       resource.NewAPI("acme", "service", "gizmo"),
				Model:                     resource.Model{},
				AssociatedResourceConfigs: []resource.AssociatedResourceConfig{},
			},
		},
		{
			Name: "associated service config",
			Conf: resource.Config{
				Name:  "foo",
				API:   resource.NewAPI("acme", "service", "gizmo"),
				Model: resource.Model{},
				AssociatedResourceConfigs: []resource.AssociatedResourceConfig{
					{
						API: resource.APINamespaceRDK.WithServiceType("some-type-1"),
						Attributes: utils.AttributeMap{
							"attr1": 1,
						},
					},
				},
			},
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			proto, err := ServiceConfigToProto(&tc.Conf)
			test.That(t, err, test.ShouldBeNil)

			out, err := ServiceConfigFromProto(proto)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, out, test.ShouldNotBeNil)

			test.That(t, out.String(), test.ShouldResemble, tc.Conf.String())
			_, err = out.Validate("test", resource.APITypeServiceName)
			test.That(t, err, test.ShouldBeNil)
		})
	}
}

func TestServiceConfigWithEmptyModelName(t *testing.T) {
	servicesConfigJSON := `
	{
		"type": "base_remote_control",
		"attributes": {},
		"depends_on": [],
		"name": "base_rc"
	}`

	var fromJSON resource.Config
	err := json.Unmarshal([]byte(servicesConfigJSON), &fromJSON)
	test.That(t, err, test.ShouldBeNil)

	// should have an empty model
	test.That(t, fromJSON.Model, test.ShouldResemble, resource.Model{})

	proto, err := ServiceConfigToProto(&fromJSON)
	test.That(t, err, test.ShouldBeNil)

	out, err := ServiceConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)

	test.That(t, out.Model, test.ShouldResemble, fromJSON.Model)
	test.That(t, out.Model.Validate(), test.ShouldBeNil)
	test.That(t, out.Model, test.ShouldResemble, resource.DefaultServiceModel)
	_, err = out.Validate("...", resource.APITypeServiceName)
	test.That(t, err, test.ShouldBeNil)
}

func TestProcessConfigToProto(t *testing.T) {
	proto, err := ProcessConfigToProto(&testProcessConfig)
	test.That(t, err, test.ShouldBeNil)
	out, err := ProcessConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, *out, test.ShouldResemble, testProcessConfig)
}

func TestNetworkConfigToProto(t *testing.T) {
	proto, err := NetworkConfigToProto(&testNetworkConfig)
	test.That(t, err, test.ShouldBeNil)
	out, err := NetworkConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, *out, test.ShouldResemble, testNetworkConfig)
}

//nolint:thelper
func validateAuthConfig(t *testing.T, actual, expected AuthConfig) {
	test.That(t, actual.TLSAuthEntities, test.ShouldResemble, expected.TLSAuthEntities)
	test.That(t, actual.Handlers, test.ShouldHaveLength, 2)
	test.That(t, actual.Handlers[0].Type, test.ShouldEqual, expected.Handlers[0].Type)
	test.That(t, actual.Handlers[0].Config.Int("config-1", 0), test.ShouldEqual, expected.Handlers[0].Config.Int("config-1", -1))
	test.That(t, actual.Handlers[1].Type, test.ShouldEqual, expected.Handlers[1].Type)
	test.That(t, actual.Handlers[1].Config.Int("config-2", 0), test.ShouldEqual, expected.Handlers[1].Config.Int("config-2", -1))
}

func TestAuthConfigToProto(t *testing.T) {
	t.Run("api-key auth handler", func(t *testing.T) {
		proto, err := AuthConfigToProto(&testAuthConfig)
		test.That(t, err, test.ShouldBeNil)
		out, err := AuthConfigFromProto(proto)
		test.That(t, err, test.ShouldBeNil)

		validateAuthConfig(t, *out, testAuthConfig)
	})

	t.Run("external auth config", func(t *testing.T) {
		keyset := jwk.NewSet()
		privKeyForWebAuth, err := rsa.GenerateKey(rand.Reader, 4096)
		test.That(t, err, test.ShouldBeNil)
		publicKeyForWebAuth, err := jwk.New(privKeyForWebAuth.PublicKey)
		test.That(t, err, test.ShouldBeNil)
		publicKeyForWebAuth.Set(jwk.KeyIDKey, "key-id-1")
		test.That(t, keyset.Add(publicKeyForWebAuth), test.ShouldBeTrue)

		authConfig := AuthConfig{
			TLSAuthEntities: []string{"tls1", "tls2"},
			ExternalAuthConfig: &ExternalAuthConfig{
				JSONKeySet: keysetToInterface(t, keyset).AsMap(),
			},
		}

		proto, err := AuthConfigToProto(&authConfig)
		test.That(t, err, test.ShouldBeNil)
		out, err := AuthConfigFromProto(proto)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, out.ExternalAuthConfig, test.ShouldResemble, authConfig.ExternalAuthConfig)
	})
}

func keysetToInterface(t *testing.T, keyset jwks.KeySet) *structpb.Struct {
	t.Helper()

	// hack around marshaling the KeySet into pb.Struct. Passing interface directly
	// does not work.
	jwksAsJSON, err := json.Marshal(keyset)
	test.That(t, err, test.ShouldBeNil)

	jwksAsInterface := map[string]interface{}{}
	err = json.Unmarshal(jwksAsJSON, &jwksAsInterface)
	test.That(t, err, test.ShouldBeNil)

	jwksAsStruct, err := structpb.NewStruct(jwksAsInterface)
	test.That(t, err, test.ShouldBeNil)

	return jwksAsStruct
}

func TestCloudConfigToProto(t *testing.T) {
	proto, err := CloudConfigToProto(&testCloudConfig)
	test.That(t, err, test.ShouldBeNil)
	out, err := CloudConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, *out, test.ShouldResemble, testCloudConfig)
}

func TestFromProto(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cloudConfig, err := CloudConfigToProto(&testCloudConfig)
	test.That(t, err, test.ShouldBeNil)

	remoteConfig, err := RemoteConfigToProto(&testRemote)
	test.That(t, err, test.ShouldBeNil)

	moduleConfig, err := ModuleConfigToProto(&testModule)
	test.That(t, err, test.ShouldBeNil)

	componentConfig, err := ComponentConfigToProto(&testComponent)
	test.That(t, err, test.ShouldBeNil)
	rewriteTestComponentProto(componentConfig)

	processConfig, err := ProcessConfigToProto(&testProcessConfig)
	test.That(t, err, test.ShouldBeNil)

	serviceConfig, err := ServiceConfigToProto(&testService)
	test.That(t, err, test.ShouldBeNil)

	networkConfig, err := NetworkConfigToProto(&testNetworkConfig)
	test.That(t, err, test.ShouldBeNil)

	authConfig, err := AuthConfigToProto(&testAuthConfig)
	test.That(t, err, test.ShouldBeNil)

	packageConfig, err := PackageConfigToProto(&testPackageConfig)
	test.That(t, err, test.ShouldBeNil)

	debug := true

	input := &pb.RobotConfig{
		Cloud:      cloudConfig,
		Remotes:    []*pb.RemoteConfig{remoteConfig},
		Modules:    []*pb.ModuleConfig{moduleConfig},
		Components: []*pb.ComponentConfig{componentConfig},
		Processes:  []*pb.ProcessConfig{processConfig},
		Services:   []*pb.ServiceConfig{serviceConfig},
		Packages:   []*pb.PackageConfig{packageConfig},
		Network:    networkConfig,
		Auth:       authConfig,
		Debug:      &debug,
	}

	out, err := FromProto(input, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, *out.Cloud, test.ShouldResemble, testCloudConfig)
	test.That(t, out.Remotes, test.ShouldHaveLength, 1)
	validateRemote(t, out.Remotes[0], testRemote)
	test.That(t, out.Modules, test.ShouldHaveLength, 1)
	validateModule(t, out.Modules[0], testModule)
	test.That(t, out.Components, test.ShouldHaveLength, 1)
	validateComponent(t, out.Components[0], testComponent)
	test.That(t, out.Processes, test.ShouldHaveLength, 1)
	test.That(t, out.Processes[0], test.ShouldResemble, testProcessConfig)
	test.That(t, out.Services, test.ShouldHaveLength, 1)
	validateService(t, out.Services[0], testService)
	test.That(t, out.Network, test.ShouldResemble, testNetworkConfig)
	validateAuthConfig(t, out.Auth, testAuthConfig)
	test.That(t, out.Debug, test.ShouldEqual, debug)
	test.That(t, out.Packages, test.ShouldHaveLength, 1)
	test.That(t, out.Packages[0], test.ShouldResemble, testPackageConfig)
}

func TestPartialStart(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cloudConfig, err := CloudConfigToProto(&testCloudConfig)
	test.That(t, err, test.ShouldBeNil)

	remoteConfig, err := RemoteConfigToProto(&testRemote)
	test.That(t, err, test.ShouldBeNil)

	moduleConfig, err := ModuleConfigToProto(&testModule)
	test.That(t, err, test.ShouldBeNil)

	componentConfig, err := ComponentConfigToProto(&testComponent)
	test.That(t, err, test.ShouldBeNil)
	rewriteTestComponentProto(componentConfig)

	processConfig, err := ProcessConfigToProto(&testProcessConfig)
	test.That(t, err, test.ShouldBeNil)

	serviceConfig, err := ServiceConfigToProto(&testService)
	test.That(t, err, test.ShouldBeNil)

	networkConfig, err := NetworkConfigToProto(&testNetworkConfig)
	test.That(t, err, test.ShouldBeNil)

	authConfig, err := AuthConfigToProto(&testAuthConfig)
	test.That(t, err, test.ShouldBeNil)

	packageConfig, err := PackageConfigToProto(&testPackageConfig)
	test.That(t, err, test.ShouldBeNil)

	remoteInvalidConfig, err := RemoteConfigToProto(&testInvalidRemote)
	test.That(t, err, test.ShouldBeNil)

	moduleInvalidConfig, err := ModuleConfigToProto(&testInvalidModule)
	test.That(t, err, test.ShouldBeNil)

	componentInvalidConfig, err := ComponentConfigToProto(&testInvalidComponent)
	test.That(t, err, test.ShouldBeNil)

	processInvalidConfig, err := ProcessConfigToProto(&testInvalidProcessConfig)
	test.That(t, err, test.ShouldBeNil)

	serviceInvalidConfig, err := ServiceConfigToProto(&testInvalidService)
	test.That(t, err, test.ShouldBeNil)

	packageInvalidConfig, err := PackageConfigToProto(&testInvalidPackage)
	test.That(t, err, test.ShouldBeNil)

	debug := true
	disablePartialStart := false

	input := &pb.RobotConfig{
		Cloud:               cloudConfig,
		Remotes:             []*pb.RemoteConfig{remoteConfig, remoteInvalidConfig},
		Modules:             []*pb.ModuleConfig{moduleConfig, moduleInvalidConfig},
		Components:          []*pb.ComponentConfig{componentConfig, componentInvalidConfig},
		Processes:           []*pb.ProcessConfig{processConfig, processInvalidConfig},
		Services:            []*pb.ServiceConfig{serviceConfig, serviceInvalidConfig},
		Packages:            []*pb.PackageConfig{packageConfig, packageInvalidConfig},
		Network:             networkConfig,
		Auth:                authConfig,
		Debug:               &debug,
		DisablePartialStart: &disablePartialStart,
	}

	out, err := FromProto(input, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, *out.Cloud, test.ShouldResemble, testCloudConfig)
	validateRemote(t, out.Remotes[0], testRemote)
	test.That(t, out.Remotes[1].Name, test.ShouldEqual, "")
	validateModule(t, out.Modules[0], testModule)
	test.That(t, out.Modules[1], test.ShouldResemble, testInvalidModule)
	test.That(t, len(out.Components), test.ShouldEqual, 1)
	validateComponent(t, out.Components[0], testComponent)
	// there should only be one valid component and service in our list
	test.That(t, out.Processes[0], test.ShouldResemble, testProcessConfig)
	test.That(t, out.Processes[1], test.ShouldResemble, testInvalidProcessConfig)
	test.That(t, len(out.Services), test.ShouldEqual, 1)
	validateService(t, out.Services[0], testService)
	test.That(t, out.Network, test.ShouldResemble, testNetworkConfig)
	validateAuthConfig(t, out.Auth, testAuthConfig)
	test.That(t, out.Debug, test.ShouldEqual, debug)
	test.That(t, out.Packages[0], test.ShouldResemble, testPackageConfig)
	test.That(t, out.Packages[1], test.ShouldResemble, testInvalidPackage)
}

func TestDisablePartialStart(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cloudConfig, err := CloudConfigToProto(&testCloudConfig)
	test.That(t, err, test.ShouldBeNil)

	remoteConfig, err := RemoteConfigToProto(&testRemote)
	test.That(t, err, test.ShouldBeNil)

	moduleConfig, err := ModuleConfigToProto(&testModule)
	test.That(t, err, test.ShouldBeNil)

	componentInvalidConfig, err := ComponentConfigToProto(&testInvalidComponent)
	test.That(t, err, test.ShouldBeNil)

	processConfig, err := ProcessConfigToProto(&testProcessConfig)
	test.That(t, err, test.ShouldBeNil)

	serviceConfig, err := ServiceConfigToProto(&testService)
	test.That(t, err, test.ShouldBeNil)

	networkConfig, err := NetworkConfigToProto(&testNetworkConfig)
	test.That(t, err, test.ShouldBeNil)

	authConfig, err := AuthConfigToProto(&testAuthConfig)
	test.That(t, err, test.ShouldBeNil)

	debug := true
	disablePartialStart := true

	input := &pb.RobotConfig{
		Cloud:               cloudConfig,
		Remotes:             []*pb.RemoteConfig{remoteConfig},
		Modules:             []*pb.ModuleConfig{moduleConfig},
		Components:          []*pb.ComponentConfig{componentInvalidConfig},
		Processes:           []*pb.ProcessConfig{processConfig},
		Services:            []*pb.ServiceConfig{serviceConfig},
		Network:             networkConfig,
		Auth:                authConfig,
		Debug:               &debug,
		DisablePartialStart: &disablePartialStart,
	}

	out, err := FromProto(input, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, out, test.ShouldBeNil)
}

func TestPackageTypeConversion(t *testing.T) {
	emptyType := PackageType("")
	_, err := PackageTypeToProto(emptyType)
	test.That(t, err, test.ShouldNotBeNil)

	moduleType := PackageType("module")
	converted, err := PackageTypeToProto(moduleType)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, converted, test.ShouldResemble, packagespb.PackageType_PACKAGE_TYPE_MODULE.Enum())

	badType := PackageType("invalid-package-type")
	converted, err = PackageTypeToProto(badType)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "invalid-package-type")
	test.That(t, converted, test.ShouldResemble, packagespb.PackageType_PACKAGE_TYPE_UNSPECIFIED.Enum())
}
