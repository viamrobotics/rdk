package generic_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/generic"
	pb "go.viam.com/rdk/proto/api/component/generic/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"

	"google.golang.org/protobuf/types/known/structpb"
)

func newServer() (pb.GenericServiceServer, *inject.Generic, *inject.Generic, error) {
	injectGeneric := &inject.Generic{}
	injectGeneric2 := &inject.Generic{}
	resourceMap := map[resource.Name]interface{}{
		generic.Named(testGenericName):   injectGeneric,
		generic.Named(failGenericName):   injectGeneric2,
		generic.Named((fakeGenericName)): "not a generic",
	}
	injectSvc, err := subtype.New(resourceMap)
	if err != nil {
		return nil, nil, nil, err
	}
	return generic.NewServer(injectSvc), injectGeneric, injectGeneric2, nil
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
		return nil, errors.New("do failed")
	}

	commandStruct, err := structpb.NewStruct(command)
	test.That(t, err, test.ShouldBeNil)

	req := pb.DoRequest{Name: testGenericName, Command: commandStruct}
	resp, err := genericServer.Do(context.Background(), &req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp.Result.AsMap()["cmd"], test.ShouldEqual, command["cmd"])
	test.That(t, resp.Result.AsMap()["data1"], test.ShouldEqual, command["data1"])

	req = pb.DoRequest{Name: failGenericName, Command: commandStruct}
	resp, err = genericServer.Do(context.Background(), &req)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, resp, test.ShouldBeNil)

	req = pb.DoRequest{Name: fakeGenericName, Command: commandStruct}
	resp, err = genericServer.Do(context.Background(), &req)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, resp, test.ShouldBeNil)
}
