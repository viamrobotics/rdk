package builtin

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/internal/cloud"
	cloudinject "go.viam.com/rdk/internal/testutils/inject"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

var (
	// Robot config which specifies data manager service.
	enabledTabularCollectorConfigPath = "services/datamanager/data/fake_robot_with_data_manager.json"
	// disabledTabularCollectorConfigPath          = "services/datamanager/data/fake_robot_with_disabled_collector.json"
	// enabledBinaryCollectorConfigPath            = "services/datamanager/data/robot_with_cam_capture.json"
	// infrequentCaptureTabularCollectorConfigPath = "services/datamanager/data/fake_robot_with_infrequent_capture.json"
	// remoteCollectorConfigPath                   = "services/datamanager/data/fake_robot_with_remote_and_data_manager.json"
	// emptyFileBytesSize                          = 90 // a "rounded down" size of leading metadata message
	// captureInterval                             = time.Millisecond * 10.
)

var connectedConn = newConnectingNoOpClientConnWithConnectivity()

func TestUntrustedEnv(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx, err := utils.WithTrustedEnvironment(context.Background(), false)
	test.That(t, err, test.ShouldBeNil)
	r := getInjectedRobot(connectedConn, map[resource.Name]resource.Resource{
		arm.Named("arm1"): &inject.Arm{},
	})
	config, deps := setupConfig(t, r, enabledTabularCollectorConfigPath)
	_, err = NewBuiltIn(ctx, deps, config, noOpCloudClientConstructor, logger)
	test.That(t, err, test.ShouldEqual, ErrCaptureDirectoryConfigurationDisabled)
}

func getInjectedRobot(conn rpc.ClientConn, rs map[resource.Name]resource.Resource) *inject.Robot {
	r := &inject.Robot{}
	rs[cloud.InternalServiceName] = &cloudinject.CloudConnectionService{
		Named: cloud.InternalServiceName.AsNamed(),
		Conn:  conn,
	}

	// injectedArm := &inject.Arm{}
	// injectedArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	// 	return spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3}), nil
	// }
	// rs[arm.Named("arm1")] = injectedArm

	// injectedRemoteArm := &inject.Arm{}
	// injectedRemoteArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	// 	return spatialmath.NewZeroPose(), nil
	// }
	// rs[arm.Named("remote1:remoteArm")] = injectedRemoteArm

	// injectedCam := &inject.Camera{}
	// img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	// var imgBuf bytes.Buffer
	// test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)
	// imgPng, _ := png.Decode(bytes.NewReader(imgBuf.Bytes()))
	// injectedCam.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	// 	return gostream.NewEmbeddedVideoStreamFromReader(
	// 		gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
	// 			return imgPng, func() {}, nil
	// 		}),
	// 	), nil
	// }
	// rs[camera.Named("c1")] = injectedCam

	r.MockResourcesFromMap(rs)
	return r
}

// func newBuiltIn(
// 	t *testing.T,
// 	cloudClientConstructor func(grpc.ClientConnInterface) v1.DataSyncServiceClient,
// 	conn rpc.ClientConn,
// ) (*builtIn, robot.Robot) {
// 	t.Helper()
// 	cfgService := resource.Config{
// 		API:                 datamanager.API,
// 		ConvertedAttributes: &Config{},
// 	}
// 	logger := logging.NewTestLogger(t)

// 	// Create local robot with injected arm and remote.
// 	r := getInjectedRobot(t, conn)
// 	// TODO: Not sure if we need this or not
// 	// remoteRobot := getInjectedRobot(t, conn)
// 	// r.RemoteByNameFunc = func(name string) (robot.Robot, bool) {
// 	// 	return remoteRobot, true
// 	// }

// 	deps := resourcesFromDeps(t, r, []string{cloud.InternalServiceName.String()})
// 	svc, err := NewBuiltIn(context.Background(), deps, cfgService, cloudClientConstructor, logger)
// 	if err != nil {
// 		t.Log(err)
// 		t.FailNow()
// 	}
// 	return svc.(*builtIn), r
// }

func setupConfig(t *testing.T, r *inject.Robot, configPath string) (resource.Config, resource.Dependencies) {
	t.Helper()
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), utils.ResolveFile(configPath), logger)
	test.That(t, err, test.ShouldBeNil)
	return resourceConfigAndDeps(t, cfg, r)
}

func resourceConfigAndDeps(t *testing.T, cfg *config.Config, r *inject.Robot) (resource.Config, resource.Dependencies) {
	var config *resource.Config
	deps := resource.Dependencies{}
	// datamanager config should be in the config, if not test is inavlif
	for _, c := range cfg.Services {
		if c.API == datamanager.API {
			if config != nil {
				t.Fatal("there should only be one instance of data manager")
			}
			_, ok := c.ConvertedAttributes.(*Config)
			test.That(t, ok, test.ShouldBeTrue)
			for name, assocConf := range c.AssociatedAttributes {
				_, ok := assocConf.(*datamanager.AssociatedConfig)
				test.That(t, ok, test.ShouldBeTrue)
				res, err := r.ResourceByName(name)
				// if the config requires a resource which we have not set a mock for, fail the test
				test.That(t, errors.Wrap(err, name.String()), test.ShouldBeNil)
				deps[name] = res
			}
			config = &c
		}
	}
	test.That(t, config, test.ShouldNotBeNil)
	return *config, deps
}

// func getServiceConfig(t *testing.T, cfg *config.Config) (*Config, map[resource.Name]resource.AssociatedConfig, []string) {
// 	t.Helper()
// 	for _, c := range cfg.Services {
// 		// Compare service type and name.
// 		if c.API == datamanager.API && c.ConvertedAttributes != nil {
// 			svcConfig, ok := c.ConvertedAttributes.(*Config)
// 			test.That(t, ok, test.ShouldBeTrue)

// 			var deps []string
// 			for _, resConf := range c.AssociatedAttributes {
// 				assocConf, ok := resConf.(*datamanager.AssociatedConfig)
// 				test.That(t, ok, test.ShouldBeTrue)
// 				if len(assocConf.CaptureMethods) == 0 {
// 					continue
// 				}
// 				deps = append(deps, assocConf.CaptureMethods[0].Name.String())
// 			}
// 			deps = append(deps, c.ImplicitDependsOn...)
// 			return svcConfig, c.AssociatedAttributes, deps
// 		}
// 	}

// 	t.Log("no service config")
// 	return nil, nil, nil
// }

//func getAllFileInfos(dir string) []os.FileInfo {
//	var files []os.FileInfo
//	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
//		if err != nil || info.IsDir() {
//			// ignore errors/unreadable files and directories
//			//nolint:nilerr
//			return nil
//		}
//		files = append(files, info)
//		return nil
//	})
//	return files
//}

// func resourcesFromDeps(t *testing.T, r robot.Robot, deps []string) resource.Dependencies {
// 	t.Helper()
// 	resources := resource.Dependencies{}
// 	for _, dep := range deps {
// 		resName, err := resource.NewFromString(dep)
// 		test.That(t, err, test.ShouldBeNil)
// 		res, err := r.ResourceByName(resName)
// 		if err == nil {
// 			// some resources are weakly linked
// 			resources[resName] = res
// 		}
// 	}
// 	return resources
// }
