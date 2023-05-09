// Package replay_test will test the  functions of a replay camera.
package replaypcd

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/testutils/inject"
)

func getInjectedRobot() *inject.Robot {
	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{}

	rs[cloud.InternalServiceName] = &noopCloudConnectionService{
		Named: cloud.InternalServiceName.AsNamed(),
	}

	r.MockResourcesFromMap(rs)
	return r
}

func TestNewReplayCamera(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	replayCamCfg := &Config{}
	cfg := resource.Config{
		ConvertedAttributes: replayCamCfg,
	}

	// Create local robot with injected camera and remote.
	r := getInjectedRobot()
	remoteRobot := getInjectedRobot()
	r.RemoteByNameFunc = func(name string) (robot.Robot, bool) {
		return remoteRobot, true
	}
	resources := resourcesFromDeps(t, r, []string{cloud.InternalServiceName.String()})

	replayCamera, err := newReplayPCDCamera(ctx, resources, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("Test NextPointCloud", func(t *testing.T) {
		_, err := replayCamera.NextPointCloud(ctx)
		test.That(t, err.Error(), test.ShouldNotBeNil)
	})

	t.Run("Test Stream", func(t *testing.T) {
		_, err := replayCamera.Stream(ctx, nil)
		test.That(t, err.Error(), test.ShouldEqual, "Stream is unimplemented")
	})

	t.Run("Test Properties", func(t *testing.T) {
		_, err := replayCamera.Properties(ctx)
		test.That(t, err.Error(), test.ShouldEqual, "Properties is unimplemented")
	})

	t.Run("Test Projector", func(t *testing.T) {
		_, err := replayCamera.Projector(ctx)
		test.That(t, err.Error(), test.ShouldEqual, "Projector is unimplemented")
	})

	err = replayCamera.Close(ctx)
	test.That(t, err.Error(), test.ShouldBeNil)
}

func TestInvalidReplayPCDCameraConfigs(t *testing.T) {
	// logger := golog.NewTestLogger(t)
	// ctx := context.Background()

	// Create local robot with injected camera and remote.
	r := getInjectedRobot()
	remoteRobot := getInjectedRobot()
	r.RemoteByNameFunc = func(name string) (robot.Robot, bool) {
		return remoteRobot, true
	}
	//resources := resourcesFromDeps(t, r, []string{cloud.InternalServiceName.String()})

	t.Run("Yes source", func(t *testing.T) {
		replayCamCfg := &Config{Source: resource.Name{Name: "test"}}
		deps, err := replayCamCfg.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, deps, test.ShouldResemble, []string{cloud.InternalServiceName.String()})
	})
	t.Run("No source", func(t *testing.T) {
		replayCamCfg := &Config{}
		deps, err := replayCamCfg.Validate("")
		test.That(t, err, test.ShouldBeError,
			goutils.NewConfigValidationFieldRequiredError("", "source"))
		test.That(t, deps, test.ShouldBeNil)
	})

}

var _ = cloud.ConnectionService(&noopCloudConnectionService{})

type noopCloudConnectionService struct {
	resource.Named
	resource.AlwaysRebuild
}

func (noop *noopCloudConnectionService) AcquireConnection(ctx context.Context) (string, rpc.ClientConn, error) {
	return "", nil, nil
}

func (noop *noopCloudConnectionService) Close(ctx context.Context) error {
	return nil
}

func resourcesFromDeps(t *testing.T, r robot.Robot, deps []string) resource.Dependencies {
	t.Helper()
	resources := resource.Dependencies{}
	for _, dep := range deps {
		resName, err := resource.NewFromString(dep)
		test.That(t, err, test.ShouldBeNil)
		res, err := r.ResourceByName(resName)
		if err == nil {
			// some resources are weakly linked
			resources[resName] = res
		}
	}
	return resources
}
