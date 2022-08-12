package config

import (
	"context"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/mock/gomock"
	mockapppb "go.viam.com/api/proto/viam/app/mock_v1"
	apppb "go.viam.com/api/proto/viam/app/v1"
	"go.viam.com/test"
)

func TestStoreToCache2(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mockapppb.NewMockRobotServiceClient(ctrl)
	client.EXPECT().Config(gomock.Any(), gomock.Any()).Return(&apppb.ConfigResponse{
		Config: &apppb.RobotConfig{
			Cloud: &apppb.CloudConfig{
				Id: "123",
			},
		},
	}, nil)

	res, err := client.Config(context.Background(), &apppb.ConfigRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)
}

func TestStoreToCache(t *testing.T) {
	logger := golog.NewTestLogger(t)
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
	}
	cfg.Cloud = cloud

	// store our config to the cloud
	err = storeToCache(cfg.Cloud.ID, cfg)
	test.That(t, err, test.ShouldBeNil)

	// read config from cloud, confirm consistency
	cloudCfg, _, err := readFromCloud(ctx, cfg, true, true, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg, test.ShouldResemble, cfg)

	// Modify our config
	newRemote := Remote{Name: "test", Address: "foo", Prefix: true}
	cfg.Remotes = append(cfg.Remotes, newRemote)

	// read config from cloud again, confirm that the cached config differs from cfg
	cloudCfg2, _, err := readFromCloud(ctx, cfg, true, true, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg2, test.ShouldNotResemble, cfg)

	// store the updated config to the cloud
	err = storeToCache(cfg.Cloud.ID, cfg)
	test.That(t, err, test.ShouldBeNil)

	// read updated cloud config, confirm that it now matches our updated cfg
	cloudCfg3, _, err := readFromCloud(ctx, cfg, true, true, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg3, test.ShouldResemble, cfg)
}

func TestProcessConfig(t *testing.T) {
	unprocessedConfig := Config{
		ConfigFilePath: "path",
	}

	cfg, err := processConfig(&unprocessedConfig, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *cfg, test.ShouldResemble, unprocessedConfig)
}
