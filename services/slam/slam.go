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
	GetPosition(ctx context.Context) (spatialmath.Pose, string, error)
	GetPointCloudMap(ctx context.Context) (func() ([]byte, error), error)
	GetInternalState(ctx context.Context) (func() ([]byte, error), error)
	GetLatestMapInfo(ctx context.Context) (time.Time, error)
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

// GetLimits returns the bounds of the slam map as a list of referenceframe.Limits.
func GetLimits(ctx context.Context, svc Service) ([]referenceframe.Limit, error) {
	data, err := GetPointCloudMapFull(ctx, svc)
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
