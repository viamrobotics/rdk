package generic_test

import (
	"context"
	"errors"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	genericpb "go.viam.com/api/component/generic/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var errDoFailed = errors.New("do failed")

func newServer() (genericpb.GenericServiceServer, *inject.GenericComponent, *inject.GenericComponent, error) {
	injectGeneric := &inject.GenericComponent{}
	injectGeneric2 := &inject.GenericComponent{}
	resourceMap := map[resource.Name]resource.Resource{
		generic.Named(testGenericName): injectGeneric,
		generic.Named(failGenericName): injectGeneric2,
	}
	injectSvc, err := resource.NewAPIResourceCollection(generic.API, resourceMap)
	if err != nil {
		return nil, nil, nil, err
	}
	return generic.NewRPCServiceServer(injectSvc).(genericpb.GenericServiceServer), injectGeneric, injectGeneric2, nil
}

func TestGenericDo(t *testing.T) {
	genericServer, workingGeneric, failingGeneric, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	workingGeneric.DoFunc = func(
		ctx context.Context,
		cmd map[string]interface{},
	) (
		map[string]interface{},
		error,
	) {
		return cmd, nil
	}
	failingGeneric.DoFunc = func(
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

	req := commonpb.DoCommandRequest{Name: testGenericName, Command: commandStruct}
	resp, err := genericServer.DoCommand(context.Background(), &req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp.Result.AsMap()["cmd"], test.ShouldEqual, testutils.TestCommand["cmd"])
	test.That(t, resp.Result.AsMap()["data"], test.ShouldEqual, testutils.TestCommand["data"])

	req = commonpb.DoCommandRequest{Name: failGenericName, Command: commandStruct}
	resp, err = genericServer.DoCommand(context.Background(), &req)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errDoFailed.Error())
	test.That(t, resp, test.ShouldBeNil)
}
