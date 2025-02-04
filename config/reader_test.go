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
	pb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config/testutils"
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
		cfgText := fmt.Sprintf(`{"cloud":{"id":%q,"app_address":%q,"secret":%q}}`, robotPartID, appAddress, secret)
		gotCfg, err := FromReader(ctx, "", strings.NewReader(cfgText), logger)
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
		cfgText := fmt.Sprintf(`{"cloud":{"id":%q,"app_address":%q,"secret":%q}}`, robotPartID, appAddress, secret)
		gotCfg, err := FromReader(ctx, "", strings.NewReader(cfgText), logger)
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
		cfgText := fmt.Sprintf(`{"cloud":{"id":%q,"app_address":%q,"secret":%q}}`, robotPartID, appAddress, secret)
		gotCfg, err := FromReader(ctx, "", strings.NewReader(cfgText), logger)
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

func TestStoreToCache(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cfg, err := FromReader(ctx, "", strings.NewReader(`{}`), logger)

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

	// errors if no unprocessed config to cache
	cfgToCache := &Config{Cloud: &Cloud{ID: "forCachingTest"}}
	err = cfgToCache.StoreToCache()
	test.That(t, err.Error(), test.ShouldContainSubstring, "no unprocessed config to cache")

	// store our config to the cache
	cfgToCache.SetToCache(cfg)
	err = cfgToCache.StoreToCache()
	test.That(t, err, test.ShouldBeNil)

	// read config from cloud, confirm consistency
	cloudCfg, err := readFromCloud(ctx, cfg, nil, true, false, logger)
	test.That(t, err, test.ShouldBeNil)
	cloudCfg.toCache = nil
	test.That(t, cloudCfg, test.ShouldResemble, cfg)

	// Modify our config
	newRemote := Remote{Name: "test", Address: "foo"}
	cfg.Remotes = append(cfg.Remotes, newRemote)

	// read config from cloud again, confirm that the cached config differs from cfg
	cloudCfg2, err := readFromCloud(ctx, cfg, nil, true, false, logger)
	test.That(t, err, test.ShouldBeNil)
	cloudCfg2.toCache = nil
	test.That(t, cloudCfg2, test.ShouldNotResemble, cfgToCache)

	// store the updated config to the cloud
	cfgToCache.SetToCache(cfg)
	err = cfgToCache.StoreToCache()
	test.That(t, err, test.ShouldBeNil)

	test.That(t, cfg.Ensure(true, logger), test.ShouldBeNil)

	// read updated cloud config, confirm that it now matches our updated cfg
	cloudCfg3, err := readFromCloud(ctx, cfg, nil, true, false, logger)
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
	cfg, err := FromReader(ctx, "", strings.NewReader(`{}`), logger)
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
		observed := additionalModuleEnvVars(nil, AuthConfig{})
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
			utils.PrimaryOrgIDEnvVar:  cloud1.PrimaryOrgID,
			utils.LocationIDEnvVar:    cloud1.LocationID,
		}
		observed := additionalModuleEnvVars(&cloud1, AuthConfig{})
		test.That(t, observed, test.ShouldResemble, expected)
	})

	authWithExternalCreds := AuthConfig{
		Handlers: []AuthHandlerConfig{{Type: rpc.CredentialsTypeExternal}},
	}

	t.Run("auth with external creds", func(t *testing.T) {
		expected := map[string]string{}
		observed := additionalModuleEnvVars(nil, authWithExternalCreds)
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
		observed := additionalModuleEnvVars(nil, authWithAPIKeyCreds)
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
		observed := additionalModuleEnvVars(nil, order1)
		test.That(t, observed, test.ShouldResemble, expected)

		observed = additionalModuleEnvVars(nil, order2)
		test.That(t, observed, test.ShouldResemble, expected)
	})

	t.Run("full", func(t *testing.T) {
		expected := map[string]string{
			utils.MachinePartIDEnvVar: cloud1.ID,
			utils.MachineIDEnvVar:     cloud1.MachineID,
			utils.PrimaryOrgIDEnvVar:  cloud1.PrimaryOrgID,
			utils.LocationIDEnvVar:    cloud1.LocationID,
			utils.APIKeyEnvVar:        apiKey,
			utils.APIKeyIDEnvVar:      apiKeyID,
		}
		observed := additionalModuleEnvVars(&cloud1, authWithAPIKeyCreds)
		test.That(t, observed, test.ShouldResemble, expected)
	})
}
