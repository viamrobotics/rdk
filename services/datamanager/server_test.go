package datamanager_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	pb "go.viam.com/rdk/proto/api/service/datamanager/v1"
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
	syncRequest := &pb.SyncRequest{}

	omMap := map[resource.Name]interface{}{}
	server, err := newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.Sync(context.Background(), syncRequest)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:data_manager\" not found"))

	// set up the robot with something that is not an datamanager service
	omMap = map[resource.Name]interface{}{datamanager.Name: "not datamanager"}
	server, err = newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.Sync(context.Background(), syncRequest)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("datamanager.Service", "string"))

	// error
	injectMS := &inject.DataManagerService{}
	omMap = map[resource.Name]interface{}{
		datamanager.Name: injectMS,
	}
	server, err = newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	passedErr := errors.New("fake sync error")
	injectMS.SyncFunc = func(
		ctx context.Context,
	) error {
		return passedErr
	}

	_, err = server.Sync(context.Background(), syncRequest)
	test.That(t, err, test.ShouldBeError, passedErr)

	// returns response
	injectMS.SyncFunc = func(
		ctx context.Context,
	) error {
		return nil
	}
	_, err = server.Sync(context.Background(), syncRequest)
	test.That(t, err, test.ShouldBeNil)
}
