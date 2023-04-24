package shell_test

import (
	"context"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/shell/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/shell"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func newServer(sMap map[resource.Name]shell.Service) (pb.ShellServiceServer, error) {
	coll, err := resource.NewAPIResourceCollection(shell.API, sMap)
	if err != nil {
		return nil, err
	}
	return shell.NewRPCServiceServer(coll).(pb.ShellServiceServer), nil
}

func TestServerDoCommand(t *testing.T) {
	resourceMap := map[resource.Name]shell.Service{
		testSvcName1: &inject.ShellService{
			DoCommandFunc: testutils.EchoFunc,
		},
	}
	server, err := newServer(resourceMap)
	test.That(t, err, test.ShouldBeNil)

	cmd, err := protoutils.StructToStructPb(testutils.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	doCommandRequest := &commonpb.DoCommandRequest{
		Name:    testSvcName1.ShortName(),
		Command: cmd,
	}
	doCommandResponse, err := server.DoCommand(context.Background(), doCommandRequest)
	test.That(t, err, test.ShouldBeNil)

	// Assert that do command response is an echoed request.
	respMap := doCommandResponse.Result.AsMap()
	test.That(t, respMap["command"], test.ShouldResemble, "test")
	test.That(t, respMap["data"], test.ShouldResemble, 500.0)
}
