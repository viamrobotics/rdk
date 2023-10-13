// Package slam implements simultaneous localization and mapping.
// This is an Experimental package.
package slam

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/service/slam/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
)

// TBD 05/04/2022: Needs more work once GRPC is included (future PR).
func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterSLAMServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.SLAMService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: position.String(),
	}, newPositionCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: pointCloudMap.String(),
	}, newPointCloudMapCollector)
}

// SubtypeName is the name of the type of service.
const SubtypeName = "slam"

// API is a variable that identifies the slam resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Named is a helper for getting the named service's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromRobot is a helper for getting the named SLAM service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// Service describes the functions that are available to the service.
type Service interface {
	resource.Resource
	Position(ctx context.Context) (spatialmath.Pose, string, error)
	PointCloudMap(ctx context.Context) (func() ([]byte, error), error)
	InternalState(ctx context.Context) (func() ([]byte, error), error)
	LatestMapInfo(ctx context.Context) (time.Time, error)
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

// PointCloudMapFull concatenates the streaming responses from PointCloudMap into a full point cloud.
func PointCloudMapFull(ctx context.Context, slamSvc Service) ([]byte, error) {
	ctx, span := trace.StartSpan(ctx, "slam::PointCloudMapFull")
	defer span.End()
	callback, err := slamSvc.PointCloudMap(ctx)
	if err != nil {
		return nil, err
	}
	return HelperConcatenateChunksToFull(callback)
}

// InternalStateFull concatenates the streaming responses from InternalState into
// the internal serialized state of the slam algorithm.
func InternalStateFull(ctx context.Context, slamSvc Service) ([]byte, error) {
	ctx, span := trace.StartSpan(ctx, "slam::InternalStateFull")
	defer span.End()
	callback, err := slamSvc.InternalState(ctx)
	if err != nil {
		return nil, err
	}
	return HelperConcatenateChunksToFull(callback)
}

// Limits returns the bounds of the slam map as a list of referenceframe.Limits.
func Limits(ctx context.Context, svc Service) ([]referenceframe.Limit, error) {
	data, err := PointCloudMapFull(ctx, svc)
	if err != nil {
		return nil, err
	}
	dims, err := pointcloud.GetPCDMetaData(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return []referenceframe.Limit{
		{Min: dims.MinX, Max: dims.MaxX},
		{Min: dims.MinY, Max: dims.MaxY},
	}, nil
}
