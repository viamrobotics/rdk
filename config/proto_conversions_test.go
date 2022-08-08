package config

import (
	"testing"

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

func TestRemoteConfigToProto(t *testing.T) {
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
