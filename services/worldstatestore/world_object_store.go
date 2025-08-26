// Package worldstatestore implements the world obejct store service, which lets users
// create custom visualizers to be rendered in the client.
// For more information, see the [WorldObjectStore service docs].
//
// [WorldStateStore service docs]: https://docs.viam.com/dev/reference/apis/services/world-state-store/
package worldstatestore

import (
	"context"
	"errors"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterWorldStateStoreServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.WorldStateStoreService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

const (
	// SubtypeName is the name of the type of service.
	SubtypeName = "world_state_store"
)

// API is a variable that identifies the world state store resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// ErrNilResponse is the error for when a nil response is returned from a world object store service.
var ErrNilResponse = errors.New("world state store service returned a nil response")

// Named is a helper for getting the named service's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromRobot is a helper for getting the named world state store service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// FromDependencies is a helper for getting the named world state store service from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Service, error) {
	return resource.FromDependencies[Service](deps, Named(name))
}

// Service describes the functions that are available to the service.
//
// For more information, see the [WorldStateStore service docs].
//
// ListUUIDs example:
//
//	// List the world state uuids of a WorldStateStore Service.
//	uuids, err := myWorldStateStoreService.ListUUIDs(ctx, nil)
//	if err != nil {
//		logger.Fatal(err)
//	}
//	// Print out the world state
//	for _, uuid := range uuids {
//		fmt.Printf("UUID: %v", uuid)
//	}
//
// For more information, see the [list uuids method docs].
//
// GetTransform example:
//
//	// Get the transform by uuid.
//	obj, err := myWorldStateStoreService.GetTransform(ctx, myUUID, nil)
//	if err != nil {
//		logger.Fatal(err)
//	}
//	// Print out the transform.
//	fmt.Printf("Name: %v\nPose: %+v\nMetadata: %+v\nGeometry: %+v", obj.Name, obj.Pose, obj.Metadata, obj.Geometry)
//
// For more information, see the [get transform method docs].
//
// StreamTransformChanges example:
//
//	// Stream transform changes.
//	changes, err := myWorldStateStoreService.StreamTransformChanges(ctx, nil)
//	if err != nil {
//		logger.Fatal(err)
//	}
//	for change := range changes {
//		fmt.Printf("Change: %v\n", change)
//	}
//
// For more information, see the [stream transform changes method docs].
//
// [WorldStateStore service docs]: https://docs.viam.com/dev/reference/apis/services/world-state-store/
// [list uuids method docs]: https://docs.viam.com/dev/reference/apis/services/list-uuids/
// [get transform method docs]: https://docs.viam.com/dev/reference/apis/services/get-transform/
// [stream transform changes method docs]: https://docs.viam.com/dev/reference/apis/services/stream-transform-changes/
type Service interface {
	resource.Resource
	ListUUIDs(ctx context.Context, extra map[string]any) ([][]byte, error)
	GetTransform(ctx context.Context, uuid []byte, extra map[string]any) (*commonpb.Transform, error)
	StreamTransformChanges(ctx context.Context, extra map[string]any) (<-chan TransformChange, error)
}

// TransformChange represents a change to a world state transform.
type TransformChange struct {
	ChangeType    pb.TransformChangeType
	Transform     *commonpb.Transform
	UpdatedFields []string
}
