// Package robot defines the robot which is the root of all robotic parts.
package robot

import (
	"context"
	"fmt"
	"strings"

	"github.com/edaniels/golog"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/pkg/errors"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/utils"
)

// NewUnimplementedLocalInterfaceError is used when there is a failed interface check.
func NewUnimplementedLocalInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((*LocalRobot)(nil), actual)
}

// A Robot encompasses all functionality of some robot comprised
// of parts, local and remote.
type Robot interface {
	// DiscoverComponents returns discovered component configurations.
	DiscoverComponents(ctx context.Context, qs []discovery.Query) ([]discovery.Discovery, error)

	// RemoteByName returns a remote robot by name.
	RemoteByName(name string) (Robot, bool)

	// ResourceByName returns a resource by name
	ResourceByName(name resource.Name) (interface{}, error)

	// RemoteNames returns the name of all known remote robots.
	RemoteNames() []string

	// ResourceNames returns a list of all known resource names.
	ResourceNames() []resource.Name

	// ResourceRPCSubtypes returns a list of all known resource RPC subtypes.
	ResourceRPCSubtypes() []resource.RPCSubtype

	// ProcessManager returns the process manager for the robot.
	ProcessManager() pexec.ProcessManager

	// OperationManager returns the operation manager the robot is using.
	OperationManager() *operation.Manager

	// SessionManager returns the session manager the robot is using.
	SessionManager() session.Manager

	// Logger returns the logger the robot is using.
	Logger() golog.Logger

	// FrameSystemConfig returns the individual parts that make up a robot's frame system
	FrameSystemConfig(ctx context.Context, additionalTransforms []*referenceframe.LinkInFrame) (framesystemparts.Parts, error)

	// TransformPose will transform the pose of the requested poseInFrame to the desired frame in the robot's frame system.
	TransformPose(
		ctx context.Context,
		pose *referenceframe.PoseInFrame,
		dst string,
		additionalTransforms []*referenceframe.LinkInFrame,
	) (*referenceframe.PoseInFrame, error)

	// Status takes a list of resource names and returns their corresponding statuses. If no names are passed in, return all statuses.
	Status(ctx context.Context, resourceNames []resource.Name) ([]Status, error)

	// Close attempts to cleanly close down all constituent parts of the robot.
	Close(ctx context.Context) error

	// StopAll cancels all current and outstanding operations for the robot and stops all actuators and movement
	StopAll(ctx context.Context, extra map[resource.Name]map[string]interface{}) error
}

// A Refresher can refresh the contents of a robot.
type Refresher interface {
	// Refresh instructs the Robot to manually refresh the contents of itself.
	Refresh(ctx context.Context) error
}

// A LocalRobot is a Robot that can have its parts modified.
type LocalRobot interface {
	Robot

	// Config returns the local config used to construct the robot.
	// This is allowed to be partial or empty.
	Config(ctx context.Context) (*config.Config, error)

	// Reconfigure instructs the robot to safely reconfigure itself based
	// on the given new config.
	Reconfigure(ctx context.Context, newConfig *config.Config)

	// StartWeb starts the web server, will return an error if server is already up.
	StartWeb(ctx context.Context, o weboptions.Options) error

	// StopWeb stops the web server, will be a noop if server is not up.
	StopWeb() error
}

// A RemoteRobot is a Robot that was created through a connection.
type RemoteRobot interface {
	Robot

	// Connected returns whether the remote is connected or not.
	Connected() bool
}

// Status holds a resource name and its corresponding status. Status is expected to be comprised of string keys
// and values comprised of primitives, list of primitives, maps with string keys (or at least can be decomposed into one),
// or lists of the forementioned type of maps. Results with other types of data are not guaranteed.
type Status struct {
	Name   resource.Name
	Status interface{}
}

// AllResourcesByName returns an array of all resources that have this simple name.
func AllResourcesByName(r Robot, name string) []interface{} {
	all := []interface{}{}

	for _, n := range r.ResourceNames() {
		if n.ShortName() == name {
			r, err := r.ResourceByName(n)
			if err != nil {
				panic("this should be impossible")
			}
			all = append(all, r)
		}
	}

	return all
}

// NamesBySubtype is a helper for getting all names from the given Robot given the subtype.
func NamesBySubtype(r Robot, subtype resource.Subtype) []string {
	names := []string{}
	for _, n := range r.ResourceNames() {
		if n.Subtype == subtype {
			names = append(names, n.ShortName())
		}
	}
	return names
}

// TypeAndMethodDescFromMethod attempts to determine the resource subtype and its respective gRPC method information
// from the given robot and method path. If nothing can be found, grpc.UnimplementedError is returned.
func TypeAndMethodDescFromMethod(r Robot, method string) (*resource.RPCSubtype, *desc.MethodDescriptor, error) {
	methodParts := strings.Split(method, "/")
	if len(methodParts) != 3 {
		return nil, nil, grpc.UnimplementedError
	}
	protoSvc := methodParts[1]
	protoMethod := methodParts[2]

	var foundType *resource.RPCSubtype
	for _, resSubtype := range r.ResourceRPCSubtypes() {
		if resSubtype.Desc.GetFullyQualifiedName() == protoSvc {
			subtypeCopy := resSubtype
			foundType = &subtypeCopy
			break
		}
	}
	if foundType == nil {
		return nil, nil, grpc.UnimplementedError
	}
	methodDesc := foundType.Desc.FindMethodByName(protoMethod)
	if methodDesc == nil {
		return nil, nil, grpc.UnimplementedError
	}

	return foundType, methodDesc, nil
}

// ResourceFromProtoMessage attempts to find out the name/resource associated with a gRPC message.
func ResourceFromProtoMessage(
	robot Robot,
	msg *dynamic.Message,
	subtype resource.Subtype,
) (interface{}, resource.Name, error) {
	// we assume a convention that there will be a field called name that will be the resource
	// name and a string.
	if !msg.HasFieldName("name") {
		return nil, resource.Name{}, errors.New("unable to determine resource name due to missing 'name' field")
	}
	name, ok := msg.GetFieldByName("name").(string)
	if !ok || name == "" {
		return nil, resource.Name{}, fmt.Errorf("unable to determine resource name due to invalid name field %v", name)
	}

	fqName := resource.NameFromSubtype(subtype, name)

	res, err := robot.ResourceByName(fqName)
	if err != nil {
		return nil, resource.Name{}, err
	}
	return res, fqName, nil
}
