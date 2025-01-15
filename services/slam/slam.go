// Package slam implements simultaneous localization and mapping.
// This is an Experimental package.
// For more information, see the [SLAM service docs].
//
// [SLAM service docs]: https://docs.viam.com/services/slam/
package slam

import (
	"bytes"
	"context"
	"io"

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
const (
	SubtypeName       = "slam"
	MappingModeNewMap = MappingMode(iota)
	MappingModeLocalizationOnly
	MappingModeUpdateExistingMap
)

func (t MappingMode) String() string {
	switch t {
	case MappingModeNewMap:
		return "mapping mode"
	case MappingModeLocalizationOnly:
		return "localizing only mode"
	case MappingModeUpdateExistingMap:
		return "updating mode"
	default:
		return "unspecified mode"
	}
}

// SensorTypeCamera is a camera sensor.
const (
	SensorTypeCamera = SensorType(iota)
	SensorTypeMovementSensor
)

func (t SensorType) String() string {
	switch t {
	case SensorTypeCamera:
		return "camera"
	case SensorTypeMovementSensor:
		return "movement sensor"
	default:
		return "unsupported sensor type"
	}
}

// API is a variable that identifies the slam resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// MappingMode describes what mapping mode the slam service is in, including
// creating a new map, localizing on an existing map or updating an existing map.
type MappingMode uint8

// SensorType describes what sensor type the sensor is, including
// camera or movement sensor.
type SensorType uint8

// SensorInfo holds information about the sensor name and sensor type.
type SensorInfo struct {
	Name string
	Type SensorType
}

// Properties returns various information regarding the current slam service,
// including whether the slam process is running in the cloud and its mapping mode.
type Properties struct {
	CloudSlam             bool
	MappingMode           MappingMode
	InternalStateFileType string
	SensorInfo            []SensorInfo
}

// Named is a helper for getting the named service's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromRobot is a helper for getting the named SLAM service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// FromDependencies is a helper for getting the named SLAM service from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Service, error) {
	return resource.FromDependencies[Service](deps, Named(name))
}

// Service describes the functions that are available to the service.
//
// The Go SDK implements helper functions that concatenate streaming
// responses. Some of the following examples use corresponding
// helper methods instead of interface methods.
// For more information, see the [SLAM service docs].
//
// Position example:
//
//	// Get the current position of the specified source component
//	// in the SLAM map as a Pose.
//	pos, name, err := mySLAMService.Position(context.Background())
//
// For more information, see the [Position method docs].
//
// PointCloudMap example (using PointCloudMapFull helper method):
//
//	// Get the point cloud map in standard PCD format.
//	pcdMapBytes, err := PointCloudMapFull(
//	    context.Background(), mySLAMService, true)
//
// For more information, see the [PointCloudMap method docs].
//
// InternalState example (using InternalStateFull helper method):
//
//	// Get the internal state of the SLAM algorithm required
//	// to continue mapping/localization.
//	internalStateBytes, err := InternalStateFull(
//	    context.Background(), mySLAMService)
//
// For more information, see the [InternalState method docs].
//
// Properties example:
//
//	// Get the properties of your current SLAM session
//	properties, err := mySLAMService.Properties(context.Background())
//
// For more information, see the [Properties method docs].
//
// [SLAM service docs]: https://docs.viam.com/operate/reference/services/slam/
// [Position method docs]: https://docs.viam.com/dev/reference/apis/services/slam/#getposition
// [PointCloudMap method docs]: https://docs.viam.com/dev/reference/apis/services/slam/#getpointcloudmap
// [InternalState method docs]: https://docs.viam.com/dev/reference/apis/services/slam/#getinternalstate
// [Properties method docs]: https://docs.viam.com/dev/reference/apis/services/slam/#getproperties
type Service interface {
	resource.Resource
	Position(ctx context.Context) (spatialmath.Pose, error)
	PointCloudMap(ctx context.Context, returnEditedMap bool) (func() ([]byte, error), error)
	InternalState(ctx context.Context) (func() ([]byte, error), error)
	Properties(ctx context.Context) (Properties, error)
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
func PointCloudMapFull(ctx context.Context, slamSvc Service, returnEditedMap bool) ([]byte, error) {
	ctx, span := trace.StartSpan(ctx, "slam::PointCloudMapFull")
	defer span.End()
	callback, err := slamSvc.PointCloudMap(ctx, returnEditedMap)
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
func Limits(ctx context.Context, svc Service, useEditedMap bool) ([]referenceframe.Limit, error) {
	data, err := PointCloudMapFull(ctx, svc, useEditedMap)
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
