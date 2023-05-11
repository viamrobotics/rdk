// Package replay_test will test the  functions of a replay camera.
package replaypcd

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/camera"
	dsmock "go.viam.com/rdk/components/camera/replaypcd/mock_v1"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/testutils/inject"
)

const datasetDirectory = "slam/mock_lidar/%d.pcd"
const numFiles = 15

func getPointCloudFromArtifact(t *testing.T, i int) pointcloud.PointCloud {
	path := filepath.Clean(artifact.MustPath(fmt.Sprintf(datasetDirectory, i)))
	pcdFile, err := os.Open(path)
	defer utils.UncheckedErrorFunc(pcdFile.Close)

	pcExpected, err := pointcloud.ReadPCD(pcdFile)
	test.That(t, err, test.ShouldBeNil)

	return pcExpected
}

func getNoopCloudDependencies(t *testing.T) resource.Dependencies {
	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{}

	rs[cloud.InternalServiceName] = &noopCloudConnectionService{
		Named: cloud.InternalServiceName.AsNamed(),
	}

	r.MockResourcesFromMap(rs)

	return resourcesFromDeps(t, r, []string{cloud.InternalServiceName.String()})
}

func createNewReplayPCDCamera(ctx context.Context, t *testing.T, replayCamCfg *Config, validDeps bool) (camera.Camera, error) {
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

func getNextDataAfterFilter(filter *datapb.Filter, last string) (int, error) {
	start := 0
	end := 1000
	startTime := filter.Interval.Start
	if startTime != nil {
		start = int(startTime.AsTime().Second())
	}
	endTime := filter.Interval.End
	if endTime != nil {
		end = int(endTime.AsTime().Second())
	}

	possibleData := makeFilteredRange(0, numFiles, start, end)

	if last == "" {
		if len(possibleData) != 0 {
			return possibleData[0], nil
		}
	} else {
		lastFileNum, err := strconv.Atoi(last)
		if err != nil {
			return 0, err
		}
		for i := range possibleData {
			if possibleData[i] > lastFileNum {
				return possibleData[i], nil
			}
		}
	}
	return 0, errEndOfDataset
}

func createMockDataServiceClient(t *testing.T) datapb.DataServiceClient {
	ctrl := gomock.NewController(t)
	dataService := dsmock.NewMockDataServiceClient(ctrl)

	dataService.EXPECT().BinaryDataByFilter(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, req *datapb.BinaryDataByFilterRequest, opts ...grpc.CallOption) (*datapb.BinaryDataByFilterResponse, error) {
			// Parse request
			filter := req.DataRequest.GetFilter()
			last := req.DataRequest.GetLast()

			newFileNum, err := getNextDataAfterFilter(filter, last)
			if err != nil {
				return nil, err
			}

			// Get point cloud data
			path := filepath.Clean(artifact.MustPath(fmt.Sprintf(datasetDirectory, newFileNum)))
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}

			var b bytes.Buffer
			gz := gzip.NewWriter(&b)
			gz.Write(data)
			gz.Close()
			dataCompressed := b.Bytes()

			// Construct response
			binaryData := &datapb.BinaryData{
				Binary:   dataCompressed,
				Metadata: &datapb.BinaryMetadata{},
			}

			resp := &datapb.BinaryDataByFilterResponse{
				Data: []*datapb.BinaryData{binaryData},
				Last: fmt.Sprint(newFileNum),
			}
			return resp, nil
		}).AnyTimes()
	return dataService
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
			goutils.NewConfigValidationFieldRequiredError("", "source"))
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
func makeFilteredRange(min, max, start, end int) []int {
	a := []int{}
	for i := 0; i < max-min; i++ {
		val := min + i
		if val >= start && val < end {
			a = append(a, val)
		}
	}
	return a
}
