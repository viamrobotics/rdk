package config

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	pb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config/testutils"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

func TestFromReader(t *testing.T) {
	const (
		robotPartID = "forCachingTest"
		secret      = testutils.FakeCredentialPayLoad
	)
	var (
		logger = logging.NewTestLogger(t)
		ctx    = context.Background()
	)

	// clear cache
	setupClearCache := func(t *testing.T) {
		t.Helper()
		clearCache(robotPartID)
		_, err := readFromCache(robotPartID)
		test.That(t, os.IsNotExist(err), test.ShouldBeTrue)
	}

	t.Run("online", func(t *testing.T) {
		setupClearCache(t)

		fakeServer, cleanup := testutils.NewFakeCloudServer(t, ctx, logger)
		defer cleanup()

		cloudResponse := &Cloud{
			ManagedBy:        "acme",
			SignalingAddress: "abc",
			ID:               robotPartID,
			Secret:           secret,
			FQDN:             "fqdn",
			LocalFQDN:        "localFqdn",
			LocationSecrets:  []LocationSecret{},
			LocationID:       "the-location",
			PrimaryOrgID:     "the-primary-org",
			MachineID:        "the-machine",
		}
		certProto := &pb.CertificateResponse{
			TlsCertificate: "cert",
			TlsPrivateKey:  "key",
		}

		cloudConfProto, err := CloudConfigToProto(cloudResponse)
		test.That(t, err, test.ShouldBeNil)
		protoConfig := &pb.RobotConfig{Cloud: cloudConfProto}
		fakeServer.StoreDeviceConfig(robotPartID, protoConfig, certProto)

		appAddress := fmt.Sprintf("http://%s", fakeServer.Addr().String())
		appConn, err := grpc.NewAppConn(ctx, appAddress, robotPartID, cloudResponse.GetCloudCredsDialOpt(), logger)
		test.That(t, err, test.ShouldBeNil)
		defer appConn.Close()
		cfgText := fmt.Sprintf(`{"cloud":{"id":%q,"app_address":%q,"secret":%q}}`, robotPartID, appAddress, secret)
		gotCfg, err := FromReader(ctx, "", strings.NewReader(cfgText), logger, appConn)
		test.That(t, err, test.ShouldBeNil)

		expectedCloud := *cloudResponse
		expectedCloud.AppAddress = appAddress
		expectedCloud.TLSCertificate = certProto.TlsCertificate
		expectedCloud.TLSPrivateKey = certProto.TlsPrivateKey
		expectedCloud.RefreshInterval = time.Duration(10000000000)
		test.That(t, gotCfg.Cloud, test.ShouldResemble, &expectedCloud)

		test.That(t, gotCfg.StoreToCache(), test.ShouldBeNil)
		defer clearCache(robotPartID)
		cachedCfg, err := readFromCache(robotPartID)
		test.That(t, err, test.ShouldBeNil)
		expectedCloud.AppAddress = ""
		test.That(t, cachedCfg.Cloud, test.ShouldResemble, &expectedCloud)
	})

	t.Run("offline with cached config", func(t *testing.T) {
		setupClearCache(t)

		cachedCloud := &Cloud{
			ManagedBy:        "acme",
			SignalingAddress: "abc",
			ID:               robotPartID,
			Secret:           secret,
			FQDN:             "fqdn",
			LocalFQDN:        "localFqdn",
			TLSCertificate:   "cert",
			TLSPrivateKey:    "key",
			LocationID:       "the-location",
			PrimaryOrgID:     "the-primary-org",
			MachineID:        "the-machine",
		}
		cachedConf := &Config{Cloud: cachedCloud}

		cfgToCache := &Config{Cloud: &Cloud{ID: robotPartID}}
		cfgToCache.SetToCache(cachedConf)
		err := cfgToCache.StoreToCache()
		test.That(t, err, test.ShouldBeNil)
		defer clearCache(robotPartID)

		fakeServer, cleanup := testutils.NewFakeCloudServer(t, ctx, logger)
		defer cleanup()
		fakeServer.FailOnConfigAndCertsWith(context.DeadlineExceeded)
		fakeServer.StoreDeviceConfig(robotPartID, nil, nil)

		appAddress := fmt.Sprintf("http://%s", fakeServer.Addr().String())
		appConn, err := grpc.NewAppConn(ctx, appAddress, robotPartID, cachedCloud.GetCloudCredsDialOpt(), logger)
		test.That(t, err, test.ShouldBeNil)
		defer appConn.Close()
		cfgText := fmt.Sprintf(`{"cloud":{"id":%q,"app_address":%q,"secret":%q}}`, robotPartID, appAddress, secret)
		gotCfg, err := FromReader(ctx, "", strings.NewReader(cfgText), logger, appConn)
		test.That(t, err, test.ShouldBeNil)

		expectedCloud := *cachedCloud
		expectedCloud.AppAddress = appAddress
		expectedCloud.TLSCertificate = "cert"
		expectedCloud.TLSPrivateKey = "key"
		expectedCloud.RefreshInterval = time.Duration(10000000000)
		test.That(t, gotCfg.Cloud, test.ShouldResemble, &expectedCloud)
	})

	t.Run("online with insecure signaling", func(t *testing.T) {
		setupClearCache(t)

		fakeServer, cleanup := testutils.NewFakeCloudServer(t, ctx, logger)
		defer cleanup()

		cloudResponse := &Cloud{
			ManagedBy:         "acme",
			SignalingAddress:  "abc",
			SignalingInsecure: true,
			ID:                robotPartID,
			Secret:            secret,
			FQDN:              "fqdn",
			LocalFQDN:         "localFqdn",
			LocationSecrets:   []LocationSecret{},
			LocationID:        "the-location",
			PrimaryOrgID:      "the-primary-org",
			MachineID:         "the-machine",
		}
		certProto := &pb.CertificateResponse{}

		cloudConfProto, err := CloudConfigToProto(cloudResponse)
		test.That(t, err, test.ShouldBeNil)
		protoConfig := &pb.RobotConfig{Cloud: cloudConfProto}
		fakeServer.StoreDeviceConfig(robotPartID, protoConfig, certProto)

		appAddress := fmt.Sprintf("http://%s", fakeServer.Addr().String())
		appConn, err := grpc.NewAppConn(ctx, appAddress, robotPartID, cloudResponse.GetCloudCredsDialOpt(), logger)
		test.That(t, err, test.ShouldBeNil)
		defer appConn.Close()
		cfgText := fmt.Sprintf(`{"cloud":{"id":%q,"app_address":%q,"secret":%q}}`, robotPartID, appAddress, secret)
		gotCfg, err := FromReader(ctx, "", strings.NewReader(cfgText), logger, appConn)
		test.That(t, err, test.ShouldBeNil)

		expectedCloud := *cloudResponse
		expectedCloud.AppAddress = appAddress
		expectedCloud.RefreshInterval = time.Duration(10000000000)
		test.That(t, gotCfg.Cloud, test.ShouldResemble, &expectedCloud)

		err = gotCfg.StoreToCache()
		defer clearCache(robotPartID)
		test.That(t, err, test.ShouldBeNil)
		cachedCfg, err := readFromCache(robotPartID)
		test.That(t, err, test.ShouldBeNil)
		expectedCloud.AppAddress = ""
		test.That(t, cachedCfg.Cloud, test.ShouldResemble, &expectedCloud)
	})
}

// TestGetFromCloudOrCacheErrorClassification verifies that when the cloud config endpoint fails
// and we fall back to a cached config, a connectivity error is logged quietly (Warn) while a
// config rejection from the cloud is surfaced loudly (Error) so it is not silently hidden.
func TestGetFromCloudOrCacheErrorClassification(t *testing.T) {
	const (
		robotPartID = "forCachingTest"
		secret      = testutils.FakeCredentialPayLoad
	)
	ctx := context.Background()

	// Seed the cache so the fallback path is exercised in every case.
	setupCache := func(t *testing.T) {
		t.Helper()
		clearCache(robotPartID)
		cachedConf := &Config{Cloud: &Cloud{ID: robotPartID, Secret: secret, FQDN: "fqdn"}}
		cfgToCache := &Config{Cloud: &Cloud{ID: robotPartID}}
		cfgToCache.SetToCache(cachedConf)
		test.That(t, cfgToCache.StoreToCache(), test.ShouldBeNil)
	}

	newAppConn := func(t *testing.T, failWith error) (*Cloud, rpc.ClientConn, func()) {
		t.Helper()
		logger := logging.NewTestLogger(t)
		fakeServer, cleanup := testutils.NewFakeCloudServer(t, ctx, logger)
		fakeServer.FailOnConfigAndCertsWith(failWith)
		fakeServer.StoreDeviceConfig(robotPartID, nil, nil)

		appAddress := fmt.Sprintf("http://%s", fakeServer.Addr().String())
		cloudCfg := &Cloud{ID: robotPartID, Secret: secret, AppAddress: appAddress}
		appConn, err := grpc.NewAppConn(ctx, appAddress, robotPartID, cloudCfg.GetCloudCredsDialOpt(), logger)
		test.That(t, err, test.ShouldBeNil)
		return cloudCfg, appConn, func() {
			test.That(t, appConn.Close(), test.ShouldBeNil)
			cleanup()
		}
	}

	t.Run("connectivity error is logged quietly and falls back to cache", func(t *testing.T) {
		setupCache(t)
		defer clearCache(robotPartID)

		cloudCfg, appConn, cleanup := newAppConn(t, status.Error(codes.Unavailable, "cloud is down"))
		defer cleanup()

		logger, logs := logging.NewObservedTestLogger(t)
		cfg, cached, err := getFromCloudOrCache(ctx, cloudCfg, true, logger, appConn)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cached, test.ShouldBeTrue)
		test.That(t, cfg, test.ShouldNotBeNil)

		test.That(t, logs.FilterMessageSnippet("unable to get cloud config; using cached version").Len(), test.ShouldEqual, 1)
		test.That(t, logs.FilterMessageSnippet("cloud rejected this robot's config").Len(), test.ShouldEqual, 0)
	})

	t.Run("rejected config is surfaced loudly and falls back to cache", func(t *testing.T) {
		setupCache(t)
		defer clearCache(robotPartID)

		// codes.Unknown is what the real config conversion failure surfaces (a plain error returned by the
		// cloud config endpoint).
		cloudCfg, appConn, cleanup := newAppConn(t, status.Error(codes.Unknown, "OrientationVectorDegrees has a normal of 0"))
		defer cleanup()

		logger, logs := logging.NewObservedTestLogger(t)
		cfg, cached, err := getFromCloudOrCache(ctx, cloudCfg, true, logger, appConn)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cached, test.ShouldBeTrue)
		test.That(t, cfg, test.ShouldNotBeNil)

		rejected := logs.FilterMessageSnippet("cloud rejected this robot's config")
		test.That(t, rejected.Len(), test.ShouldEqual, 1)
		test.That(t, rejected.All()[0].Level, test.ShouldEqual, zapcore.ErrorLevel)
		test.That(t, logs.FilterMessageSnippet("unable to get cloud config; using cached version").Len(), test.ShouldEqual, 0)
	})

	t.Run("rejected config with no cache returns a clear error", func(t *testing.T) {
		clearCache(robotPartID)

		cloudCfg, appConn, cleanup := newAppConn(t, status.Error(codes.Unknown, "OrientationVectorDegrees has a normal of 0"))
		defer cleanup()

		logger := logging.NewTestLogger(t)
		_, _, err := getFromCloudOrCache(ctx, cloudCfg, true, logger, appConn)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "cloud rejected this robot's config and no cached config exists")
		test.That(t, err.Error(), test.ShouldContainSubstring, "OrientationVectorDegrees has a normal of 0")
	})
}

func TestStoreToCache(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cfg, err := FromReader(ctx, "", strings.NewReader(`{}`), logger, nil)

	test.That(t, err, test.ShouldBeNil)

	cloud := &Cloud{
		ManagedBy:        "acme",
		SignalingAddress: "abc",
		ID:               "forCachingTest",
		Secret:           "ghi",
		FQDN:             "fqdn",
		LocalFQDN:        "localFqdn",
		TLSCertificate:   "cert",
		TLSPrivateKey:    "key",
		AppAddress:       "https://app.viam.dev:443",
		LocationID:       "the-location",
		PrimaryOrgID:     "the-primary-org",
		MachineID:        "the-machine",
	}
	cfg.Cloud = cloud

	appConn, err := grpc.NewAppConn(ctx, cloud.AppAddress, cloud.ID, cfg.Cloud.GetCloudCredsDialOpt(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer appConn.Close()

	// errors if no unprocessed config to cache
	cfgToCache := &Config{Cloud: &Cloud{ID: "forCachingTest"}}
	err = cfgToCache.StoreToCache()
	test.That(t, err.Error(), test.ShouldContainSubstring, "no unprocessed config to cache")

	// store our config to the cache
	cfgToCache.SetToCache(cfg)
	err = cfgToCache.StoreToCache()
	test.That(t, err, test.ShouldBeNil)

	// read config from cloud, confirm consistency
	cloudCfg, err := readFromCloud(ctx, cfg, nil, true, false, logger, appConn)
	test.That(t, err, test.ShouldBeNil)
	cloudCfg.toCache = nil
	test.That(t, cloudCfg, test.ShouldResemble, cfg)

	// Modify our config
	newRemote := Remote{Name: "test", Address: "foo"}
	cfg.Remotes = append(cfg.Remotes, newRemote)

	// read config from cloud again, confirm that the cached config differs from cfg
	cloudCfg2, err := readFromCloud(ctx, cfg, nil, true, false, logger, appConn)
	test.That(t, err, test.ShouldBeNil)
	cloudCfg2.toCache = nil
	test.That(t, cloudCfg2, test.ShouldNotResemble, cfgToCache)

	// store the updated config to the cloud
	cfgToCache.SetToCache(cfg)
	err = cfgToCache.StoreToCache()
	test.That(t, err, test.ShouldBeNil)

	test.That(t, cfg.Ensure(true, logger), test.ShouldBeNil)

	// read updated cloud config, confirm that it now matches our updated cfg
	cloudCfg3, err := readFromCloud(ctx, cfg, nil, true, false, logger, appConn)
	test.That(t, err, test.ShouldBeNil)
	cloudCfg3.toCache = nil
	test.That(t, cloudCfg3, test.ShouldResemble, cfg)
}

func TestCacheInvalidation(t *testing.T) {
	id := uuid.New().String()
	// store invalid config in cache
	cachePath := getCloudCacheFilePath(id)
	err := os.WriteFile(cachePath, []byte("invalid-json"), 0o750)
	test.That(t, err, test.ShouldBeNil)

	// read from cache, should return parse error and remove file
	_, err = readFromCache(id)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot parse the cached config as json")

	// read from cache again and file should not exist
	_, err = readFromCache(id)
	test.That(t, os.IsNotExist(err), test.ShouldBeTrue)
}

func TestShouldCheckForCert(t *testing.T) {
	cloud1 := Cloud{
		ManagedBy:        "acme",
		SignalingAddress: "abc",
		ID:               "forCachingTest",
		Secret:           "ghi",
		FQDN:             "fqdn",
		LocalFQDN:        "localFqdn",
		TLSCertificate:   "cert",
		TLSPrivateKey:    "key",
		LocationID:       "the-location",
		PrimaryOrgID:     "the-primary-org",
		MachineID:        "the-machine",
		LocationSecrets: []LocationSecret{
			{ID: "id1", Secret: "secret1"},
			{ID: "id2", Secret: "secret2"},
		},
	}
	cloud2 := cloud1
	test.That(t, shouldCheckForCert(&cloud1, &cloud2), test.ShouldBeFalse)

	cloud2.TLSCertificate = "abc"
	test.That(t, shouldCheckForCert(&cloud1, &cloud2), test.ShouldBeFalse)

	cloud2 = cloud1
	cloud2.LocationSecret = "something else"
	test.That(t, shouldCheckForCert(&cloud1, &cloud2), test.ShouldBeTrue)

	cloud2 = cloud1
	cloud2.LocationSecrets = []LocationSecret{
		{ID: "id1", Secret: "secret1"},
		{ID: "id2", Secret: "secret3"},
	}
	test.That(t, shouldCheckForCert(&cloud1, &cloud2), test.ShouldBeTrue)
}

func TestProcessConfig(t *testing.T) {
	logger := logging.NewTestLogger(t)
	unprocessedConfig := Config{
		ConfigFilePath: "path",
	}

	cfg, err := processConfig(&unprocessedConfig, true, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *cfg, test.ShouldResemble, unprocessedConfig)
}

func TestReadTLSFromCache(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cfg, err := FromReader(ctx, "", strings.NewReader(`{}`), logger, nil)
	test.That(t, err, test.ShouldBeNil)

	robotPartID := "forCachingTest"
	t.Run("no cached config", func(t *testing.T) {
		clearCache(robotPartID)
		test.That(t, err, test.ShouldBeNil)

		tls := tlsConfig{}
		err = tls.readFromCache(robotPartID, logger)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("cache config without cloud", func(t *testing.T) {
		defer clearCache(robotPartID)
		cfg.Cloud = nil

		cfgToCache := &Config{Cloud: &Cloud{ID: robotPartID}}
		cfgToCache.SetToCache(cfg)
		err = cfgToCache.StoreToCache()
		test.That(t, err, test.ShouldBeNil)

		tls := tlsConfig{}
		err = tls.readFromCache(robotPartID, logger)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("invalid cached TLS", func(t *testing.T) {
		defer clearCache(robotPartID)
		cloud := &Cloud{
			ID:            robotPartID,
			TLSPrivateKey: "key",
		}
		cfg.Cloud = cloud

		cfgToCache := &Config{Cloud: &Cloud{ID: robotPartID}}
		cfgToCache.SetToCache(cfg)
		err = cfgToCache.StoreToCache()
		test.That(t, err, test.ShouldBeNil)

		tls := tlsConfig{}
		err = tls.readFromCache(robotPartID, logger)
		test.That(t, err, test.ShouldNotBeNil)

		_, err = readFromCache(robotPartID)
		test.That(t, errors.Is(err, fs.ErrNotExist), test.ShouldBeTrue)
	})

	t.Run("invalid cached TLS but insecure signaling", func(t *testing.T) {
		defer clearCache(robotPartID)
		cloud := &Cloud{
			ID:                robotPartID,
			TLSPrivateKey:     "key",
			SignalingInsecure: true,
		}
		cfg.Cloud = cloud

		cfgToCache := &Config{Cloud: &Cloud{ID: robotPartID}}
		cfgToCache.SetToCache(cfg)
		err = cfgToCache.StoreToCache()
		test.That(t, err, test.ShouldBeNil)

		tls := tlsConfig{}
		err = tls.readFromCache(robotPartID, logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = readFromCache(robotPartID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("valid cached TLS", func(t *testing.T) {
		defer clearCache(robotPartID)
		cloud := &Cloud{
			ID:             robotPartID,
			TLSCertificate: "cert",
			TLSPrivateKey:  "key",
		}
		cfg.Cloud = cloud

		cfgToCache := &Config{Cloud: &Cloud{ID: robotPartID}}
		cfgToCache.SetToCache(cfg)
		err = cfgToCache.StoreToCache()
		test.That(t, err, test.ShouldBeNil)

		// the config is missing several fields required to start the robot, but this
		// should not prevent us from reading TLS information from it.
		_, err = processConfigFromCloud(cfg, logger)
		test.That(t, err, test.ShouldNotBeNil)
		tls := tlsConfig{}
		err = tls.readFromCache(robotPartID, logger)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestAdditionalModuleEnvVars(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		expected := map[string]string{}
		observed := additionalModuleEnvVars(nil, AuthConfig{}, TracingConfig{})
		test.That(t, observed, test.ShouldResemble, expected)
	})

	cloud1 := Cloud{
		ID:           "test",
		LocationID:   "the-location",
		PrimaryOrgID: "the-primary-org",
		MachineID:    "the-machine",
	}
	t.Run("cloud", func(t *testing.T) {
		expected := map[string]string{
			utils.MachinePartIDEnvVar: cloud1.ID,
			utils.MachineIDEnvVar:     cloud1.MachineID,
			utils.MachineFQDNEnvVar:   cloud1.FQDN,
			utils.PrimaryOrgIDEnvVar:  cloud1.PrimaryOrgID,
			utils.LocationIDEnvVar:    cloud1.LocationID,
		}
		observed := additionalModuleEnvVars(&cloud1, AuthConfig{}, TracingConfig{})
		test.That(t, observed, test.ShouldResemble, expected)
	})

	authWithExternalCreds := AuthConfig{
		Handlers: []AuthHandlerConfig{{Type: rpc.CredentialsTypeExternal}},
	}

	t.Run("auth with external creds", func(t *testing.T) {
		expected := map[string]string{}
		observed := additionalModuleEnvVars(nil, authWithExternalCreds, TracingConfig{})
		test.That(t, observed, test.ShouldResemble, expected)
	})
	apiKeyID := "abc"
	apiKey := "def"
	authWithAPIKeyCreds := AuthConfig{
		Handlers: []AuthHandlerConfig{{Type: rpc.CredentialsTypeAPIKey, Config: utils.AttributeMap{
			apiKeyID: apiKey,
			"keys":   []string{apiKeyID},
		}}},
	}

	t.Run("auth with api key creds", func(t *testing.T) {
		expected := map[string]string{
			utils.APIKeyEnvVar:   apiKey,
			utils.APIKeyIDEnvVar: apiKeyID,
		}
		observed := additionalModuleEnvVars(nil, authWithAPIKeyCreds, TracingConfig{})
		test.That(t, observed, test.ShouldResemble, expected)
	})

	apiKeyID2 := "uvw"
	apiKey2 := "xyz"
	order1 := AuthConfig{
		Handlers: []AuthHandlerConfig{{Type: rpc.CredentialsTypeAPIKey, Config: utils.AttributeMap{
			apiKeyID:  apiKey,
			apiKeyID2: apiKey2,
			"keys":    []string{apiKeyID, apiKeyID2},
		}}},
	}
	order2 := AuthConfig{
		Handlers: []AuthHandlerConfig{{Type: rpc.CredentialsTypeAPIKey, Config: utils.AttributeMap{
			apiKeyID2: apiKey2,
			apiKeyID:  apiKey,
			"keys":    []string{apiKeyID, apiKeyID2},
		}}},
	}

	t.Run("auth with keys in different order are stable", func(t *testing.T) {
		expected := map[string]string{
			utils.APIKeyEnvVar:   apiKey,
			utils.APIKeyIDEnvVar: apiKeyID,
		}
		observed := additionalModuleEnvVars(nil, order1, TracingConfig{})
		test.That(t, observed, test.ShouldResemble, expected)

		observed = additionalModuleEnvVars(nil, order2, TracingConfig{})
		test.That(t, observed, test.ShouldResemble, expected)
	})

	t.Run("full", func(t *testing.T) {
		expected := map[string]string{
			utils.MachineFQDNEnvVar:   cloud1.FQDN,
			utils.MachinePartIDEnvVar: cloud1.ID,
			utils.MachineIDEnvVar:     cloud1.MachineID,
			utils.PrimaryOrgIDEnvVar:  cloud1.PrimaryOrgID,
			utils.LocationIDEnvVar:    cloud1.LocationID,
			utils.APIKeyEnvVar:        apiKey,
			utils.APIKeyIDEnvVar:      apiKeyID,
		}
		observed := additionalModuleEnvVars(&cloud1, authWithAPIKeyCreds, TracingConfig{})
		test.That(t, observed, test.ShouldResemble, expected)
	})
}
