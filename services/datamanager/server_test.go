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

func newServer(resourceMap map[resource.Name]interface{}) (pb.DataManagerServiceServer, error) {
	omSvc, err := subtype.New(resourceMap)
	if err != nil {
		return nil, err
	}
	return datamanager.NewServer(omSvc), nil
}

func TestServerSync(t *testing.T) {
	tests := map[string]struct {
		resourceMap   map[resource.Name]interface{}
		expectedError error
	}{
		"missing datamanager": {
			resourceMap:   map[resource.Name]interface{}{},
			expectedError: errors.New("resource \"rdk:service:data_manager\" not found"),
		},
		"not datamanager": {
			resourceMap: map[resource.Name]interface{}{
				datamanager.Name: "not datamanager",
			},
			expectedError: rutils.NewUnimplementedInterfaceError("datamanager.Service", "string"),
		},
		"returns error": {
			resourceMap: map[resource.Name]interface{}{
				datamanager.Name: &inject.DataManagerService{
					SyncFunc: func(
						ctx context.Context,
					) error {
						return errors.New("fake datasync error")
					},
				},
			},
			expectedError: errors.New("fake datasync error"),
		},
		"returns response": {
			resourceMap: map[resource.Name]interface{}{
				datamanager.Name: &inject.DataManagerService{
					SyncFunc: func(
						ctx context.Context,
					) error {
						return nil
					},
				},
			},
			expectedError: nil,
		},
	}

	syncRequest := &pb.SyncRequest{}
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
			}
		})
	}
}
