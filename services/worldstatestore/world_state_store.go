// Package worldstatestore implements the world state store service, which lets users
// create custom visualizers to be rendered in the client.
// For more information, see the [WorldStateStore service docs].
//
// [WorldStateStore service docs]: https://docs.viam.com/dev/reference/apis/services/world-state-store/
package worldstatestore

import (
	"context"
	"errors"
	"io"

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

// Deprecated: FromRobot is a helper for getting the named world state store service from the given Robot.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// Deprecated: FromDependencies is a helper for getting the named world state store service from a collection of
// dependencies. Use FromProvider instead.
//
//nolint:revive // ignore exported comment check.
func FromDependencies(deps resource.Dependencies, name string) (Service, error) {
	return resource.FromDependencies[Service](deps, Named(name))
}

// FromProvider is a helper for getting the named World State Store service
// from a resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (Service, error) {
	return resource.FromProvider[Service](provider, Named(name))
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
// [list uuids method docs]: https://docs.viam.com/dev/reference/apis/services/world-state-store/#listuuids
// [get transform method docs]: https://docs.viam.com/dev/reference/apis/services/world-state-store/#gettransform
// [stream transform changes method docs]: https://docs.viam.com/dev/reference/apis/services/world-state-store/#streamtransformchanges
type Service interface {
	resource.Resource
	ListUUIDs(ctx context.Context, extra map[string]any) ([][]byte, error)
	GetTransform(ctx context.Context, uuid []byte, extra map[string]any) (*commonpb.Transform, error)
	StreamTransformChanges(ctx context.Context, extra map[string]any) (*TransformChangeStream, error)
}

// TransformChange represents a change to a world state transform.
type TransformChange struct {
	ChangeType    pb.TransformChangeType
	Transform     *commonpb.Transform
	UpdatedFields []string
}

// TransformChangeStream provides an iterator interface for receiving transform changes.
// Call Next repeatedly until it returns io.EOF.
type TransformChangeStream struct {
	next func() (TransformChange, error)
}

// Next returns the next TransformChange, or io.EOF when the stream ends.
func (s *TransformChangeStream) Next() (TransformChange, error) {
	if s == nil || s.next == nil {
		return TransformChange{}, io.EOF
	}
	return s.next()
}

// NewTransformChangeStreamFromChannel wraps a channel of TransformChange as a TransformChangeStream.
// The provided context is used to cancel iteration; when ctx is done, Next returns ctx.Err().
func NewTransformChangeStreamFromChannel(ctx context.Context, ch <-chan TransformChange) *TransformChangeStream {
	return &TransformChangeStream{
		next: func() (TransformChange, error) {
			select {
			case <-ctx.Done():
				return TransformChange{}, ctx.Err()
			case change, ok := <-ch:
				if !ok {
					return TransformChange{}, io.EOF
				}
				return change, nil
			}
		},
	}
}
