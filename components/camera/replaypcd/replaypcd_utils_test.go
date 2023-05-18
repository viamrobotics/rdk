package replaypcd

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/camera"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/internal/cloud"
	cloudinject "go.viam.com/rdk/internal/testutils/inject"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/testutils/inject"
)

var testTime string = "2000-01-01T12:00:%02dZ"

// mockDataServiceServer is a struct that includes unimplemented versions of all the Data Service endpoints. These
// can be overwritten to allow developers to trigger desired behaviors during testing.
type mockDataServiceServer struct {
	datapb.UnimplementedDataServiceServer
}

// BinaryDataByFilter is a mocked version of the Data Service function of a similar name. It returns a response with
// data corresponding to a stored pcd artifact based on the filter and last file accessed.
func (mDServer *mockDataServiceServer) BinaryDataByFilter(ctx context.Context, req *datapb.BinaryDataByFilterRequest,
) (*datapb.BinaryDataByFilterResponse, error) {
	// Parse request
	filter := req.DataRequest.GetFilter()
	last := req.DataRequest.GetLast()

	newFileNum, err := getNextDataAfterFilter(filter, last)
	if err != nil {
		return nil, err
	}

	// Get point cloud data in gzip compressed format
	path := filepath.Clean(artifact.MustPath(fmt.Sprintf(datasetDirectory, newFileNum)))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var dataBuf bytes.Buffer
	gz := gzip.NewWriter(&dataBuf)
	gz.Write(data)
	gz.Close()

	// Construct response
	timeReq, err := time.Parse(time.RFC3339, fmt.Sprintf(testTime, newFileNum))
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing time")
	}
	timeRec := timeReq.Add(time.Second)
	binaryData := &datapb.BinaryData{
		Binary: dataBuf.Bytes(),
		Metadata: &datapb.BinaryMetadata{
			TimeRequested: timestamppb.New(timeReq),
			TimeReceived:  timestamppb.New(timeRec),
		},
	}

	resp := &datapb.BinaryDataByFilterResponse{
		Data: []*datapb.BinaryData{binaryData},
		Last: fmt.Sprint(newFileNum),
	}
	return resp, nil
}

// createMockCloudDependencies creates a mockDataServiceServer and rpc client connection to it which is then
// stored in a mockCloudConnectionService.
func createMockCloudDependencies(ctx context.Context, t *testing.T, logger golog.Logger, b bool) (resource.Dependencies, func() error) {
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	test.That(t, rpcServer.RegisterServiceServer(
		ctx,
		&datapb.DataService_ServiceDesc,
		&mockDataServiceServer{},
		datapb.RegisterDataServiceHandlerFromEndpoint,
	), test.ShouldBeNil)

	go rpcServer.Serve(listener)

	conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	mockCloudConnectionService := &cloudinject.CloudConnectionService{
		Named: cloud.InternalServiceName.AsNamed(),
		Conn:  conn,
	}
	if !b {
		mockCloudConnectionService.AcquireConnectionErr = errors.New("cloud connection error")
	}

	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{}
	rs[cloud.InternalServiceName] = mockCloudConnectionService
	r.MockResourcesFromMap(rs)

	return resourcesFromDeps(t, r, []string{cloud.InternalServiceName.String()}), rpcServer.Stop
}

// createNewReplayPCDCamera will create a new replay_pcd camera based on the provided config with either
// a valid or invalid data client.
func createNewReplayPCDCamera(ctx context.Context, t *testing.T, replayCamCfg *Config, validDeps bool,
) (camera.Camera, func() error, error) {
	logger := golog.NewTestLogger(t)

	resources, closeRPCFunc := createMockCloudDependencies(ctx, t, logger, validDeps)

	cfg := resource.Config{ConvertedAttributes: replayCamCfg}
	cam, err := newPCDCamera(ctx, resources, cfg, logger)

	return cam, closeRPCFunc, err
}

// resourcesFromDeps returns a list of dependencies from the provided robot.
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

// getNextDataAfterFilter returns the artifact index of the next point cloud data to be return based on
// the provided filter and last returned artifact.
func getNextDataAfterFilter(filter *datapb.Filter, last string) (int, error) {
	// Basic component part (source) filter
	if filter.ComponentName != "" && filter.ComponentName != "source" {
		return 0, errEndOfDataset
	}

	// Basic robot_id filter
	if filter.RobotId != "" && filter.RobotId != "robot_id" {
		return 0, errEndOfDataset
	}

	// Apply the time-based filter based on the seconds value in the start and end fields. Because artifacts
	// do not have timestamps associated with them but are numerically ordered we can approximate the filtering
	// by sorting for the files which occur after the start second count and before the end second count.
	// For example, if there are 15 files in the artifact directory, the start time is 2000-01-01T12:00:10Z
	// and the end time is 2000-01-01T12:00:14Z, we will return files 10-14.
	start := 0
	end := numPCDFiles
	if filter.Interval.Start != nil {
		start = filter.Interval.Start.AsTime().Second()
	}
	if filter.Interval.End != nil {
		end = int(math.Min(float64(filter.Interval.End.AsTime().Second()), float64(end)))
	}

	if last == "" {
		return getFile(start, end)
	}
	lastFileNum, err := strconv.Atoi(last)
	if err != nil {
		return 0, err
	}
	return getFile(lastFileNum+1, end)
}

// getFile will return the next file to be returned after checking it satisfies the end condition.
func getFile(i, end int) (int, error) {
	if i < end {
		return i, nil
	}
	return 0, errEndOfDataset
}
