package datamanager_test

import (
	"context"
	"errors"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/datamanager/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func newServer(resourceMap map[resource.Name]datamanager.Service) (pb.DataManagerServiceServer, error) {
	coll, err := resource.NewAPIResourceCollection(datamanager.API, resourceMap)
	if err != nil {
		return nil, err
	}
	return datamanager.NewRPCServiceServer(coll).(pb.DataManagerServiceServer), nil
}

func TestServerSync(t *testing.T) {
	var extraOptions map[string]interface{}

	tests := map[string]struct {
		resourceMap   map[resource.Name]datamanager.Service
		expectedError error
	}{
		"missing datamanager": {
			resourceMap:   map[resource.Name]datamanager.Service{},
			expectedError: errors.New("resource \"rdk:service:data_manager/DataManager1\" not found"),
		},
		"returns error": {
			resourceMap: map[resource.Name]datamanager.Service{
				datamanager.Named(testDataManagerServiceName): &inject.DataManagerService{
					SyncFunc: func(ctx context.Context, extra map[string]interface{}) error {
						return errors.New("fake sync error")
					},
				},
			},
			expectedError: errors.New("fake sync error"),
		},
		"returns response": {
			resourceMap: map[resource.Name]datamanager.Service{
				datamanager.Named(testDataManagerServiceName): &inject.DataManagerService{
					SyncFunc: func(ctx context.Context, extra map[string]interface{}) error {
						extraOptions = extra
						return nil
					},
				},
			},
			expectedError: nil,
		},
	}
	extra := map[string]interface{}{"foo": "Sync"}
	ext, err := protoutils.StructToStructPb(extra)
	test.That(t, err, test.ShouldBeNil)

	syncRequest := &pb.SyncRequest{Name: testDataManagerServiceName, Extra: ext}
	// put resource
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resourceMap := tc.resourceMap
			server, err := newServer(resourceMap)
			test.That(t, err, test.ShouldBeNil)
			_, err = server.Sync(context.Background(), syncRequest)
			if tc.expectedError != nil {
				test.That(t, err, test.ShouldBeError, tc.expectedError)
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, extraOptions, test.ShouldResemble, extra)
			}
		})
	}
}

func TestServerDoCommand(t *testing.T) {
	resourceMap := map[resource.Name]datamanager.Service{
		datamanager.Named(testDataManagerServiceName): &inject.DataManagerService{
			DoCommandFunc: testutils.EchoFunc,
		},
	}
	server, err := newServer(resourceMap)
	test.That(t, err, test.ShouldBeNil)

	cmd, err := protoutils.StructToStructPb(testutils.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	doCommandRequest := &commonpb.DoCommandRequest{
		Name:    testDataManagerServiceName,
		Command: cmd,
	}
	doCommandResponse, err := server.DoCommand(context.Background(), doCommandRequest)
	test.That(t, err, test.ShouldBeNil)

	// Assert that do command response is an echoed request.
	respMap := doCommandResponse.Result.AsMap()
	test.That(t, respMap["command"], test.ShouldResemble, "test")
	test.That(t, respMap["data"], test.ShouldResemble, 500.0)
}
