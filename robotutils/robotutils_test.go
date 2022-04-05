package robotutils

import (
	"crypto/tls"
	"errors"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	rutils "go.viam.com/rdk/utils"
)

func TestNewTLSConfig(t *testing.T) {
	for _, tc := range []struct {
		TestName     string
		Config       *config.Config
		HasTLSConfig bool
	}{
		{TestName: "no cloud", Config: &config.Config{}, HasTLSConfig: false},
		{TestName: "cloud but no cert", Config: &config.Config{Cloud: &config.Cloud{TLSCertificate: ""}}, HasTLSConfig: false},
		{TestName: "cloud and cert", Config: &config.Config{Cloud: &config.Cloud{TLSCertificate: "abc"}}, HasTLSConfig: true},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := NewTLSConfig(tc.Config)
			if tc.HasTLSConfig {
				test.That(t, observed.MinVersion, test.ShouldEqual, tls.VersionTLS12)
			} else {
				test.That(t, observed, test.ShouldResemble, &TLSConfig{})
			}
		})
	}
}

func TestUpdateCert(t *testing.T) {
	t.Run("cert update", func(t *testing.T) {
		cfg := &config.Config{
			Cloud: &config.Cloud{
				TLSCertificate: `-----BEGIN CERTIFICATE-----
MIIBCzCBtgIJAIuXZJ6ZiHraMA0GCSqGSIb3DQEBCwUAMA0xCzAJBgNVBAYTAnVz
MB4XDTIyMDQwNTE5MTMzNVoXDTIzMDQwNTE5MTMzNVowDTELMAkGA1UEBhMCdXMw
XDANBgkqhkiG9w0BAQEFAANLADBIAkEAyiHLgbZFf5UNAue0HAdQfv1Z15n8ldkI
bi4Owm5Iwb9IGGdkQNniEgveue536vV/ugAdt8ZxLuM1vzYFSApxXwIDAQABMA0G
CSqGSIb3DQEBCwUAA0EAOYH+xj8NuneL6w5D/FlW0+qUwBaS+/J3nL+PW1MQqjs8
1AHgPDxOtY7dUXK2E8SYia75JjtK9/FnpaFVHdQ9jQ==
-----END CERTIFICATE-----`,
				TLSPrivateKey: `-----BEGIN PRIVATE KEY-----
MIIBUwIBADANBgkqhkiG9w0BAQEFAASCAT0wggE5AgEAAkEAyiHLgbZFf5UNAue0
HAdQfv1Z15n8ldkIbi4Owm5Iwb9IGGdkQNniEgveue536vV/ugAdt8ZxLuM1vzYF
SApxXwIDAQABAkAEY412qI2DwqnAqWVIwoPl7fxYaRiJ7Gd5dPiPEjP0OPglB7eJ
VuSJeiPi3XSFXE9tw//Lpe2oOITF6OBCZURBAiEA7oZslGO+24+leOffb8PpceNm
EgHnAdibedkHD7ZprX8CIQDY8NASxuaEMa6nH7b9kkx/KaOo0/dOkW+sWb5PeIbs
IQIgOUd6p5/UY3F5cTFtjK9lTf4nssdWLDFSFM6zTWimtA0CIHwhFj2YN2/uaYvQ
1siyfDjKn41Lc5cuGmLYms8oHLNhAiBxeGqLlEyHdk+Trp99+nK+pFi4cj5NZSFh
ph2C/7IgjA==
-----END PRIVATE KEY-----`,
			},
		}
		cert, err := tls.X509KeyPair([]byte(cfg.Cloud.TLSCertificate), []byte(cfg.Cloud.TLSPrivateKey))
		test.That(t, err, test.ShouldBeNil)

		tlsCfg := &TLSConfig{}
		err = tlsCfg.UpdateCert(cfg)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, tlsCfg.tlsCert, test.ShouldResemble, &cert)
	})
	t.Run("cert error", func(t *testing.T) {
		cfg := &config.Config{Cloud: &config.Cloud{TLSCertificate: "abcd", TLSPrivateKey: "abcd"}}
		tlsCfg := &TLSConfig{}
		err := tlsCfg.UpdateCert(cfg)
		test.That(t, err, test.ShouldBeError, errors.New("tls: failed to find any PEM data in certificate input"))
	})
}

func TestProcessConfig(t *testing.T) {
	// no remote
	// nil cloud
	// cloud but nil tls
	// cloud and has tls

	// has remote
	// nil cloud
	// cloud but nil tls
	// cloud and has tls
	// managed by diff

	cloud := &config.Cloud{
		ManagedBy:        "acme",
		SignalingAddress: "abc",
		ID:               "def",
		Secret:           "ghi",
		TLSCertificate:   "",
	}
	cloudWTLS := &config.Cloud{
		ManagedBy:        "acme",
		SignalingAddress: "abc",
		ID:               "def",
		Secret:           "ghi",
		TLSCertificate: `-----BEGIN CERTIFICATE-----
MIIBCzCBtgIJAIuXZJ6ZiHraMA0GCSqGSIb3DQEBCwUAMA0xCzAJBgNVBAYTAnVz
MB4XDTIyMDQwNTE5MTMzNVoXDTIzMDQwNTE5MTMzNVowDTELMAkGA1UEBhMCdXMw
XDANBgkqhkiG9w0BAQEFAANLADBIAkEAyiHLgbZFf5UNAue0HAdQfv1Z15n8ldkI
bi4Owm5Iwb9IGGdkQNniEgveue536vV/ugAdt8ZxLuM1vzYFSApxXwIDAQABMA0G
CSqGSIb3DQEBCwUAA0EAOYH+xj8NuneL6w5D/FlW0+qUwBaS+/J3nL+PW1MQqjs8
1AHgPDxOtY7dUXK2E8SYia75JjtK9/FnpaFVHdQ9jQ==
-----END CERTIFICATE-----`,
		TLSPrivateKey: `-----BEGIN PRIVATE KEY-----
MIIBUwIBADANBgkqhkiG9w0BAQEFAASCAT0wggE5AgEAAkEAyiHLgbZFf5UNAue0
HAdQfv1Z15n8ldkIbi4Owm5Iwb9IGGdkQNniEgveue536vV/ugAdt8ZxLuM1vzYF
SApxXwIDAQABAkAEY412qI2DwqnAqWVIwoPl7fxYaRiJ7Gd5dPiPEjP0OPglB7eJ
VuSJeiPi3XSFXE9tw//Lpe2oOITF6OBCZURBAiEA7oZslGO+24+leOffb8PpceNm
EgHnAdibedkHD7ZprX8CIQDY8NASxuaEMa6nH7b9kkx/KaOo0/dOkW+sWb5PeIbs
IQIgOUd6p5/UY3F5cTFtjK9lTf4nssdWLDFSFM6zTWimtA0CIHwhFj2YN2/uaYvQ
1siyfDjKn41Lc5cuGmLYms8oHLNhAiBxeGqLlEyHdk+Trp99+nK+pFi4cj5NZSFh
ph2C/7IgjA==
-----END PRIVATE KEY-----`,
	}

	remoteAuth := config.RemoteAuth{
		Credentials:            &rpc.Credentials{rutils.CredentialsTypeRobotSecret, "xyz"},
		Managed:                false,
		SignalingServerAddress: "xyz",
		SignalingAuthEntity:    "xyz",
	}
	remote := config.Remote{
		ManagedBy: "acme",
		Auth:      remoteAuth,
	}
	remoteDiffManager := config.Remote{
		ManagedBy: "viam",
		Auth:      remoteAuth,
	}
	noCloudCfg := &config.Config{Remotes: []config.Remote{}}
	cloudCfg := &config.Config{Cloud: cloud, Remotes: []config.Remote{}}
	cloudWTLSCfg := &config.Config{Cloud: cloudWTLS, Remotes: []config.Remote{}}
	remotesNoCloudCfg := &config.Config{Remotes: []config.Remote{remote, remoteDiffManager}}
	remotesCloudCfg := &config.Config{Cloud: cloud, Remotes: []config.Remote{remote, remoteDiffManager}}
	remotesCloudWTLSCfg := &config.Config{Cloud: cloudWTLS, Remotes: []config.Remote{remote, remoteDiffManager}}

	expectedRemoteAuthNoCloud := remoteAuth
	expectedRemoteAuthNoCloud.SignalingCreds = expectedRemoteAuthNoCloud.Credentials

	expectedRemoteAuthCloud := remoteAuth
	expectedRemoteAuthCloud.Managed = true
	expectedRemoteAuthCloud.SignalingServerAddress = cloud.SignalingAddress
	expectedRemoteAuthCloud.SignalingAuthEntity = cloud.ID
	expectedRemoteAuthCloud.SignalingCreds = &rpc.Credentials{rutils.CredentialsTypeRobotSecret, cloud.Secret}

	expectedRemoteNoCloud := remote
	expectedRemoteNoCloud.Auth = expectedRemoteAuthNoCloud
	expectedRemoteCloud := remote
	expectedRemoteCloud.Auth = expectedRemoteAuthCloud

	expectedRemoteDiffManagerNoCloud := remoteDiffManager
	expectedRemoteDiffManagerNoCloud.Auth = expectedRemoteAuthNoCloud

	tlsCfg := &TLSConfig{}
	err := tlsCfg.UpdateCert(cloudWTLSCfg)
	test.That(t, err, test.ShouldBeNil)

	expectedCloudWTLSCfg := &config.Config{Cloud: cloudWTLS, Remotes: []config.Remote{}}
	expectedCloudWTLSCfg.Network.TLSConfig = tlsCfg.Config

	expectedRemotesCloudWTLSCfg := &config.Config{Cloud: cloudWTLS, Remotes: []config.Remote{expectedRemoteCloud, remoteDiffManager}}
	expectedRemotesCloudWTLSCfg.Network.TLSConfig = tlsCfg.Config

	for _, tc := range []struct {
		TestName string
		Config   *config.Config
		Expected *config.Config
	}{
		{TestName: "no cloud", Config: noCloudCfg, Expected: noCloudCfg},
		{TestName: "cloud but no cert", Config: cloudCfg, Expected: cloudCfg},
		{TestName: "cloud and cert", Config: cloudWTLSCfg, Expected: expectedCloudWTLSCfg},
		{
			TestName: "remotes no cloud",
			Config:   remotesNoCloudCfg,
			Expected: &config.Config{Remotes: []config.Remote{expectedRemoteNoCloud, expectedRemoteDiffManagerNoCloud}},
		},
		{
			TestName: "remotes cloud but no cert",
			Config:   remotesCloudCfg,
			Expected: &config.Config{Cloud: cloud, Remotes: []config.Remote{expectedRemoteCloud, remoteDiffManager}},
		},
		{TestName: "remotes cloud and cert", Config: remotesCloudWTLSCfg, Expected: expectedRemotesCloudWTLSCfg},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed, err := ProcessConfig(tc.Config, &TLSConfig{})
			test.That(t, err, test.ShouldBeNil)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}

	t.Run("cert error", func(t *testing.T) {
		cfg := &config.Config{Cloud: &config.Cloud{TLSCertificate: "abcd", TLSPrivateKey: "abcd"}}
		_, err := ProcessConfig(cfg, &TLSConfig{})
		test.That(t, err, test.ShouldBeError, errors.New("tls: failed to find any PEM data in certificate input"))
	})
}
