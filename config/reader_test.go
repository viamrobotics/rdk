package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"reflect"
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
// malformed config from the cloud is surfaced loudly (Error).
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
		cached := &Config{Cloud: &Cloud{ID: robotPartID, Secret: secret, FQDN: "fqdn"}}
		test.That(t, cached.SetToCache(cached), test.ShouldBeNil)
		test.That(t, cached.StoreToCache(), test.ShouldBeNil)
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
		cfg, cached, err := getFromCloudOrCache(ctx, cloudCfg.ID, true, logger, appConn)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cached, test.ShouldBeTrue)
		test.That(t, cfg, test.ShouldNotBeNil)

		// Same message for both cases; a transient failure is distinguished only by its Warn level.
		quiet := logs.FilterMessageSnippet("could not apply new cloud config; using cached version")
		test.That(t, quiet.Len(), test.ShouldEqual, 1)
		test.That(t, quiet.All()[0].Level, test.ShouldEqual, zapcore.WarnLevel)
	})

	t.Run("malformed config is surfaced loudly and falls back to cache", func(t *testing.T) {
		setupCache(t)
		defer clearCache(robotPartID)

		// codes.Unknown is what the real config conversion failure surfaces
		cloudCfg, appConn, cleanup := newAppConn(t, status.Error(codes.Unknown, "OrientationVectorDegrees has a normal of 0"))
		defer cleanup()

		logger, logs := logging.NewObservedTestLogger(t)
		cfg, cached, err := getFromCloudOrCache(ctx, cloudCfg.ID, true, logger, appConn)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cached, test.ShouldBeTrue)
		test.That(t, cfg, test.ShouldNotBeNil)

		// Same message as the transient case, but a malformed config is surfaced loudly at Error.
		loud := logs.FilterMessageSnippet("could not apply new cloud config; using cached version")
		test.That(t, loud.Len(), test.ShouldEqual, 1)
		test.That(t, loud.All()[0].Level, test.ShouldEqual, zapcore.ErrorLevel)
	})

	t.Run("malformed config with no cache returns a clear error", func(t *testing.T) {
		clearCache(robotPartID)

		cloudCfg, appConn, cleanup := newAppConn(t, status.Error(codes.Unknown, "OrientationVectorDegrees has a normal of 0"))
		defer cleanup()

		logger := logging.NewTestLogger(t)
		_, _, err := getFromCloudOrCache(ctx, cloudCfg.ID, true, logger, appConn)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, IsMalformedConfigError(err), test.ShouldBeTrue)
		test.That(t, err.Error(), test.ShouldContainSubstring, "config was malformed")
		test.That(t, err.Error(), test.ShouldContainSubstring, "cached config does not exist")
		test.That(t, err.Error(), test.ShouldContainSubstring, "OrientationVectorDegrees has a normal of 0")
	})

	t.Run("connectivity error with no cache returns the original error and is not malformed", func(t *testing.T) {
		clearCache(robotPartID)

		cloudCfg, appConn, cleanup := newAppConn(t, status.Error(codes.Unavailable, "cloud is down"))
		defer cleanup()

		logger := logging.NewTestLogger(t)
		_, _, err := getFromCloudOrCache(ctx, cloudCfg.ID, true, logger, appConn)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, IsMalformedConfigError(err), test.ShouldBeFalse)
		test.That(t, err.Error(), test.ShouldContainSubstring, "cached config does not exist")
		test.That(t, err.Error(), test.ShouldNotContainSubstring, "config was malformed")
	})
}

// TestIsCloudConfigMalformed pins down the classification that decides whether a cloud config-fetch
// failure means the config is malformed or is a transient failure to reach the cloud.
func TestIsCloudConfigMalformed(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		malformed bool
	}{
		// Status codes the cloud returns when it cannot produce a usable config.
		{"unknown (real conversion failure)", status.Error(codes.Unknown, "boom"), true},
		{"invalid argument", status.Error(codes.InvalidArgument, "boom"), true},
		{"failed precondition", status.Error(codes.FailedPrecondition, "boom"), true},
		// Transient status codes should not be treated as if the config is malformed.
		{"unavailable", status.Error(codes.Unavailable, "boom"), false},
		{"deadline exceeded", status.Error(codes.DeadlineExceeded, "boom"), false},
		{"canceled", status.Error(codes.Canceled, "boom"), false},
		{"internal", status.Error(codes.Internal, "boom"), false},
		{"resource exhausted", status.Error(codes.ResourceExhausted, "boom"), false},
		{"unauthenticated", status.Error(codes.Unauthenticated, "boom"), false},
		{"permission denied", status.Error(codes.PermissionDenied, "boom"), false},
		{"not found", status.Error(codes.NotFound, "boom"), false},
		// Non-status errors never reached the cloud, so they are transient even though status.Code
		// reports Unknown for them. "not connected" is what the rpc layer returns when the app
		// connection was never established.
		{"not connected (no gRPC status)", errors.New("not connected"), false},
		{"bare context deadline", context.DeadlineExceeded, false},
		// Wrapping must not hide a real malformed-config error.
		{"wrapped malformed", errors.WithMessage(status.Error(codes.Unknown, "boom"), "fetching config"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			test.That(t, isCloudConfigMalformed(tc.err), test.ShouldEqual, tc.malformed)
		})
	}
}

// TestGetFromCloudGRPCProtoDecodeFailureIsMalformed verifies that a config the cloud serves but that
// this robot cannot decode from proto is surfaced as a malformed config.
func TestGetFromCloudGRPCProtoDecodeFailureIsMalformed(t *testing.T) {
	const robotPartID = "forProtoDecodeTest"
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	clearCache(robotPartID)
	defer clearCache(robotPartID)

	fakeServer, cleanup := testutils.NewFakeCloudServer(t, ctx, logger)
	defer cleanup()

	cloudResponse := &Cloud{ID: robotPartID, Secret: testutils.FakeCredentialPayLoad, FQDN: "fqdn", SignalingInsecure: true}
	cloudConfProto, err := CloudConfigToProto(cloudResponse)
	test.That(t, err, test.ShouldBeNil)

	// An auth handler with an unspecified credential type passes the wire but fails FromProto.
	protoConfig := &pb.RobotConfig{
		Cloud: cloudConfProto,
		Auth:  &pb.AuthConfig{Handlers: []*pb.AuthHandlerConfig{{Type: pb.CredentialsType_CREDENTIALS_TYPE_UNSPECIFIED}}},
	}
	fakeServer.StoreDeviceConfig(robotPartID, protoConfig, nil)

	appAddress := fmt.Sprintf("http://%s", fakeServer.Addr().String())
	cloudCfg := &Cloud{ID: robotPartID, Secret: testutils.FakeCredentialPayLoad, AppAddress: appAddress}
	appConn, err := grpc.NewAppConn(ctx, appAddress, robotPartID, cloudCfg.GetCloudCredsDialOpt(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer appConn.Close()

	cfg, err := getFromCloudGRPC(ctx, cloudCfg.ID, logger, appConn)
	test.That(t, cfg, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, IsMalformedConfigError(err), test.ShouldBeTrue)
	test.That(t, err.Error(), test.ShouldContainSubstring, "config was malformed")
	test.That(t, err.Error(), test.ShouldContainSubstring, "converting config from proto")
}

// TestReadFromCloudMarksUnprocessableConfigMalformed verifies that a config the cloud serves
// successfully but that this robot cannot process locally (here, a bind address with no port) is
// surfaced as a malformed config.
func TestReadFromCloudMarksUnprocessableConfigMalformed(t *testing.T) {
	const (
		robotPartID = "forUnprocessableTest"
		secret      = testutils.FakeCredentialPayLoad
	)
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	clearCache(robotPartID)
	defer clearCache(robotPartID)

	fakeServer, cleanup := testutils.NewFakeCloudServer(t, ctx, logger)
	defer cleanup()

	cloudResponse := &Cloud{ID: robotPartID, Secret: secret, FQDN: "fqdn", LocalFQDN: "localFqdn", SignalingInsecure: true}
	cloudConfProto, err := CloudConfigToProto(cloudResponse)
	test.That(t, err, test.ShouldBeNil)

	// A bind address with no port passes proto conversion but fails local processing (Network.Validate
	// in Ensure), mirroring a cloud config the cloud serves but this robot cannot apply.
	networkConfProto, err := NetworkConfigToProto(&NetworkConfig{NetworkConfigData: NetworkConfigData{BindAddress: "no-port-here"}})
	test.That(t, err, test.ShouldBeNil)

	protoConfig := &pb.RobotConfig{Cloud: cloudConfProto, Network: networkConfProto}
	fakeServer.StoreDeviceConfig(robotPartID, protoConfig, &pb.CertificateResponse{})

	appAddress := fmt.Sprintf("http://%s", fakeServer.Addr().String())
	appConn, err := grpc.NewAppConn(ctx, appAddress, robotPartID, cloudResponse.GetCloudCredsDialOpt(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer appConn.Close()

	cfgText := fmt.Sprintf(`{"cloud":{"id":%q,"app_address":%q,"secret":%q}}`, robotPartID, appAddress, secret)
	cfg, err := FromReader(ctx, "", strings.NewReader(cfgText), logger, appConn)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, IsMalformedConfigError(err), test.ShouldBeTrue)
	test.That(t, err.Error(), test.ShouldContainSubstring, "config was malformed")
	test.That(t, cfg, test.ShouldBeNil)
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

	// read config from cloud, confirm consistency. The app address is unreachable, so this
	// exercises the cache fallback in firstReadFromCloud.
	cloudCfg, err := firstReadFromCloud(ctx, cfg.Cloud, logger, appConn)
	test.That(t, err, test.ShouldBeNil)
	cloudCfg.toCache = nil
	test.That(t, cloudCfg, test.ShouldResemble, cfg)

	// Modify our config
	newRemote := Remote{Name: "test", Address: "foo"}
	cfg.Remotes = append(cfg.Remotes, newRemote)

	// read config from cloud again, confirm that the cached config differs from cfg
	cloudCfg2, err := firstReadFromCloud(ctx, cfg.Cloud, logger, appConn)
	test.That(t, err, test.ShouldBeNil)
	cloudCfg2.toCache = nil
	test.That(t, cloudCfg2, test.ShouldNotResemble, cfgToCache)

	// store the updated config to the cloud
	cfgToCache.SetToCache(cfg)
	err = cfgToCache.StoreToCache()
	test.That(t, err, test.ShouldBeNil)

	test.That(t, cfg.Ensure(true, logger), test.ShouldBeNil)

	// read updated cloud config, confirm that it now matches our updated cfg
	cloudCfg3, err := firstReadFromCloud(ctx, cfg.Cloud, logger, appConn)
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

// fullyPopulatedCloud returns a Cloud with every field set to a distinct, non-zero value. The
// zero-field check in TestCloudFieldsAreAccountedFor fails if a new field is left unset here, which
// forces it to be populated so the behavioral assertions below actually exercise it.
func fullyPopulatedCloud() *Cloud {
	return &Cloud{
		ID:                "id-1",
		Secret:            "secret-1",
		LocationSecret:    "locsecret-1",
		LocationSecrets:   []LocationSecret{{ID: "ls-id-1", Secret: "ls-secret-1"}},
		APIKey:            APIKey{ID: "key-id-1", Key: "key-secret-1"},
		LocationID:        "loc-1",
		PrimaryOrgID:      "org-1",
		MachineID:         "machine-1",
		ManagedBy:         "managed-1",
		FQDN:              "fqdn-1",
		LocalFQDN:         "localfqdn-1",
		SignalingAddress:  "sig-1",
		SignalingInsecure: true,
		AppAddress:        "app-1",
		RefreshInterval:   42 * time.Second,
		TLSCertificate:    "cert-1",
		TLSPrivateKey:     "privkey-1",
	}
}

// TestCloudFieldsAreAccountedFor pins down where every field of Cloud comes from after a cloud
// read. A field the cloud does not send must be restored from the local config or it is silently
// zeroed -- that is how APIKey was once dropped, downgrading API-key machines to secret auth.
//
// It guards the several "keep in sync with the Cloud struct" comments at once: restoreLocalOnlyFields,
// Cloud.Copy, and cloudData/MarshalJSON. If it fails, a field was added to Cloud; classify it into
// one of the buckets below and make sure Copy clones it and cloudData round-trips it.
func TestCloudFieldsAreAccountedFor(t *testing.T) {
	// Fields the cloud sends back; CloudConfigFromProto populates these.
	fromCloud := []string{
		"LocationSecret", "LocationSecrets", "LocationID", "PrimaryOrgID", "MachineID",
		"ManagedBy", "FQDN", "LocalFQDN", "SignalingAddress", "SignalingInsecure",
	}
	// Fields only the on-disk config has; restoreLocalOnlyFields must put these back.
	fromLocal := []string{"ID", "Secret", "APIKey", "AppAddress", "RefreshInterval"}
	// Fetched from the certificate endpoint and stamped by applyCloudConfig.
	fromCertEndpoint := []string{"TLSCertificate", "TLSPrivateKey"}

	fromLocalSet := map[string]bool{}
	accountedFor := map[string]bool{}
	for _, name := range fromLocal {
		fromLocalSet[name] = true
	}
	for _, group := range [][]string{fromCloud, fromLocal, fromCertEndpoint} {
		for _, name := range group {
			accountedFor[name] = true
		}
	}

	cloudType := reflect.TypeOf(Cloud{})
	for i := range cloudType.NumField() {
		name := cloudType.Field(i).Name
		test.That(t, accountedFor[name], test.ShouldBeTrue)
		delete(accountedFor, name)
	}
	// Nothing listed above should have been removed from Cloud without updating this test.
	test.That(t, accountedFor, test.ShouldBeEmpty)

	// Guard the helper: if a new field was added to Cloud and not set above, the behavioral checks
	// that follow would silently skip it, so fail here instead.
	populated := reflect.ValueOf(fullyPopulatedCloud()).Elem()
	for i := range populated.NumField() {
		test.That(t, populated.Field(i).IsZero(), test.ShouldBeFalse)
	}

	// restoreLocalOnlyFields must set exactly the local-owned fields to the local config's values
	// and leave every cloud-owned and cert field untouched. Re-adding the old mergeCloudConfig
	// clobber (which overwrote the whole cloud section) fails here.
	cloudBase := fullyPopulatedCloud()
	beforeRestore := *cloudBase
	local := &Cloud{
		ID:              "local-id",
		Secret:          "local-secret",
		APIKey:          APIKey{ID: "local-key-id", Key: "local-key-secret"},
		AppAddress:      "local-app",
		RefreshInterval: 99 * time.Second,
	}
	cloudBase.restoreLocalOnlyFields(local)
	restored := reflect.ValueOf(cloudBase).Elem()
	localVal := reflect.ValueOf(local).Elem()
	origVal := reflect.ValueOf(&beforeRestore).Elem()
	for i := range restored.NumField() {
		name := restored.Type().Field(i).Name
		if fromLocalSet[name] {
			test.That(t, restored.Field(i).Interface(), test.ShouldResemble, localVal.Field(i).Interface())
		} else {
			test.That(t, restored.Field(i).Interface(), test.ShouldResemble, origVal.Field(i).Interface())
		}
	}

	// Cloud.Copy must be a deep copy: an equal value whose reference-typed fields do not alias the
	// original's. A new slice/map field that Copy forgets to clone is caught by the aliasing check.
	src := fullyPopulatedCloud()
	dst := src.Copy()
	test.That(t, dst, test.ShouldResemble, src)
	srcElem := reflect.ValueOf(src).Elem()
	dstElem := reflect.ValueOf(dst).Elem()
	for i := range srcElem.NumField() {
		f := srcElem.Field(i)
		switch f.Kind() {
		case reflect.Slice, reflect.Map:
			if f.Len() > 0 {
				test.That(t, dstElem.Field(i).Pointer(), test.ShouldNotEqual, f.Pointer())
			}
		}
	}
	// And prove the clone is independent: mutating the copy leaves the original alone.
	dst.LocationSecrets[0].Secret = "mutated"
	dst.LocationSecrets = append(dst.LocationSecrets, LocationSecret{ID: "extra"})
	test.That(t, src.LocationSecrets, test.ShouldResemble, fullyPopulatedCloud().LocationSecrets)

	// Every field must round-trip through cloudData/MarshalJSON/UnmarshalJSON, or the on-disk cache
	// silently drops it. A new Cloud field not wired into cloudData is caught here.
	full := fullyPopulatedCloud()
	data, err := json.Marshal(full)
	test.That(t, err, test.ShouldBeNil)
	var roundTripped Cloud
	test.That(t, json.Unmarshal(data, &roundTripped), test.ShouldBeNil)
	test.That(t, &roundTripped, test.ShouldResemble, full)
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

	// certs are scoped to a location, so a location change must force a refetch.
	cloud2 = cloud1
	cloud2.LocationID = "another-location"
	test.That(t, shouldCheckForCert(&cloud1, &cloud2), test.ShouldBeTrue)

	cloud2 = cloud1
	cloud2.PrimaryOrgID = "another-org"
	test.That(t, shouldCheckForCert(&cloud1, &cloud2), test.ShouldBeTrue)
}

// TestReadFromCloudCertStaleCache is a regression test for RSDK-11851.
//
// The cert only comes down from the cloud once an hour, so every other watcher poll carries it
// over from somewhere. That used to be the on-disk cache, which is only written at the *end* of
// reconfiguration (localRobot.reconfigure -> StoreToCache). When a reconfigure runs longer than
// the refresh interval -- a stalled remote, a slow module download -- the next poll reads a cache
// still holding the *previous* cert, and the polls take turns writing their cert back to disk. The
// cloud section then changes every poll and the machine reconfigures forever.
//
// readFromCloud must therefore carry the cert forward from prevCloudCfg (in memory) and never
// consult the cache, so a lagging cache cannot drag the cert backwards.
func TestReadFromCloudCertStaleCache(t *testing.T) {
	const (
		robotPartID = "certStaleCacheTest"
		secret      = testutils.FakeCredentialPayLoad

		// Three distinct cert sources, so assertions can tell which one was used: the lagging
		// on-disk cache, the cert carried forward in memory, and what the cloud has now.
		staleCert = "stale-cert-on-disk"
		staleKey  = "stale-key-on-disk"
		heldCert  = "held-cert-in-memory"
		heldKey   = "held-key-in-memory"
		cloudCert = "cert-from-cloud"
		cloudKey  = "key-from-cloud"
	)
	var (
		logger = logging.NewTestLogger(t)
		ctx    = context.Background()
	)

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

	setup := func(t *testing.T) (*testutils.FakeCloudServer, rpc.ClientConn, *Cloud) {
		t.Helper()

		clearCache(robotPartID)
		t.Cleanup(func() { clearCache(robotPartID) })

		fakeServer, cleanup := testutils.NewFakeCloudServer(t, ctx, logger)
		t.Cleanup(cleanup)

		cloudConfProto, err := CloudConfigToProto(cloudResponse)
		test.That(t, err, test.ShouldBeNil)
		fakeServer.StoreDeviceConfig(robotPartID, &pb.RobotConfig{Cloud: cloudConfProto},
			&pb.CertificateResponse{TlsCertificate: cloudCert, TlsPrivateKey: cloudKey})

		appAddress := fmt.Sprintf("http://%s", fakeServer.Addr().String())
		appConn, err := grpc.NewAppConn(ctx, appAddress, robotPartID, cloudResponse.GetCloudCredsDialOpt(), logger)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { test.That(t, appConn.Close(), test.ShouldBeNil) })

		// Simulate a reconfigure that is still in flight: the cert the previous poll fetched is
		// held in memory, but the cache on disk has not caught up and still holds the old one.
		staleOnDisk := *cloudResponse
		staleOnDisk.TLSCertificate = staleCert
		staleOnDisk.TLSPrivateKey = staleKey
		cfgToCache := &Config{Cloud: &Cloud{ID: robotPartID}}
		test.That(t, cfgToCache.SetToCache(&Config{Cloud: &staleOnDisk}), test.ShouldBeNil)
		test.That(t, cfgToCache.StoreToCache(), test.ShouldBeNil)

		prevCloudCfg := *cloudResponse
		prevCloudCfg.AppAddress = appAddress
		prevCloudCfg.TLSCertificate = heldCert
		prevCloudCfg.TLSPrivateKey = heldKey

		return fakeServer, appConn, &prevCloudCfg
	}

	t.Run("stale cache does not clobber the in-memory cert", func(t *testing.T) {
		_, appConn, prevCloudCfg := setup(t)

		// checkForNewCert is false: this is one of the ~360 polls per hour that does not refetch.
		gotCfg, err := readFromCloud(ctx, robotPartID, prevCloudCfg, false, logger, appConn)
		test.That(t, err, test.ShouldBeNil)

		// Before the fix this read the cert off disk and came back with staleCert. It must not be
		// cloudCert either -- that would mean hitting the cert endpoint on a non-refresh poll.
		test.That(t, gotCfg.Cloud.TLSCertificate, test.ShouldEqual, heldCert)
		test.That(t, gotCfg.Cloud.TLSPrivateKey, test.ShouldEqual, heldKey)
	})

	t.Run("repeated polls do not flip-flop the cert", func(t *testing.T) {
		_, appConn, prevCloudCfg := setup(t)

		// Drive the watcher's loop by hand without ever storing to the cache -- i.e. reconfiguration
		// never finishes. Any cert change here is a cloud-section diff that reconfigures again.
		for range 5 {
			gotCfg, err := readFromCloud(ctx, robotPartID, prevCloudCfg, false, logger, appConn)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, gotCfg.Cloud.TLSCertificate, test.ShouldEqual, heldCert)
			test.That(t, gotCfg.Cloud.TLSPrivateKey, test.ShouldEqual, heldKey)

			cp, err := gotCfg.CopyOnlyPublicFields()
			test.That(t, err, test.ShouldBeNil)
			prevCloudCfg = cp.Cloud
		}
	})

	t.Run("rotated cert is picked up when checkForNewCert is set", func(t *testing.T) {
		_, appConn, prevCloudCfg := setup(t)

		// This is the once-an-hour poll: it must take what the cloud has now, not what it held.
		gotCfg, err := readFromCloud(ctx, robotPartID, prevCloudCfg, true, logger, appConn)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotCfg.Cloud.TLSCertificate, test.ShouldEqual, cloudCert)
		test.That(t, gotCfg.Cloud.TLSPrivateKey, test.ShouldEqual, cloudKey)
	})

	t.Run("cert is fetched when the previous cloud config has none", func(t *testing.T) {
		_, appConn, prevCloudCfg := setup(t)
		prevCloudCfg.TLSCertificate = ""
		prevCloudCfg.TLSPrivateKey = ""

		// checkForNewCert is false, but we have nothing to carry forward, so we must fetch.
		gotCfg, err := readFromCloud(ctx, robotPartID, prevCloudCfg, false, logger, appConn)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotCfg.Cloud.TLSCertificate, test.ShouldEqual, cloudCert)
		test.That(t, gotCfg.Cloud.TLSPrivateKey, test.ShouldEqual, cloudKey)
	})

	t.Run("cert is refetched when the cloud section invalidates it", func(t *testing.T) {
		_, appConn, prevCloudCfg := setup(t)

		// The cert is issued for the machine's FQDN, so an FQDN change forces a refetch even with
		// checkForNewCert false.
		prevCloudCfg.FQDN = "some-old-fqdn"

		gotCfg, err := readFromCloud(ctx, robotPartID, prevCloudCfg, false, logger, appConn)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotCfg.Cloud.TLSCertificate, test.ShouldEqual, cloudCert)
		test.That(t, gotCfg.Cloud.TLSPrivateKey, test.ShouldEqual, cloudKey)
	})

	// breakCertEndpoint fails the cert endpoint while leaving the config endpoint working. An empty
	// CertificateResponse is what the cloud sends when a cert is not ready yet.
	breakCertEndpoint := func(t *testing.T, fakeServer *testutils.FakeCloudServer) {
		t.Helper()
		cloudConfProto, err := CloudConfigToProto(cloudResponse)
		test.That(t, err, test.ShouldBeNil)
		fakeServer.StoreDeviceConfig(robotPartID, &pb.RobotConfig{Cloud: cloudConfProto},
			&pb.CertificateResponse{})
	}

	t.Run("a failed cert refresh keeps the previous cert and the new config", func(t *testing.T) {
		fakeServer, appConn, prevCloudCfg := setup(t)
		breakCertEndpoint(t, fakeServer)

		// A transient cert-endpoint failure must not cost us the config we just read.
		gotCfg, err := readFromCloud(ctx, robotPartID, prevCloudCfg, true, logger, appConn)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotCfg.Cloud.TLSCertificate, test.ShouldEqual, heldCert)
		test.That(t, gotCfg.Cloud.TLSPrivateKey, test.ShouldEqual, heldKey)
	})

	t.Run("a failed cert refresh with no previous cert errors", func(t *testing.T) {
		fakeServer, appConn, prevCloudCfg := setup(t)
		breakCertEndpoint(t, fakeServer)
		prevCloudCfg.TLSCertificate = ""
		prevCloudCfg.TLSPrivateKey = ""

		// Nothing to fall back to and signaling is secure, so error rather than handing back a
		// config with no cert.
		_, err := readFromCloud(ctx, robotPartID, prevCloudCfg, true, logger, appConn)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("cloud fields that govern the app connection are preserved", func(t *testing.T) {
		_, appConn, prevCloudCfg := setup(t)
		prevCloudCfg.RefreshInterval = 42 * time.Second

		gotCfg, err := readFromCloud(ctx, robotPartID, prevCloudCfg, false, logger, appConn)
		test.That(t, err, test.ShouldBeNil)

		// The cloud does not send these back, so they must survive from the previous config.
		test.That(t, gotCfg.Cloud.ID, test.ShouldEqual, prevCloudCfg.ID)
		test.That(t, gotCfg.Cloud.Secret, test.ShouldEqual, prevCloudCfg.Secret)
		test.That(t, gotCfg.Cloud.AppAddress, test.ShouldEqual, prevCloudCfg.AppAddress)
		test.That(t, gotCfg.Cloud.RefreshInterval, test.ShouldEqual, 42*time.Second)
	})

	// The insecure-signaling branch of readFromCloud does not fetch a cert and must not carry one
	// forward either. A machine that had a cert and then flips to insecure signaling is talking to
	// an app instance that has no cert for it. Carrying the old cert forward would leave
	// weboptions.FromConfig setting up TLS and binding :8080 against that instance, and would keep
	// re-caching a cert nothing can validate. Blanking is safe: ProcessConfig skips
	// CreateTLSWithCert when TLSCertificate is empty.
	t.Run("insecure signaling blanks the carried-forward cert", func(t *testing.T) {
		fakeServer, appConn, prevCloudCfg := setup(t)

		// The cloud flips to insecure signaling, so it no longer has a cert for this machine, while
		// prevCloudCfg still carries the cert held from when signaling was secure.
		insecureCloud := *cloudResponse
		insecureCloud.SignalingInsecure = true
		insecureProto, err := CloudConfigToProto(&insecureCloud)
		test.That(t, err, test.ShouldBeNil)
		fakeServer.StoreDeviceConfig(robotPartID, &pb.RobotConfig{Cloud: insecureProto}, &pb.CertificateResponse{})

		gotCfg, err := readFromCloud(ctx, robotPartID, prevCloudCfg, false, logger, appConn)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotCfg.Cloud.SignalingInsecure, test.ShouldBeTrue)
		test.That(t, gotCfg.Cloud.TLSCertificate, test.ShouldBeEmpty)
		test.That(t, gotCfg.Cloud.TLSPrivateKey, test.ShouldBeEmpty)

		// The cache is staged from the same blanked cert, so the next boot does not resurrect it.
		test.That(t, gotCfg.toCache, test.ShouldNotBeNil)
		test.That(t, string(gotCfg.toCache), test.ShouldNotContainSubstring, heldCert)
		test.That(t, string(gotCfg.toCache), test.ShouldNotContainSubstring, heldKey)
	})
}

// TestFirstReadFromCloudRequiresCert covers startup when the cloud has no cert to hand out --
// typically a machine whose cert has not been issued yet, where the Certificate endpoint answers
// with an empty response.
//
// Coming up anyway is worse than failing. weboptions.FromConfig gates its TLS *and* its
// bind-address setup on a non-empty TLSCertificate, so a certless machine serves plaintext on the
// default bind address. Worse, applyCloudConfig would cache the empty cert, and on the following
// boot tlsConfig.readFromCache fails ValidateTLS and clears the entire cache -- so the machine also
// loses the config it needs to boot offline.
func TestFirstReadFromCloudRequiresCert(t *testing.T) {
	const (
		robotPartID = "certRequiredTest"
		secret      = testutils.FakeCredentialPayLoad

		cachedCert = "cached-cert"
		cachedKey  = "cached-key"
	)
	var (
		logger = logging.NewTestLogger(t)
		ctx    = context.Background()
	)

	// setup stands up a cloud that serves a config but no certificate.
	setup := func(t *testing.T, signalingInsecure bool) (*testutils.FakeCloudServer, rpc.ClientConn, *Cloud) {
		t.Helper()

		clearCache(robotPartID)
		t.Cleanup(func() { clearCache(robotPartID) })

		fakeServer, cleanup := testutils.NewFakeCloudServer(t, ctx, logger)
		t.Cleanup(cleanup)

		cloudResponse := &Cloud{
			ManagedBy:         "acme",
			SignalingAddress:  "abc",
			ID:                robotPartID,
			Secret:            secret,
			FQDN:              "fqdn",
			LocalFQDN:         "localFqdn",
			LocationSecrets:   []LocationSecret{},
			SignalingInsecure: signalingInsecure,
		}
		cloudConfProto, err := CloudConfigToProto(cloudResponse)
		test.That(t, err, test.ShouldBeNil)
		// An empty CertificateResponse is what the cloud sends when no cert has been issued.
		fakeServer.StoreDeviceConfig(robotPartID, &pb.RobotConfig{Cloud: cloudConfProto}, &pb.CertificateResponse{})

		appAddress := fmt.Sprintf("http://%s", fakeServer.Addr().String())
		appConn, err := grpc.NewAppConn(ctx, appAddress, robotPartID, cloudResponse.GetCloudCredsDialOpt(), logger)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { test.That(t, appConn.Close(), test.ShouldBeNil) })

		localCloudCfg := *cloudResponse
		localCloudCfg.AppAddress = appAddress
		localCloudCfg.RefreshInterval = time.Second
		return fakeServer, appConn, &localCloudCfg
	}

	// storeCachedCert writes a cached config carrying a cert, as an earlier boot would have.
	storeCachedCert := func(t *testing.T, cloudCfg *Cloud) {
		t.Helper()
		cached := *cloudCfg
		cached.TLSCertificate = cachedCert
		cached.TLSPrivateKey = cachedKey
		cfgToCache := &Config{Cloud: &Cloud{ID: robotPartID}}
		test.That(t, cfgToCache.SetToCache(&Config{Cloud: &cached}), test.ShouldBeNil)
		test.That(t, cfgToCache.StoreToCache(), test.ShouldBeNil)
	}

	t.Run("no cert from the cloud and no cache errors", func(t *testing.T) {
		_, appConn, localCloudCfg := setup(t, false)

		_, err := firstReadFromCloud(ctx, localCloudCfg, logger, appConn)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no TLS certificate available")
	})

	// The backstop in applyCloudConfig, tested directly: the callers above error out before
	// reaching it, so nothing else exercises it.
	t.Run("applyCloudConfig refuses to stage a cache write with no cert", func(t *testing.T) {
		localCloudCfg := &Cloud{ID: robotPartID, Secret: secret}

		newResult := func(insecure bool) cloudReadResult {
			return cloudReadResult{
				processed:   &Config{Cloud: &Cloud{ID: robotPartID, SignalingInsecure: insecure}},
				unprocessed: &Config{Cloud: &Cloud{ID: robotPartID, SignalingInsecure: insecure}},
			}
		}

		// Secure signaling with no cert: must not be staged.
		res := newResult(false)
		applyCloudConfig(res, tlsConfig{}, localCloudCfg, logger)
		test.That(t, res.processed.toCache, test.ShouldBeNil)

		// Insecure signaling legitimately has no cert, so staging is expected.
		res = newResult(true)
		applyCloudConfig(res, tlsConfig{}, localCloudCfg, logger)
		test.That(t, res.processed.toCache, test.ShouldNotBeNil)

		// Secure signaling with a cert is the normal path.
		res = newResult(false)
		applyCloudConfig(res, tlsConfig{certificate: cachedCert, privateKey: cachedKey}, localCloudCfg, logger)
		test.That(t, res.processed.toCache, test.ShouldNotBeNil)
	})

	t.Run("no cert from the cloud falls back to the cached cert", func(t *testing.T) {
		_, appConn, localCloudCfg := setup(t, false)
		storeCachedCert(t, localCloudCfg)

		gotCfg, err := firstReadFromCloud(ctx, localCloudCfg, logger, appConn)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotCfg.Cloud.TLSCertificate, test.ShouldEqual, cachedCert)
		test.That(t, gotCfg.Cloud.TLSPrivateKey, test.ShouldEqual, cachedKey)
	})

	t.Run("insecure signaling does not need a cert", func(t *testing.T) {
		_, appConn, localCloudCfg := setup(t, true)

		gotCfg, err := firstReadFromCloud(ctx, localCloudCfg, logger, appConn)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotCfg.Cloud.TLSCertificate, test.ShouldBeEmpty)
		test.That(t, gotCfg.Cloud.TLSPrivateKey, test.ShouldBeEmpty)
	})

	// The discriminating case for the insecure branch: the config itself comes from the cache, so
	// it arrives carrying the cert an earlier secure boot cached. It must still be blanked.
	t.Run("insecure signaling drops a cert carried in from the cache", func(t *testing.T) {
		fakeServer, appConn, localCloudCfg := setup(t, true)
		storeCachedCert(t, localCloudCfg)
		fakeServer.FailOnConfigAndCerts(true)

		gotCfg, err := firstReadFromCloud(ctx, localCloudCfg, logger, appConn)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotCfg.Cloud.SignalingInsecure, test.ShouldBeTrue)
		test.That(t, gotCfg.Cloud.TLSCertificate, test.ShouldBeEmpty)
		test.That(t, gotCfg.Cloud.TLSPrivateKey, test.ShouldBeEmpty)
	})
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
