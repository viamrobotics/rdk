// Package replay_test will test the  functions of a replay camera.
package replaypcd

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/testutils/inject"
)

func getNoopCloudDependencies(t *testing.T) resource.Dependencies {
	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{}

	rs[cloud.InternalServiceName] = &noopCloudConnectionService{
		Named: cloud.InternalServiceName.AsNamed(),
	}

	r.MockResourcesFromMap(rs)

	return resourcesFromDeps(t, r, []string{cloud.InternalServiceName.String()})
}

func createNewReplayPCDCamera(t *testing.T, ctx context.Context, replayCamCfg *Config, validDeps bool) (camera.Camera, error) {
	logger := golog.NewTestLogger(t)

	cfg := resource.Config{
		ConvertedAttributes: replayCamCfg,
	}

	var resources resource.Dependencies
	if validDeps {
		resources = getNoopCloudDependencies(t)
	}
	return newPCDCamera(ctx, resources, cfg, logger)
}

func TestNewReplayPCD(t *testing.T) {
	ctx := context.Background()

	t.Run("valid config with internal cloud service", func(t *testing.T) {
		replayCamCfg := &Config{Source: "source"}
		replayCamera, err := createNewReplayPCDCamera(t, ctx, replayCamCfg, true)
		test.That(t, err, test.ShouldBeNil)

		err = replayCamera.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("no internal cloud service", func(t *testing.T) {
		replayCamCfg := &Config{Source: "source"}
		replayCamera, err := createNewReplayPCDCamera(t, ctx, replayCamCfg, false)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "missing from dependencies")
		test.That(t, replayCamera, test.ShouldBeNil)

	})

	t.Run("bad start timestamp", func(t *testing.T) {
		replayCamCfg := &Config{
			Source: "source",
			Interval: TimeInterval{
				Start: "bad timestamp",
			},
		}
		replayCamera, err := createNewReplayPCDCamera(t, ctx, replayCamCfg, true)
		test.That(t, err, test.ShouldBeError, errors.New("invalid time format, use RFC3339"))
		test.That(t, replayCamera, test.ShouldBeNil)
	})

	t.Run("bad end timestamp", func(t *testing.T) {
		replayCamCfg := &Config{
			Source: "source",
			Interval: TimeInterval{
				End: "bad timestamp",
			},
		}
		replayCamera, err := createNewReplayPCDCamera(t, ctx, replayCamCfg, true)
		test.That(t, err, test.ShouldBeError, errors.New("invalid time format, use RFC3339"))
		test.That(t, replayCamera, test.ShouldBeNil)
	})
}

func TestNextPointCloud(t *testing.T) {
	ctx := context.Background()

	replayCamCfg := &Config{Source: "test"}
	replayCamera, err := createNewReplayPCDCamera(t, ctx, replayCamCfg, true)
	test.That(t, err, test.ShouldBeNil)

	// t.Run("Test NextPointCloud", func(t *testing.T) {
	// 	_, err := replayCamera.NextPointCloud(ctx)
	// 	test.That(t, err.Error(), test.ShouldNotBeNil)
	// })

	err = replayCamera.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestConfigValidation(t *testing.T) {
	t.Run("Valid config with source and no timestamp", func(t *testing.T) {
		replayCamCfg := &Config{
			Source:   "test",
			Interval: TimeInterval{},
		}
		deps, err := replayCamCfg.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, deps, test.ShouldResemble, []string{cloud.InternalServiceName.String()})
	})

	t.Run("Valid config with start timestamp", func(t *testing.T) {
		replayCamCfg := &Config{
			Source: "test",
			Interval: TimeInterval{
				Start: "2000-01-01T12:00:00",
			},
		}
		deps, err := replayCamCfg.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, deps, test.ShouldResemble, []string{cloud.InternalServiceName.String()})
	})

	t.Run("Valid config with end timestamp", func(t *testing.T) {
		replayCamCfg := &Config{
			Source: "test",
			Interval: TimeInterval{
				End: "2000-01-01T12:00:00",
			},
		}
		deps, err := replayCamCfg.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, deps, test.ShouldResemble, []string{cloud.InternalServiceName.String()})
	})

	t.Run("Valid config with start and end timestamps", func(t *testing.T) {
		replayCamCfg := &Config{
			Source: "test",
			Interval: TimeInterval{
				Start: "2000-01-01T12:00:00",
				End:   "2000-01-01T12:00:01",
			},
		}
		deps, err := replayCamCfg.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, deps, test.ShouldResemble, []string{cloud.InternalServiceName.String()})
	})

	t.Run("Invalid config no source and no timestamp", func(t *testing.T) {
		replayCamCfg := &Config{}
		deps, err := replayCamCfg.Validate("")
		test.That(t, err, test.ShouldBeError,
			goutils.NewConfigValidationFieldRequiredError("", "source"))
		test.That(t, deps, test.ShouldBeNil)
	})

	t.Run("Invalid config with bad start timestamp", func(t *testing.T) {
		replayCamCfg := &Config{
			Source: "test",
			Interval: TimeInterval{
				Start: "3000-01-01T12:00:00",
			},
		}
		deps, err := replayCamCfg.Validate("")
		test.That(t, err, test.ShouldBeError, errors.New("invalid config, start time must be in the past"))
		test.That(t, deps, test.ShouldBeNil)
	})

	t.Run("Invalid config with bad end timestamp", func(t *testing.T) {
		replayCamCfg := &Config{
			Source: "test",
			Interval: TimeInterval{
				End: "3000-01-01T12:00:00",
			},
		}
		deps, err := replayCamCfg.Validate("")
		test.That(t, err, test.ShouldBeError, errors.New("invalid config, end time must be in the past"))
		test.That(t, deps, test.ShouldBeNil)
	})

	t.Run("Invalid config with start after end timestamps", func(t *testing.T) {
		replayCamCfg := &Config{
			Source: "test",
			Interval: TimeInterval{
				Start: "2000-01-01T12:00:01",
				End:   "2000-01-01T12:00:00",
			},
		}
		deps, err := replayCamCfg.Validate("")

		test.That(t, err, test.ShouldBeError, errors.New("invalid config, end time must be after start time"))
		test.That(t, deps, test.ShouldBeNil)
	})
}

func TestUnimplementedFunctions(t *testing.T) {
	ctx := context.Background()

	replayCamCfg := &Config{Source: "test"}
	replayCamera, err := createNewReplayPCDCamera(t, ctx, replayCamCfg, true)
	test.That(t, err, test.ShouldBeNil)

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
	test.That(t, err, test.ShouldBeNil)
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
