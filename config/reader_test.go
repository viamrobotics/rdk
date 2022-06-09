package config

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

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
	cloudCfg, _, err := readFromCloud(ctx, cfg.Cloud, true, true, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg, test.ShouldResemble, cfg)

	// Modify our config
	newRemote := Remote{Name: "test", Address: "foo", Prefix: true}
	cfg.Remotes = append(cfg.Remotes, newRemote)

	// read config from cloud again, confirm that the cached config differs from cfg
	cloudCfg2, _, err := readFromCloud(ctx, cfg.Cloud, true, true, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg2, test.ShouldNotResemble, cfg)

	// store the updated config to the cloud
	err = storeToCache(cfg.Cloud.ID, cfg)
	test.That(t, err, test.ShouldBeNil)

	// read updated cloud config, confirm that it now matches our updated cfg
	cloudCfg3, _, err := readFromCloud(ctx, cfg.Cloud, true, true, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg3, test.ShouldResemble, cfg)
}

func taintAndStoreToCache(id string, cfg *Config) error {
	if err := os.MkdirAll(viamDotDir, 0o700); err != nil {
		return err
	}

	md, err := json.MarshalIndent(cfg, "", "  ")
	md = append(md, '?')
	if err != nil {
		return err
	}
	reader := bytes.NewReader(md)

	path := getCloudCacheFilePath(id)

	return artifact.AtomicStore(path, reader, id)
}

func TestDeleteTaintedConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cfg, err := FromReader(ctx, "", strings.NewReader(`{}`), logger)

	test.That(t, err, test.ShouldBeNil)

	cloud := &Cloud{
		ManagedBy:        "acme",
		SignalingAddress: "abc",
		ID:               "forBadCachingTest",
		Secret:           "ghi",
		FQDN:             "fqdn",
		LocalFQDN:        "localFqdn",
		TLSCertificate:   "cert",
		TLSPrivateKey:    "key",
	}
	cfg.Cloud = cloud

	// store our config to the cloud
	err = taintAndStoreToCache(cfg.Cloud.ID, cfg)
	test.That(t, err, test.ShouldBeNil)

	// read config from cloud, confirm consistency
	cloudCfg, _, err := readFromCloud(ctx, cfg.Cloud, true, true, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudCfg, test.ShouldResemble, cfg)

	// // Modify our config
	// newRemote := Remote{Name: "test", Address: "foo", Prefix: true}
	// cfg.Remotes = append(cfg.Remotes, newRemote)
	//
	// // read config from cloud again, confirm that the cached config differs from cfg
	// cloudCfg2, _, err := readFromCloud(ctx, cfg.Cloud, true, true, logger)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, cloudCfg2, test.ShouldNotResemble, cfg)
	//
	// // store the updated config to the cloud
	// err = storeToCache(cfg.Cloud.ID, cfg)
	// test.That(t, err, test.ShouldBeNil)
	//
	// // read updated cloud config, confirm that it now matches our updated cfg
	// cloudCfg3, _, err := readFromCloud(ctx, cfg.Cloud, true, true, logger)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, cloudCfg3, test.ShouldResemble, cfg)
}
