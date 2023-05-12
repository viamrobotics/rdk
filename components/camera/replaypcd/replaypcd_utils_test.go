// Package replay_test will test the  functions of a replay camera.
package replaypcd

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/edaniels/golog"
	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/camera"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/testutils/inject"
)

// mockDataServiceServer is a struct that includes unimplemented versions of all the Data Service endpoints. These
// can be overwritten to allow developers to triggers desired behaviors during testing.
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
	binaryData := &datapb.BinaryData{
		Binary:   dataBuf.Bytes(),
		Metadata: &datapb.BinaryMetadata{},
	}

	resp := &datapb.BinaryDataByFilterResponse{
		Data: []*datapb.BinaryData{binaryData},
		Last: fmt.Sprint(newFileNum),
	}
	return resp, nil
}

// createMockCloudDependencies creates a mockDataServiceServer and rpc client connection to it which is then
// stored in a mockCloudConnectionService.
func createMockCloudDependencies(ctx context.Context, t *testing.T, logger golog.Logger) (resource.Dependencies, func() error) {
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

	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{}
	rs[cloud.InternalServiceName] = &mockCloudConnectionService{
		Named: cloud.InternalServiceName.AsNamed(),
		conn:  conn,
	}

	r.MockResourcesFromMap(rs)

	return resourcesFromDeps(t, r, []string{cloud.InternalServiceName.String()}), rpcServer.Stop
}

// createNewReplayPCDCamera will create a new replay_pcd camera based on the provided config with either
// a valid or invalid data client.
func createNewReplayPCDCamera(ctx context.Context, t *testing.T, replayCamCfg *Config, validDeps bool,
) (camera.Camera, func() error, error) {
	logger := golog.NewTestLogger(t)

	var resources resource.Dependencies
	var closeRPCFunc func() error
	if validDeps {
		resources, closeRPCFunc = createMockCloudDependencies(ctx, t, logger)
	}

	cfg := resource.Config{ConvertedAttributes: replayCamCfg}
	cam, err := newPCDCamera(ctx, resources, cfg, logger)

	return cam, closeRPCFunc, err
}

var _ = cloud.ConnectionService(&mockCloudConnectionService{})

// mockCloudConnectionService creates a mocked version of a cloud connection service.
type mockCloudConnectionService struct {
	resource.Named
	resource.AlwaysRebuild
	conn rpc.ClientConn
}

// AcquireConnection returns a connection to the rpc server stored in the mockCloudConnectionService object.
func (noop *mockCloudConnectionService) AcquireConnection(ctx context.Context) (string, rpc.ClientConn, error) {
	return "", noop.conn, nil
}

// Close is used by the mockCloudConnectionService to complete the cloud connection service interface.
func (noop *mockCloudConnectionService) Close(ctx context.Context) error {
	return nil
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

// getNextDataAfterFilter returns the next point cloud data to be return based on the provided filter and
// last returned item.
func getNextDataAfterFilter(filter *datapb.Filter, last string) (int, error) {
	// Apply the time-based filter based on the second's value in the start and end fields. Because artifacts
	// do not have timestamps associated with them but are numerically ordered we can approximate the filtering
	// by sorting for the files which occur after the start second count and before the end second count.
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

	possibleData := makeFilteredRange(0, numPCDFiles, start, end)

	// Return most recent file after last (should it exist)
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

// makeFilteredRange returns a numerical range of values representing the possible files to be returned
// after filtering.
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
