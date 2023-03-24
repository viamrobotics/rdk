package builtin

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/internal"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	rdkutils "go.viam.com/rdk/utils"
)

var (
	// Robot config which specifies data manager service.
	configPath = "services/datamanager/data/fake_robot_with_data_manager.json"
)

func getInjectedRobot() *inject.Robot {
	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{}

	rs[cloud.InternalServiceName] = &noopCloudConnectionService{
		Named: cloud.InternalServiceName.AsNamed(),
	}

	injectedArm := &inject.Arm{}
	injectedArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3}), nil
	}
	rs[arm.Named("arm1")] = injectedArm

	injectedRemoteArm := &inject.Arm{}
	injectedRemoteArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return spatialmath.NewZeroPose(), nil
	}
	rs[arm.Named("remoteArm")] = injectedRemoteArm

	injectedCam := &inject.Camera{}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var imgBuf bytes.Buffer
	png.Encode(&imgBuf, img)
	imgPng, _ := png.Decode(bytes.NewReader(imgBuf.Bytes()))
	injectedCam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
		return gostream.NewEmbeddedVideoStreamFromReader(
			gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
				return imgPng, func() {}, nil
			}),
		), nil
	}
	rs[camera.Named("c1")] = injectedCam

	r.MockResourcesFromMap(rs)
	return r
}

func newTestDataManager(t *testing.T) (internal.DMService, robot.Robot) {
	t.Helper()
	dmCfg := &Config{}
	cfgService := resource.Config{
		API:                 datamanager.Subtype,
		ConvertedAttributes: dmCfg,
	}
	logger := golog.NewTestLogger(t)

	// Create local robot with injected arm and remote.
	r := getInjectedRobot()
	remoteRobot := getInjectedRobot()
	r.RemoteByNameFunc = func(name string) (robot.Robot, bool) {
		return remoteRobot, true
	}

	resources := resourcesFromDeps(t, r, []string{cloud.InternalServiceName.String()})
	svc, err := NewBuiltIn(context.Background(), resources, cfgService, logger)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	return svc.(internal.DMService), r
}

func setupConfig(t *testing.T, relativePath string) (*Config, []string) {
	t.Helper()
	logger := golog.NewTestLogger(t)
	testCfg, err := config.Read(context.Background(), rdkutils.ResolveFile(relativePath), logger)
	test.That(t, err, test.ShouldBeNil)
	return getServiceConfig(t, testCfg)
}

func getServiceConfig(t *testing.T, cfg *config.Config) (*Config, []string) {
	t.Helper()
	for _, c := range cfg.Services {
		// Compare service type and name.
		if c.API == datamanager.Subtype && c.ConvertedAttributes != nil {
			svcConfig, ok := c.ConvertedAttributes.(*Config)
			test.That(t, ok, test.ShouldBeTrue)
			return svcConfig, c.ImplicitDependsOn
		}
	}

	t.Log("no service config")
	t.FailNow()
	return nil, nil
}

func TestGetDurationFromHz(t *testing.T) {
	test.That(t, GetDurationFromHz(0.1), test.ShouldEqual, time.Second*10)
	test.That(t, GetDurationFromHz(0.5), test.ShouldEqual, time.Second*2)
	test.That(t, GetDurationFromHz(1), test.ShouldEqual, time.Second)
	test.That(t, GetDurationFromHz(1000), test.ShouldEqual, time.Millisecond)
	test.That(t, GetDurationFromHz(0), test.ShouldEqual, 0)
}

func TestUntrustedEnv(t *testing.T) {
	dmsvc, r := newTestDataManager(t)
	defer dmsvc.Close(context.Background())

	config, deps := setupConfig(t, enabledTabularCollectorConfigPath)
	ctx, err := utils.WithTrustedEnvironment(context.Background(), false)
	test.That(t, err, test.ShouldBeNil)

	resources := resourcesFromDeps(t, r, deps)
	err = dmsvc.Reconfigure(ctx, resources, resource.Config{
		ConvertedAttributes: config,
	})
	test.That(t, err, test.ShouldEqual, errCaptureDirectoryConfigurationDisabled)
}

func getLocalServerConn(rpcServer rpc.Server, logger golog.Logger) (rpc.ClientConn, error) {
	return rpc.DialDirectGRPC(
		context.Background(),
		rpcServer.InternalAddr().String(),
		logger,
		rpc.WithInsecure(),
	)
}

func getAllFileInfos(dir string) []os.FileInfo {
	var files []os.FileInfo
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, info)
		return nil
	})
	return files
}
