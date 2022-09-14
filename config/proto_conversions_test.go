package config

import (
	"testing"

	pb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	spatial "go.viam.com/rdk/spatialmath"
)

func TestComponentConfigToProto(t *testing.T) {
	component := Component{
		Name:      "some-name",
		Type:      "some-type",
		Namespace: "some-namespace",
		SubType:   "some-sub-type",
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
		Frame: &Frame{
			Parent: "world",
			Translation: spatial.TranslationConfig{
				X: 1,
				Y: 2,
				Z: 3,
			},
			Orientation: spatial.NewOrientationVector(),
		},
	}

	proto, err := ComponentConfigToProto(&component)
	test.That(t, err, test.ShouldBeNil)

	out, err := ComponentConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)

	test.That(t, out.Name, test.ShouldEqual, component.Name)
	test.That(t, out.Type, test.ShouldEqual, component.Type)
	test.That(t, out.Namespace, test.ShouldEqual, component.Namespace)
	test.That(t, out.SubType, test.ShouldEqual, component.SubType)
	test.That(t, out.Model, test.ShouldEqual, component.Model)
	test.That(t, out.DependsOn, test.ShouldResemble, component.DependsOn)
	test.That(t, out.Attributes.Int("attr1", 0), test.ShouldEqual, component.Attributes.Int("attr1", -1))
	test.That(t, out.Attributes.String("attr2"), test.ShouldEqual, component.Attributes.String("attr2"))

	test.That(t, out.ServiceConfig, test.ShouldHaveLength, 2)
	test.That(t, out.ServiceConfig[0].Type, test.ShouldEqual, component.ServiceConfig[0].Type)
	test.That(t, out.ServiceConfig[0].Attributes.Int("attr1", 0), test.ShouldEqual, component.ServiceConfig[0].Attributes.Int("attr1", -1))
	test.That(t, out.ServiceConfig[1].Type, test.ShouldEqual, component.ServiceConfig[1].Type)
	test.That(t, out.ServiceConfig[1].Attributes.Int("attr1", 0), test.ShouldEqual, component.ServiceConfig[1].Attributes.Int("attr1", -1))

	test.That(t, out.Frame, test.ShouldResemble, component.Frame)
	test.That(t, out.Frame, test.ShouldResemble, component.Frame)
}

func TestFrameConfigFromProto(t *testing.T) {
	expectedFrameWithOrientation := func(or spatial.Orientation) *Frame {
		return &Frame{
			Parent: "world",
			Translation: spatial.TranslationConfig{
				X: 1,
				Y: 2,
				Z: 3,
			},
			Orientation: or,
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
		expectedFrame *Frame
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
			test.That(t, frameOut, test.ShouldResemble, testCase.expectedFrame)
		})
	}
}

func TestRemoteConfigToProto(t *testing.T) {
	t.Run("With RemoteAuth", func(t *testing.T) {
		remote := Remote{
			Name:    "some-name",
			Address: "localohst:8080",
			Prefix:  true,
			Frame: &Frame{
				Parent: "world",
				Translation: spatial.TranslationConfig{
					X: 1,
					Y: 2,
					Z: 3,
				},
				Orientation: spatial.NewOrientationVector(),
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

		proto, err := RemoteConfigToProto(&remote)
		test.That(t, err, test.ShouldBeNil)

		out, err := RemoteConfigFromProto(proto)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, out.Name, test.ShouldEqual, remote.Name)
		test.That(t, out.Address, test.ShouldEqual, remote.Address)
		test.That(t, out.Prefix, test.ShouldEqual, remote.Prefix)
		test.That(t, out.ManagedBy, test.ShouldEqual, remote.ManagedBy)
		test.That(t, out.Insecure, test.ShouldEqual, remote.Insecure)
		test.That(t, out.ReconnectInterval, test.ShouldEqual, remote.ReconnectInterval)
		test.That(t, out.ConnectionCheckInterval, test.ShouldEqual, remote.ConnectionCheckInterval)
		test.That(t, out.Auth, test.ShouldResemble, remote.Auth)
		test.That(t, out.Frame, test.ShouldResemble, remote.Frame)

		test.That(t, out.ServiceConfig, test.ShouldHaveLength, 2)
		test.That(t, out.ServiceConfig[0].Type, test.ShouldEqual, remote.ServiceConfig[0].Type)
		test.That(t, out.ServiceConfig[0].Attributes.Int("attr1", 0), test.ShouldEqual, remote.ServiceConfig[0].Attributes.Int("attr1", -1))
		test.That(t, out.ServiceConfig[1].Type, test.ShouldEqual, remote.ServiceConfig[1].Type)
		test.That(t, out.ServiceConfig[1].Attributes.Int("attr1", 0), test.ShouldEqual, remote.ServiceConfig[1].Attributes.Int("attr1", -1))
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

func TestServiceConfigToProto(t *testing.T) {
	service := Service{
		Name:      "some-name",
		Namespace: "some-namespace",
		Type:      "some-type",
		Attributes: AttributeMap{
			"attr1": 1,
		},
	}

	proto, err := ServiceConfigToProto(&service)
	test.That(t, err, test.ShouldBeNil)

	out, err := ServiceConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, out.Name, test.ShouldEqual, service.Name)
	test.That(t, out.Namespace, test.ShouldEqual, service.Namespace)
	test.That(t, out.Type, test.ShouldEqual, service.Type)
	test.That(t, out.Attributes.Int("attr1", 0), test.ShouldEqual, service.Attributes.Int("attr1", -1))
}

func TestProcessConfigToProto(t *testing.T) {
	service := pexec.ProcessConfig{
		ID:      "Some-id",
		Name:    "Some-name",
		Args:    []string{"arg1", "arg2"},
		CWD:     "/home",
		OneShot: true,
		Log:     true,
	}

	proto, err := ProcessConfigToProto(&service)
	test.That(t, err, test.ShouldBeNil)
	out, err := ProcessConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, *out, test.ShouldResemble, service)
}

func TestNetworkConfigToProto(t *testing.T) {
	in := NetworkConfig{
		NetworkConfigData: NetworkConfigData{
			FQDN:        "some.fqdn",
			BindAddress: "0.0.0.0:1234",
			TLSCertFile: "./cert.pub",
			TLSKeyFile:  "./cert.private",
		},
	}

	proto, err := NetworkConfigToProto(&in)
	test.That(t, err, test.ShouldBeNil)
	out, err := NetworkConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, *out, test.ShouldResemble, in)
}

func TestAuthConfigToProto(t *testing.T) {
	in := AuthConfig{
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

	proto, err := AuthConfigToProto(&in)
	test.That(t, err, test.ShouldBeNil)
	out, err := AuthConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, out.TLSAuthEntities, test.ShouldResemble, in.TLSAuthEntities)
	test.That(t, out.Handlers, test.ShouldHaveLength, 2)
	test.That(t, out.Handlers[0].Type, test.ShouldEqual, in.Handlers[0].Type)
	test.That(t, out.Handlers[0].Config.Int("config-1", 0), test.ShouldEqual, out.Handlers[0].Config.Int("config-1", -1))
	test.That(t, out.Handlers[1].Type, test.ShouldEqual, in.Handlers[1].Type)
	test.That(t, out.Handlers[1].Config.Int("config-2", 0), test.ShouldEqual, out.Handlers[1].Config.Int("config-2", -1))
}

func TestCloudConfigToProto(t *testing.T) {
	in := Cloud{
		ID:                "some-id",
		Secret:            "some-secret",
		LocationSecret:    "other-secret",
		ManagedBy:         "managed-by",
		FQDN:              "some.fqdn",
		LocalFQDN:         "local.fqdn",
		SignalingAddress:  "0.0.0.0:8080",
		SignalingInsecure: true,
	}

	proto, err := CloudConfigToProto(&in)
	test.That(t, err, test.ShouldBeNil)
	out, err := CloudConfigFromProto(proto)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, *out, test.ShouldResemble, in)
}
