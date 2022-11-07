package datamanager_test

import (
	"context"
	"errors"
	"testing"

	pb "go.viam.com/api/service/datamanager/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func newServer(resourceMap map[resource.Name]interface{}) (pb.DataManagerServiceServer, error) {
	dmSvc, err := subtype.New(resourceMap)
	if err != nil {
		return nil, err
	}
	return datamanager.NewServer(dmSvc), nil
}

func TestServerSync(t *testing.T) {
	var extraOptions map[string]interface{}

	tests := map[string]struct {
		resourceMap   map[resource.Name]interface{}
		expectedError error
	}{
		"missing datamanager": {
			resourceMap:   map[resource.Name]interface{}{},
			expectedError: errors.New("resource \"rdk:service:data_manager/DataManager1\" not found"),
		},
		"not datamanager": {
			resourceMap: map[resource.Name]interface{}{
				datamanager.Named(testDataManagerServiceName): "not datamanager",
			},
			expectedError: datamanager.NewUnimplementedInterfaceError("string"),
		},
		"returns error": {
			resourceMap: map[resource.Name]interface{}{
				datamanager.Named(testDataManagerServiceName): &inject.DataManagerService{
					SyncFunc: func(ctx context.Context, extra map[string]interface{}) error {
						return errors.New("fake sync error")
					},
				},
			},
			expectedError: errors.New("fake sync error"),
		},
		"returns response": {
			resourceMap: map[resource.Name]interface{}{
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
