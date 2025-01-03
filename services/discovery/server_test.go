package discovery_test

import (
	"context"
	"errors"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/discovery/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/discovery"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testDiscoveryName = "discovery1"
	failDiscoveryName = "discovery2"
)

var errDoFailed = errors.New("do failed")
var errDiscoverFailed = errors.New("discover failed")

func newServer() (pb.DiscoveryServiceServer, *inject.DiscoveryService, *inject.DiscoveryService, error) {
	injectDiscovery := &inject.DiscoveryService{}
	injectDiscovery2 := &inject.DiscoveryService{}
	resourceMap := map[resource.Name]discovery.Service{
		discovery.Named(testDiscoveryName): injectDiscovery,
		discovery.Named(failDiscoveryName): injectDiscovery2,
	}
	injectSvc, err := resource.NewAPIResourceCollection(discovery.API, resourceMap)
	if err != nil {
		return nil, nil, nil, err
	}
	return discovery.NewRPCServiceServer(injectSvc).(pb.DiscoveryServiceServer), injectDiscovery, injectDiscovery2, nil
}

func TestDiscoverResources(t *testing.T) {
	discoveryServer, workingDiscovery, failingDiscovery, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	workingDiscovery.DiscoverResourcesFunc = func(ctx context.Context, extra map[string]any) ([]*resource.Config, error) {
		return nil, nil
	}
	failingDiscovery.DiscoverResourcesFunc = func(ctx context.Context, extra map[string]any) ([]*resource.Config, error) {
		return nil, errDiscoverFailed
	}
	resp, err := discoveryServer.DiscoverResources(context.Background(), &pb.DiscoverResourcesRequest{Name: testDiscoveryName})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)

	respFail, err := discoveryServer.DiscoverResources(context.Background(), &pb.DiscoverResourcesRequest{Name: failDiscoveryName})
	test.That(t, err, test.ShouldEqual, errDiscoverFailed)
	test.That(t, respFail, test.ShouldBeNil)
}

func TestDiscoveryDo(t *testing.T) {
	discoveryServer, workingDiscovery, failingDiscovery, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	workingDiscovery.DoFunc = func(
		ctx context.Context,
		cmd map[string]interface{},
	) (
		map[string]interface{},
		error,
	) {
		return cmd, nil
	}
	failingDiscovery.DoFunc = func(
		ctx context.Context,
		cmd map[string]interface{},
	) (
		map[string]interface{},
		error,
	) {
		return nil, errDoFailed
	}

	commandStruct, err := protoutils.StructToStructPb(testutils.TestCommand)
	test.That(t, err, test.ShouldBeNil)

	req := commonpb.DoCommandRequest{Name: testDiscoveryName, Command: commandStruct}
	resp, err := discoveryServer.DoCommand(context.Background(), &req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp.Result.AsMap()["cmd"], test.ShouldEqual, testutils.TestCommand["cmd"])
	test.That(t, resp.Result.AsMap()["data"], test.ShouldEqual, testutils.TestCommand["data"])

	req = commonpb.DoCommandRequest{Name: failDiscoveryName, Command: commandStruct}
	resp, err = discoveryServer.DoCommand(context.Background(), &req)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errDoFailed.Error())
	test.That(t, resp, test.ShouldBeNil)
}
