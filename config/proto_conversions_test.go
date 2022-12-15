package config

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/lestrrat-go/jwx/jwk"
	pb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"go.viam.com/utils/jwks"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/rpc/oauth"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

var testComponent = Component{
	Name:      "some-name",
	Type:      "some-type",
	Namespace: "some-namespace",
	Model:     "some-model",
	DependsOn: []string{"dep1", "dep2"},
	Attributes: AttributeMap{
		"attr1": 1,
		"attr2": "attr-string",
	},
	ServiceConfig: []ResourceLevelServiceConfig{
		{
			Type: "some-type-1",
			Attributes: AttributeMap{
				"attr1": 1,
			},
		},
		{
			Type: "some-type-2",
			Attributes: AttributeMap{
				"attr1": 1,
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

var testFrame = referenceframe.LinkConfig{
	Parent: "world",
	Translation: r3.Vector{
		X: 1,
		Y: 2,
		Z: 3,
	},
	Orientation: &spatial.OrientationConfig{
		Type:  spatial.EulerAnglesType,
		Value: json.RawMessage([]byte(`{"roll":0,"pitch":0,"yaw":0}`)),
	},
	Geometry: &spatial.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
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
	ServiceConfig: []ResourceLevelServiceConfig{
		{
			Type: "some-type-1",
			Attributes: AttributeMap{
				"attr1": 1,
			},
		},
		{
			Type: "some-type-2",
			Attributes: AttributeMap{
				"attr1": 1,
			},
		},
	},
}

var testService = Service{
	Name:      "some-name",
	Namespace: "some-namespace",
	Type:      "some-type",
	Attributes: AttributeMap{
		"attr1": 1,
	},
	Model:     "some-model",
	DependsOn: []string{"some-depends-on"},
}

var testProcessConfig = pexec.ProcessConfig{
	ID:      "Some-id",
	Name:    "Some-name",
	Args:    []string{"arg1", "arg2"},
	CWD:     "/home",
	OneShot: true,
	Log:     true,
}

var testNetworkConfig = NetworkConfig{
	NetworkConfigData: NetworkConfigData{
		FQDN:        "some.fqdn",
		BindAddress: "0.0.0.0:1234",
		TLSCertFile: "./cert.pub",
		TLSKeyFile:  "./cert.private",
	},
}

var testAuthConfig = AuthConfig{
	Handlers: []AuthHandlerConfig{
		{
			Type: rpc.CredentialsTypeAPIKey,
			Config: AttributeMap{
				"config-1": 1,
			},
		},
		{
			Type: rpc.CredentialsTypeAPIKey,
			Config: AttributeMap{
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

//nolint:thelper
func validateComponent(t *testing.T, actual, expected Component) {
	test.That(t, actual.Name, test.ShouldEqual, expected.Name)
	test.That(t, actual.Type, test.ShouldEqual, expected.Type)
	test.That(t, actual.Namespace, test.ShouldEqual, expected.Namespace)
	test.That(t, actual.Model, test.ShouldEqual, expected.Model)
	test.That(t, actual.DependsOn, test.ShouldResemble, expected.DependsOn)
	test.That(t, actual.Attributes.Int("attr1", 0), test.ShouldEqual, expected.Attributes.Int("attr1", -1))
	test.That(t, actual.Attributes.String("attr2"), test.ShouldEqual, expected.Attributes.String("attr2"))

	test.That(t, actual.ServiceConfig, test.ShouldHaveLength, 2)
	test.That(t, actual.ServiceConfig[0].Type, test.ShouldEqual, expected.ServiceConfig[0].Type)
	test.That(t, actual.ServiceConfig[0].Attributes.Int("attr1", 0), test.ShouldEqual, expected.ServiceConfig[0].Attributes.Int("attr1", -1))
	test.That(t, actual.ServiceConfig[1].Type, test.ShouldEqual, expected.ServiceConfig[1].Type)
	test.That(t, actual.ServiceConfig[1].Attributes.Int("attr1", 0), test.ShouldEqual, expected.ServiceConfig[1].Attributes.Int("attr1", -1))

	f1, err := actual.Frame.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	f2, err := testComponent.Frame.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f1, test.ShouldResemble, f2)
}

func TestComponentConfigToProto(t *testing.T) {
	proto, err := ComponentConfigToProto(&testComponent)
	test.That(t, err, test.ShouldBeNil)

	out, err := ComponentConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)

	validateComponent(t, *out, testComponent)
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

	test.That(t, actual.ServiceConfig, test.ShouldHaveLength, 2)
	test.That(t, actual.ServiceConfig[0].Type, test.ShouldEqual, expected.ServiceConfig[0].Type)
	test.That(t, actual.ServiceConfig[0].Attributes.Int("attr1", 0), test.ShouldEqual, expected.ServiceConfig[0].Attributes.Int("attr1", -1))
	test.That(t, actual.ServiceConfig[1].Type, test.ShouldEqual, expected.ServiceConfig[1].Type)
	test.That(t, actual.ServiceConfig[1].Attributes.Int("attr1", 0), test.ShouldEqual, expected.ServiceConfig[1].Attributes.Int("attr1", -1))
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
func validateService(t *testing.T, actual, expected Service) {
	test.That(t, actual.Name, test.ShouldEqual, expected.Name)
	test.That(t, actual.Type, test.ShouldEqual, expected.Type)
	test.That(t, actual.Namespace, test.ShouldEqual, expected.Namespace)
	test.That(t, actual.Model, test.ShouldEqual, expected.Model)
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

	t.Run("web oauth auth handler", func(t *testing.T) {
		keyset := jwk.NewSet()
		privKeyForWebAuth, err := rsa.GenerateKey(rand.Reader, 4096)
		test.That(t, err, test.ShouldBeNil)
		publicKeyForWebAuth, err := jwk.New(privKeyForWebAuth.PublicKey)
		test.That(t, err, test.ShouldBeNil)
		publicKeyForWebAuth.Set(jwk.KeyIDKey, "key-id-1")
		test.That(t, keyset.Add(publicKeyForWebAuth), test.ShouldBeTrue)

		authConfig := AuthConfig{
			Handlers: []AuthHandlerConfig{
				{
					Type:   oauth.CredentialsTypeOAuthWeb,
					Config: AttributeMap{},
					WebOAuthConfig: &WebOAuthConfig{
						AllowedAudiences: []string{"aud1", "aud2"},
						JSONKeySet:       keysetToInterface(t, keyset).AsMap(),
					},
				},
			},
			TLSAuthEntities: []string{"tls1", "tls2"},
		}

		proto, err := AuthConfigToProto(&authConfig)
		test.That(t, err, test.ShouldBeNil)
		out, err := AuthConfigFromProto(proto)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, out.Handlers[0], test.ShouldResemble, authConfig.Handlers[0])
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
	cloudConfig, err := CloudConfigToProto(&testCloudConfig)
	test.That(t, err, test.ShouldBeNil)

	remoteConfig, err := RemoteConfigToProto(&testRemote)
	test.That(t, err, test.ShouldBeNil)

	componentConfig, err := ComponentConfigToProto(&testComponent)
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

	input := &pb.RobotConfig{
		Cloud:      cloudConfig,
		Remotes:    []*pb.RemoteConfig{remoteConfig},
		Components: []*pb.ComponentConfig{componentConfig},
		Processes:  []*pb.ProcessConfig{processConfig},
		Services:   []*pb.ServiceConfig{serviceConfig},
		Network:    networkConfig,
		Auth:       authConfig,
		Debug:      &debug,
	}

	out, err := FromProto(input)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, *out.Cloud, test.ShouldResemble, testCloudConfig)
	validateRemote(t, out.Remotes[0], testRemote)
	validateComponent(t, out.Components[0], testComponent)
	test.That(t, out.Processes[0], test.ShouldResemble, testProcessConfig)
	validateService(t, out.Services[0], testService)
	test.That(t, out.Network, test.ShouldResemble, testNetworkConfig)
	validateAuthConfig(t, out.Auth, testAuthConfig)
	test.That(t, out.Debug, test.ShouldEqual, debug)
}
