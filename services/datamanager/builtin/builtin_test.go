package builtin

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/internal/cloud"
	cloudinject "go.viam.com/rdk/internal/testutils/inject"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/internal"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func getInjectedRobot() *inject.Robot {
	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{}

	rs[cloud.InternalServiceName] = &cloudinject.CloudConnectionService{
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
	rs[arm.Named("remote1:remoteArm")] = injectedRemoteArm

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
		API:                 datamanager.API,
		ConvertedAttributes: dmCfg,
	}
	logger := logging.NewTestLogger(t)

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

func setupConfig(t *testing.T, relativePath string) (*Config, map[resource.Name]resource.AssociatedConfig, []string) {
	t.Helper()
	logger := logging.NewTestLogger(t)
	testCfg, err := config.Read(context.Background(), utils.ResolveFile(relativePath), logger)
	test.That(t, err, test.ShouldBeNil)
	return getServiceConfig(t, testCfg)
}

func getServiceConfig(t *testing.T, cfg *config.Config) (*Config, map[resource.Name]resource.AssociatedConfig, []string) {
	t.Helper()
	for _, c := range cfg.Services {
		// Compare service type and name.
		if c.API == datamanager.API && c.ConvertedAttributes != nil {
			svcConfig, ok := c.ConvertedAttributes.(*Config)
			test.That(t, ok, test.ShouldBeTrue)

			var deps []string
			for _, resConf := range c.AssociatedAttributes {
				assocConf, ok := resConf.(*datamanager.AssociatedConfig)
				test.That(t, ok, test.ShouldBeTrue)
				if len(assocConf.CaptureMethods) == 0 {
					continue
				}
				deps = append(deps, assocConf.CaptureMethods[0].Name.String())
			}
			deps = append(deps, c.ImplicitDependsOn...)
			return svcConfig, c.AssociatedAttributes, deps
		}
	}

	t.Log("no service config")
	return nil, nil, nil
}

func TestEmptyConfig(t *testing.T) {
	// Data manager should not be enabled implicitly, an empty config will not result in a data manager being configured.
	initConfig, associations, deps := setupConfig(t, enabledTabularCollectorEmptyConfigPath)
	test.That(t, initConfig, test.ShouldBeNil)
	test.That(t, associations, test.ShouldBeNil)
	test.That(t, deps, test.ShouldBeNil)
}

func TestUntrustedEnv(t *testing.T) {
	dmsvc, r := newTestDataManager(t)
	defer dmsvc.Close(context.Background())

	config, associations, deps := setupConfig(t, enabledTabularCollectorConfigPath)
	ctx, err := utils.WithTrustedEnvironment(context.Background(), false)
	test.That(t, err, test.ShouldBeNil)

	resources := resourcesFromDeps(t, r, deps)
	err = dmsvc.Reconfigure(ctx, resources, resource.Config{
		ConvertedAttributes:  config,
		AssociatedAttributes: associations,
	})
	test.That(t, err, test.ShouldEqual, data.ErrCaptureDirectoryConfigurationDisabled)
}

func getAllFileInfos(dir string) []os.FileInfo {
	var files []os.FileInfo
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			// ignore errors/unreadable files and directories
			//nolint:nilerr
			return nil
		}
		files = append(files, info)
		return nil
	})
	return files
}
