// Package posetracker contains the interface and gRPC infrastructure
// for a pose tracker component
package posetracker

import (
	"context"

	pb "go.viam.com/api/component/posetracker/v1"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype[PoseTracker]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterPoseTrackerServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.PoseTrackerService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// SubtypeName is a constant that identifies the component resource subtype string "posetracker".
const SubtypeName = resource.SubtypeName("pose_tracker")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named PoseTracker's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// BodyToPoseInFrame represents a map of body names to PoseInFrames.
type BodyToPoseInFrame map[string]*referenceframe.PoseInFrame

// A PoseTracker represents a robot component that can observe bodies in an
// environment and provide their respective poses in space. These poses are
// given in the context of the PoseTracker's frame of reference.
type PoseTracker interface {
	sensor.Sensor
	Poses(ctx context.Context, bodyNames []string, extra map[string]interface{}) (BodyToPoseInFrame, error)
}

// FromRobot is a helper for getting the named force matrix sensor from the given Robot.
func FromRobot(r robot.Robot, name string) (PoseTracker, error) {
	return robot.ResourceFromRobot[PoseTracker](r, Named(name))
}

// Readings is a helper for getting all readings from a PoseTracker.
func Readings(ctx context.Context, poseTracker PoseTracker) (map[string]interface{}, error) {
	poseLookup, err := poseTracker.Poses(ctx, []string{}, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{}
	for bodyName, poseInFrame := range poseLookup {
		result[bodyName] = poseInFrame
	}
	return result, nil
}
