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

	"github.com/edaniels/golog"
	"github.com/golang/mock/gomock"
	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
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

func getPointCloudFromArtifact(t *testing.T, i int) pointcloud.PointCloud {
	path := filepath.Clean(artifact.MustPath(fmt.Sprintf(datasetDirectory, i)))
	pcdFile, err := os.Open(path)
	test.That(t, err, test.ShouldBeNil)
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

func getNextDataAfterFilter(filter *datapb.Filter, last string) (int, error) {
	start := 0
	end := 1000
	startTime := filter.Interval.Start
	if startTime != nil {
		start = startTime.AsTime().Second()
	}
	endTime := filter.Interval.End
	if endTime != nil {
		end = endTime.AsTime().Second()
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
