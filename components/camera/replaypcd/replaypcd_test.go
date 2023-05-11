// Package replay_test will test the  functions of a replay camera.
package replaypcd

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/internal/cloud"
)

const (
	datasetDirectory = "slam/mock_lidar/%d.pcd"
	numFiles         = 15
)

func TestNewReplayPCD(t *testing.T) {
	ctx := context.Background()

	t.Run("valid config with internal cloud service", func(t *testing.T) {
		replayCamCfg := &Config{Source: "source"}
		replayCamera, err := createNewReplayPCDCamera(ctx, t, replayCamCfg, true)
		test.That(t, err, test.ShouldBeNil)

		err = replayCamera.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("no internal cloud service", func(t *testing.T) {
		replayCamCfg := &Config{Source: "source"}
		replayCamera, err := createNewReplayPCDCamera(ctx, t, replayCamCfg, false)
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
		replayCamera, err := createNewReplayPCDCamera(ctx, t, replayCamCfg, true)
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
		replayCamera, err := createNewReplayPCDCamera(ctx, t, replayCamCfg, true)
		test.That(t, err, test.ShouldBeError, errors.New("invalid time format, use RFC3339"))
		test.That(t, replayCamera, test.ShouldBeNil)
	})
}

func TestNextPointCloud(t *testing.T) {
	t.Run("Calling NextPointCloud no filter", func(t *testing.T) {
		ctx := context.Background()

		replayCamCfg := &Config{Source: "test"}
		replayCamera, err := createNewReplayPCDCamera(ctx, t, replayCamCfg, true)
		test.That(t, err, test.ShouldBeNil)

		rr := replayCamera.(*pcdCamera)
		rr.dataClient = createMockDataServiceClient(t)

		for i := 0; i < numFiles; i++ {
			pc, err := rr.NextPointCloud(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pc, test.ShouldResemble, getPointCloudFromArtifact(t, i))
		}

		pc, err := rr.NextPointCloud(ctx)
		test.That(t, err, test.ShouldBeError, errEndOfDataset)
		test.That(t, pc, test.ShouldBeNil)

		err = rr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("Calling NextPointCloud with filter no data", func(t *testing.T) {
		ctx := context.Background()

		replayCamCfg := &Config{
			Source: "test",
			Interval: TimeInterval{
				Start: "2000-01-01T12:00:30Z",
				End:   "2000-01-01T12:00:40Z",
			},
		}

		replayCamera, err := createNewReplayPCDCamera(ctx, t, replayCamCfg, true)
		test.That(t, err, test.ShouldBeNil)

		rr := replayCamera.(*pcdCamera)
		rr.dataClient = createMockDataServiceClient(t)

		pc, err := rr.NextPointCloud(ctx)
		test.That(t, err, test.ShouldBeError, errEndOfDataset)
		test.That(t, pc, test.ShouldBeNil)

		err = rr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("Calling NextPointCloud with start and end filter", func(t *testing.T) {
		ctx := context.Background()

		replayCamCfg := &Config{
			Source: "test",
			Interval: TimeInterval{
				Start: "2000-01-01T12:00:05Z",
				End:   "2000-01-01T12:00:10Z",
			},
		}

		replayCamera, err := createNewReplayPCDCamera(ctx, t, replayCamCfg, true)
		test.That(t, err, test.ShouldBeNil)

		rr := replayCamera.(*pcdCamera)
		rr.dataClient = createMockDataServiceClient(t)

		startTime, err := time.Parse(timeFormat, replayCamCfg.Interval.Start)
		test.That(t, err, test.ShouldBeNil)
		endTime, err := time.Parse(timeFormat, replayCamCfg.Interval.End)
		test.That(t, err, test.ShouldBeNil)

		for i := startTime.Second(); i < endTime.Second(); i++ {
			pc, err := rr.NextPointCloud(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pc, test.ShouldResemble, getPointCloudFromArtifact(t, i))
		}

		pc, err := rr.NextPointCloud(ctx)
		test.That(t, err, test.ShouldBeError, errEndOfDataset)
		test.That(t, pc, test.ShouldBeNil)

		err = rr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("Calling NextPointCloud with end filter", func(t *testing.T) {
		ctx := context.Background()

		replayCamCfg := &Config{
			Source: "test",
			Interval: TimeInterval{
				End: "2000-01-01T12:00:10Z",
			},
		}

		replayCamera, err := createNewReplayPCDCamera(ctx, t, replayCamCfg, true)
		test.That(t, err, test.ShouldBeNil)

		rr := replayCamera.(*pcdCamera)
		rr.dataClient = createMockDataServiceClient(t)

		endTime, err := time.Parse(timeFormat, replayCamCfg.Interval.End)
		test.That(t, err, test.ShouldBeNil)

		for i := 0; i < endTime.Second(); i++ {
			pc, err := rr.NextPointCloud(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pc, test.ShouldResemble, getPointCloudFromArtifact(t, i))
		}

		pc, err := rr.NextPointCloud(ctx)
		test.That(t, err, test.ShouldBeError, errEndOfDataset)
		test.That(t, pc, test.ShouldBeNil)

		err = rr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("Calling NextPointCloud with start filter", func(t *testing.T) {
		ctx := context.Background()

		replayCamCfg := &Config{
			Source: "test",
			Interval: TimeInterval{
				Start: "2000-01-01T12:00:05Z",
			},
		}

		replayCamera, err := createNewReplayPCDCamera(ctx, t, replayCamCfg, true)
		test.That(t, err, test.ShouldBeNil)

		rr := replayCamera.(*pcdCamera)
		rr.dataClient = createMockDataServiceClient(t)

		startTime, err := time.Parse(timeFormat, replayCamCfg.Interval.Start)
		test.That(t, err, test.ShouldBeNil)

		for i := startTime.Second(); i < numFiles; i++ {
			pc, err := rr.NextPointCloud(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pc, test.ShouldResemble, getPointCloudFromArtifact(t, i))
		}

		pc, err := rr.NextPointCloud(ctx)
		test.That(t, err, test.ShouldBeError, errEndOfDataset)
		test.That(t, pc, test.ShouldBeNil)

		err = rr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})
}

// func Test2NextPointCloud(t *testing.T) {
// 	ctx := context.Background()

// 	replayCamCfg := &Config{Source: "test"}
// 	replayCamera, err := createNewReplayPCDCamera(ctx, t, replayCamCfg, true)
// 	test.That(t, err, test.ShouldBeNil)

// 	t.Run("Calling NextPointCloud", func(t *testing.T) {
// 		i := 0
// 		path := filepath.Clean(artifact.MustPath(fmt.Sprintf(datasetDirectory, i)))
// 		pcdFile, err := os.Open(path)
// 		defer utils.UncheckedErrorFunc(pcdFile.Close)
// 		pcExpected, err := pointcloud.ReadPCD(pcdFile)
// 		test.That(t, err, test.ShouldBeNil)

// 		pc, err := replayCamera.NextPointCloud(ctx)
// 		test.That(t, err, test.ShouldBeNil)
// 		test.That(t, pcExpected, test.ShouldResemble, pc)
// 	})

// 	err = replayCamera.Close(ctx)
// 	test.That(t, err, test.ShouldBeNil)
// }

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
				Start: "2000-01-01T12:00:00Z",
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
				End: "2000-01-01T12:00:00Z",
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
				Start: "2000-01-01T12:00:00Z",
				End:   "2000-01-01T12:00:01Z",
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
			utils.NewConfigValidationFieldRequiredError("", "source"))
		test.That(t, deps, test.ShouldBeNil)
	})

	t.Run("Invalid config with bad start timestamp", func(t *testing.T) {
		replayCamCfg := &Config{
			Source: "test",
			Interval: TimeInterval{
				Start: "3000-01-01T12:00:00Z",
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
				End: "3000-01-01T12:00:00Z",
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
				Start: "2000-01-01T12:00:01Z",
				End:   "2000-01-01T12:00:00Z",
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
	replayCamera, err := createNewReplayPCDCamera(ctx, t, replayCamCfg, true)
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
