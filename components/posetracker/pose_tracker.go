// Package posetracker contains the interface and gRPC infrastructure
// for a pose tracker component.
package posetracker

import (
	"context"
	"fmt"

	pb "go.viam.com/api/component/posetracker/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[PoseTracker]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterPoseTrackerServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.PoseTrackerService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: doCommand.String(),
	}, newDoCommandCollector)
}

// SubtypeName is a constant that identifies the component resource API string "posetracker".
const SubtypeName = "pose_tracker"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named PoseTracker's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// A PoseTracker represents a robot component that can observe bodies in an
// environment and provide their respective poses in space. These poses are
// given in the context of the PoseTracker's frame of reference.
type PoseTracker interface {
	resource.Resource
	Poses(ctx context.Context, bodyNames []string, extra map[string]interface{}) (referenceframe.FrameSystemPoses, error)
}

// GetResource is a helper for getting the named PoseTracker from either a collection of dependencies
// or the given robot.
func GetResource(src any, name string) (PoseTracker, error) {
	switch v := src.(type) {
	case resource.Dependencies:
		return resource.FromDependencies[PoseTracker](v, Named(name))
	case robot.Robot:
		return robot.ResourceFromRobot[PoseTracker](v, Named(name))
	default:
		return nil, fmt.Errorf("unsupported source type %T", src)
	}
}
