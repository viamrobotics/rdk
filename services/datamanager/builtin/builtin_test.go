package builtin

import (
	"bytes"
	"context"
	"fmt"
	"go.viam.com/rdk/spatialmath"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager/internal"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	captureWaitTime = time.Millisecond * 25
	syncWaitTime    = time.Millisecond * 100
)

var (
	// Robot config which specifies data manager service.
	configPath = "services/datamanager/data/fake_robot_with_data_manager.json"
)

// readDir filters out folders from a slice of FileInfos.
func readDir(t *testing.T, dir string) ([]fs.DirEntry, error) {
	t.Helper()
	filesAndFolders, err := os.ReadDir(dir)
	if err != nil {
		t.Log(err)
	}
	var onlyFiles []fs.DirEntry
	for _, s := range filesAndFolders {
		if !s.IsDir() {
			onlyFiles = append(onlyFiles, s)
		}
	}
	return onlyFiles, err
}

// TODO: get rid of this and just use os.RemoveAll where needed
func resetFolder(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Log(err)
	}
}

func getInjectedRobot() *inject.Robot {
	r := &inject.Robot{}
	rs := map[resource.Name]interface{}{}
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

func newTestDataManager(t *testing.T) internal.DMService {
	t.Helper()
	dmCfg := &Config{}
	cfgService := config.Service{
		Type:                "data_manager",
		ConvertedAttributes: dmCfg,
	}
	logger := golog.NewTestLogger(t)

	// Create local robot with injected arm.
	r := getInjectedRobot()
	remoteRobot := getInjectedRobot()
	r.RemoteByNameFunc = func(name string) (robot.Robot, bool) {
		return remoteRobot, true
	}

	// If passed, create remote robot with an injected arm.
	//if remoteArmKey != "" {
	//	remoteRobot := getInjectedRobot()
	//
	//	r.RemoteByNameFunc = func(name string) (robot.Robot, bool) {
	//		return remoteRobot, true
	//	}
	//}

	svc, err := NewBuiltIn(context.Background(), r, cfgService, logger)
	if err != nil {
		t.Log(err)
	}
	return svc.(internal.DMService)
}

func setupConfig(t *testing.T, relativePath string) *config.Config {
	t.Helper()
	logger := golog.NewTestLogger(t)
	testCfg, err := config.Read(context.Background(), rdkutils.ResolveFile(relativePath), logger)
	test.That(t, err, test.ShouldBeNil)
	testCfg.Cloud = &config.Cloud{ID: "part_id"}
	return testCfg
}

// TODO: add sync testing here too
// TODO: update to not longer hard code all these paths. Maybe even just include as part of other test suite.
func TestNewRemoteDataManager(t *testing.T) {
	// Empty config at initialization.
	captureDir := "/tmp/capture"
	dmsvc := newTestDataManager(t)

	// Set capture parameters in Update.
	conf := setupConfig(t, "services/datamanager/data/fake_robot_with_remote_and_data_manager.json")
	defer resetFolder(t, captureDir)
	fmt.Println("updating")
	err := dmsvc.Update(context.Background(), conf)
	fmt.Println("updated")
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(captureWaitTime)

	// Verify that after close is called, the collector is no longer writing.
	fmt.Println("closing")
	err = dmsvc.Close(context.Background())
	fmt.Println("Closed")
	test.That(t, err, test.ShouldBeNil)

	// Verify that the local and remote collectors wrote to their files.
	localArmDir := captureDir + "/arm/arm1/EndPosition"
	filesInLocalArmDir, err := readDir(t, localArmDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInLocalArmDir), test.ShouldEqual, 1)
	info, err := filesInLocalArmDir[0].Info()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info.Size(), test.ShouldBeGreaterThan, 0)

	remoteArmDir := captureDir + "/arm/remoteArm/EndPosition"
	filesInRemoteArmDir, err := readDir(t, remoteArmDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInRemoteArmDir), test.ShouldEqual, 1)
	info, err = filesInRemoteArmDir[0].Info()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info.Size(), test.ShouldBeGreaterThan, 0)
}

// TODO: add arbitrary file sync tests, including with syncAdditionalPaths

func TestGetDurationFromHz(t *testing.T) {
	test.That(t, GetDurationFromHz(0.1), test.ShouldEqual, time.Second*10)
	test.That(t, GetDurationFromHz(0.5), test.ShouldEqual, time.Second*2)
	test.That(t, GetDurationFromHz(1), test.ShouldEqual, time.Second)
	test.That(t, GetDurationFromHz(1000), test.ShouldEqual, time.Millisecond)
	test.That(t, GetDurationFromHz(0), test.ShouldEqual, 0)
}

func getDataManagerConfig(config *config.Config) (*Config, error) {
	svcConfig, ok, err := GetServiceConfig(config)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("failed to get service config")
	}
	return svcConfig, nil
}

func getLocalServerConn(rpcServer rpc.Server, logger golog.Logger) (rpc.ClientConn, error) {
	return rpc.DialDirectGRPC(
		context.Background(),
		rpcServer.InternalAddr().String(),
		logger,
		rpc.WithInsecure(),
	)
}

func getAllFiles(dir string) []os.FileInfo {
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
