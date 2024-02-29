package config

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/logging"
)

type InjectRobotServiceClient struct{}

func (c *InjectRobotServiceClient) Config(ctx context.Context, in *apppb.ConfigRequest, opts ...grpc.CallOption) (*apppb.ConfigResponse, error) {
	resp := &apppb.ConfigResponse{
		Config: &apppb.RobotConfig{
			Cloud: &apppb.CloudConfig{
				ManagedBy:        "acme",
				SignalingAddress: "abc",
				Id:               "forCachingTest",
				Secret:           "ghi",
				Fqdn:             "fqdn",
				LocalFqdn:        "localFqdn",
				LocationId:       "the-location",
				PrimaryOrgId:     "the-primary-org",
			},
		},
	}
	return resp, nil
}

func (c *InjectRobotServiceClient) Certificate(ctx context.Context, in *apppb.CertificateRequest, opts ...grpc.CallOption) (*apppb.CertificateResponse, error) {
	resp := &apppb.CertificateResponse{
		TlsCertificate: "cert",
		TlsPrivateKey:  "key",
	}
	return resp, nil
}

func (c *InjectRobotServiceClient) Log(ctx context.Context, in *apppb.LogRequest, opts ...grpc.CallOption) (*apppb.LogResponse, error) {
	return nil, errors.New("failed to get log")
}

func (c *InjectRobotServiceClient) NeedsRestart(ctx context.Context, in *apppb.NeedsRestartRequest, opts ...grpc.CallOption) (*apppb.NeedsRestartResponse, error) {
	return nil, errors.New("failed to get needs restart")
}

func createInjectRobotService(ctx context.Context, cloud *Cloud, logger logging.Logger) (cloudRobotService, func() error, error) {
	svc := cloudRobotService{&InjectRobotServiceClient{}}
	return svc, func() error { return nil }, nil
}

func TestFromReader(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cfgText := `{"cloud":{"id":"forCachingTest","secret":"ghi"}}`
	gotCfg, err := fromReader(ctx, "", strings.NewReader(cfgText), logger, createInjectRobotService)

	test.That(t, err, test.ShouldBeNil)

	expectedCloud := &Cloud{
		ManagedBy:        "acme",
		SignalingAddress: "abc",
		ID:               "forCachingTest",
		Secret:           "ghi",
		FQDN:             "fqdn",
		LocalFQDN:        "localFqdn",
		TLSCertificate:   "cert",
		TLSPrivateKey:    "key",
		RefreshInterval:  time.Duration(10000000000),
		LocationSecrets:  []LocationSecret{},
	}
	test.That(t, gotCfg.Cloud, test.ShouldResemble, expectedCloud)
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

	svc := cloudRobotService{client: &InjectRobotServiceClient{}}

	// read config from cloud, confirm consistency
	cloudCfg, err := svc.readFromCloud(ctx, cfg, nil, true, false, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg, test.ShouldResemble, cfg)

	// Modify our config
	newRemote := Remote{Name: "test", Address: "foo"}
	cfg.Remotes = append(cfg.Remotes, newRemote)

	// read config from cloud again, confirm that the cached config differs from cfg
	cloudCfg2, err := svc.readFromCloud(ctx, cfg, nil, true, false, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg2, test.ShouldNotResemble, cfg)

	// store the updated config to the cloud
	err = storeToCache(cfg.Cloud.ID, cfg)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, cfg.Ensure(true, logger), test.ShouldBeNil)

	// read updated cloud config, confirm that it now matches our updated cfg
	cloudCfg3, err := svc.readFromCloud(ctx, cfg, nil, true, false, logger)
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
