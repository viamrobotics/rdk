// Package slam implements simultaneous localization and mapping.
// This is an Experimental package.
package slam

import (
	"context"
	"io"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/service/slam/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
)

// TBD 05/04/2022: Needs more work once GRPC is included (future PR).
func init() {
	resource.RegisterSubtype(Subtype, resource.SubtypeRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterSLAMServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.SLAMService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name resource.Name, logger golog.Logger) (Service, error) {
			return NewClientFromConn(ctx, conn, name, logger), nil
		},
	})
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("slam")

// Subtype is a constant that identifies the slam resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named service's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// FromRobot is a helper for getting the named SLAM service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// Service describes the functions that are available to the service.
type Service interface {
	resource.Resource
	GetPosition(context.Context) (spatialmath.Pose, string, error)
	GetPointCloudMap(ctx context.Context) (func() ([]byte, error), error)
	GetInternalState(ctx context.Context) (func() ([]byte, error), error)
}

// HelperConcatenateChunksToFull concatenates the chunks from a streamed grpc endpoint.
func HelperConcatenateChunksToFull(f func() ([]byte, error)) ([]byte, error) {
	var fullBytes []byte
	for {
		chunk, err := f()
		if errors.Is(err, io.EOF) {
			return fullBytes, nil
		}
		if err != nil {
			return nil, err
		}

		fullBytes = append(fullBytes, chunk...)
	}
}

// GetPointCloudMapFull concatenates the streaming responses from GetPointCloudMap into a full point cloud.
func GetPointCloudMapFull(ctx context.Context, slamSvc Service) ([]byte, error) {
	ctx, span := trace.StartSpan(ctx, "slam::GetPointCloudMapFull")
	defer span.End()
	callback, err := slamSvc.GetPointCloudMap(ctx)
	if err != nil {
		return nil, err
	}
	return HelperConcatenateChunksToFull(callback)
}

// GetInternalStateFull concatenates the streaming responses from GetInternalState into
// the internal serialized state of the slam algorithm.
func GetInternalStateFull(ctx context.Context, slamSvc Service) ([]byte, error) {
	ctx, span := trace.StartSpan(ctx, "slam::GetInternalStateFull")
	defer span.End()
	callback, err := slamSvc.GetInternalState(ctx)
	if err != nil {
		return nil, err
	}
	return HelperConcatenateChunksToFull(callback)
}
