package datamanager_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/gripper"
	pb "go.viam.com/rdk/proto/api/service/datamanager/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func newServer(omMap map[resource.Name]interface{}) (pb.DataManagerServiceServer, error) {
	omSvc, err := subtype.New(omMap)
	if err != nil {
		return nil, err
	}
	return datamanager.NewServer(omSvc), nil
}

func TestServerSync(t *testing.T) {
	syncRequest := &pb.SyncRequest{
		Name: protoutils.ResourceNameToProto(gripper.Named("fake")),
	}

	omMap := map[resource.Name]interface{}{}
	server, err := newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.Sync(context.Background(), syncRequest)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:motion\" not found"))

	// set up the robot with something that is not an motion service
	omMap = map[resource.Name]interface{}{datamanager.Name: "not motion"}
	server, err = newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.Sync(context.Background(), syncRequest)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("motion.Service", "string"))

	// error
	injectMS := &inject.DataManagerService{}
	omMap = map[resource.Name]interface{}{
		datamanager.Name: injectMS,
	}
	server, err = newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	passedErr := errors.New("fake move error")
	injectMS.SyncFunc = func(
		ctx context.Context,
		componentName resource.Name,
	) (bool, error) {
		return false, passedErr
	}

	_, err = server.Sync(context.Background(), syncRequest)
	test.That(t, err, test.ShouldBeError, passedErr)

	// returns response
	injectMS.SyncFunc = func(
		ctx context.Context,
		componentName resource.Name,
	) (bool, error) {
		return true, nil
	}
	resp, err := server.Sync(context.Background(), syncRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetSuccess(), test.ShouldBeTrue)
}
