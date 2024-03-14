package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	pb "go.viam.com/api/app/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/config/testutils"
	"go.viam.com/rdk/logging"
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

		fakeServer, err := testutils.NewFakeCloudServer(context.Background(), logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, fakeServer.Shutdown(), test.ShouldBeNil)
		}()

		appAddress := fmt.Sprintf("http://%s", fakeServer.Addr().String())
		expectedCloud := &Cloud{
			ManagedBy:        "acme",
			SignalingAddress: "abc",
			ID:               robotPartID,
			Secret:           secret,
			FQDN:             "fqdn",
			LocalFQDN:        "localFqdn",
			TLSCertificate:   "cert",
			TLSPrivateKey:    "key",
			RefreshInterval:  time.Duration(10000000000),
			LocationSecrets:  []LocationSecret{},
			AppAddress:       appAddress,
			LocationID:       "the-location",
			PrimaryOrgID:     "the-primary-org",
		}
		cloudConfProto, err := CloudConfigToProto(expectedCloud)
		test.That(t, err, test.ShouldBeNil)
		protoConfig := &pb.RobotConfig{Cloud: cloudConfProto}
		protoCertificate := &pb.CertificateResponse{
			TlsCertificate: "cert",
			TlsPrivateKey:  "key",
		}

		fakeServer.StoreDeviceConfig(robotPartID, protoConfig, protoCertificate)
		defer fakeServer.Clear()

		cfgText := fmt.Sprintf(`{"cloud":{"id":%q,"app_address":%q,"secret":%q}}`, robotPartID, appAddress, secret)
		gotCfg, err := FromReader(ctx, "", strings.NewReader(cfgText), logger)
		defer clearCache(robotPartID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotCfg.Cloud, test.ShouldResemble, expectedCloud)

		cachedCfg, err := readFromCache(robotPartID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cachedCfg.Cloud, test.ShouldResemble, expectedCloud)
	})

	// t.Run("offline", func(t *testing.T) {
	// 	setupClearCache(t)
	// 	newOfflineTestReader := func(
	// 		ctx context.Context,
	// 		cloud *Cloud,
	// 		logger logging.Logger,
	// 	) (*configReader, func() error) {
	// 		return &configReader{nil}, func() error { return nil }
	// 	}
	//
	// 	cloud := &Cloud{
	// 		ManagedBy:        "acme",
	// 		SignalingAddress: "abc",
	// 		ID:               robotPartID,
	// 		Secret:           "ghi",
	// 		FQDN:             "fqdn",
	// 		LocalFQDN:        "localFqdn",
	// 		TLSCertificate:   "cert",
	// 		TLSPrivateKey:    "key",
	// 		AppAddress:       "https://app.viam.dev:443",
	// 		LocationID:       "the-location",
	// 		PrimaryOrgID:     "the-primary-org",
	// 		LocationSecrets:  []LocationSecret{},
	// 	}
	// 	cfg := &Config{Cloud: cloud}
	//
	// 	// store our config to the cloud
	// 	err := storeToCache(cfg.Cloud.ID, cfg)
	// 	test.That(t, err, test.ShouldBeNil)
	// 	defer clearCache(robotPartID)
	//
	// 	cfgText := fmt.Sprintf(`{"cloud":{"id":%q,"secret":"ghi"}}`, robotPartID)
	// 	gotCfg, err := fromReader(ctx, "", strings.NewReader(cfgText), logger, newOfflineTestReader)
	//
	// 	expectedCloud := &Cloud{
	// 		ManagedBy:        "acme",
	// 		SignalingAddress: "abc",
	// 		ID:               robotPartID,
	// 		Secret:           "ghi",
	// 		FQDN:             "fqdn",
	// 		LocalFQDN:        "localFqdn",
	// 		TLSCertificate:   "cert",
	// 		TLSPrivateKey:    "key",
	// 		RefreshInterval:  time.Duration(10000000000),
	// 		LocationSecrets:  []LocationSecret{},
	// 	}
	// 	test.That(t, gotCfg.Cloud, test.ShouldResemble, expectedCloud)
	//
	// 	// TODO: why isn't this included in the result that comes from `fromReader`
	// 	expectedCloud.LocationID = "the-location"
	// 	expectedCloud.PrimaryOrgID = "the-primary-org"
	// 	expectedCloud.AppAddress = "https://app.viam.dev:443"
	// 	cachedCfg, err := readFromCache(robotPartID)
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, cachedCfg.Cloud, test.ShouldResemble, expectedCloud)
	// })
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
	}
	cfg.Cloud = cloud

	// store our config to the cloud
	err = storeToCache(cfg.Cloud.ID, cfg)
	test.That(t, err, test.ShouldBeNil)

	// read config from cloud, confirm consistency
	cloudCfg, err := readFromCloud(ctx, cfg, nil, true, false, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg, test.ShouldResemble, cfg)

	// Modify our config
	newRemote := Remote{Name: "test", Address: "foo"}
	cfg.Remotes = append(cfg.Remotes, newRemote)

	// read config from cloud again, confirm that the cached config differs from cfg
	cloudCfg2, err := readFromCloud(ctx, cfg, nil, true, false, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg2, test.ShouldNotResemble, cfg)

	// store the updated config to the cloud
	err = storeToCache(cfg.Cloud.ID, cfg)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, cfg.Ensure(true, logger), test.ShouldBeNil)

	// read updated cloud config, confirm that it now matches our updated cfg
	cloudCfg3, err := readFromCloud(ctx, cfg, nil, true, false, logger)
	test.That(t, err, test.ShouldBeNil)
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

		err = storeToCache(robotPartID, cfg)
		test.That(t, err, test.ShouldBeNil)

		tls := tlsConfig{}
		err = tls.readFromCache(robotPartID, logger)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("invalid cached TLS", func(t *testing.T) {
		defer clearCache(robotPartID)
		cloud := &Cloud{
			TLSPrivateKey: "key",
		}
		cfg.Cloud = cloud

		err = storeToCache(robotPartID, cfg)
		test.That(t, err, test.ShouldBeNil)

		tls := tlsConfig{}
		err = tls.readFromCache(robotPartID, logger)
		test.That(t, err, test.ShouldNotBeNil)

		_, err = readFromCache(robotPartID)
		test.That(t, os.IsNotExist(err), test.ShouldBeTrue)
	})

	t.Run("valid cached TLS", func(t *testing.T) {
		defer clearCache(robotPartID)
		cloud := &Cloud{
			TLSCertificate: "cert",
			TLSPrivateKey:  "key",
		}
		cfg.Cloud = cloud

		err = storeToCache(robotPartID, cfg)
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
