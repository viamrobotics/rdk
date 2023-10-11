// Package robot defines the robot which is the root of all robotic parts.
package robot

import (
	"context"
	"fmt"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/pkg/errors"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/packages"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/session"
)

// A Robot encompasses all functionality of some robot comprised
// of parts, local and remote.
type Robot interface {
	// DiscoverComponents returns discovered component configurations.
	DiscoverComponents(ctx context.Context, qs []resource.DiscoveryQuery) ([]resource.Discovery, error)

	// RemoteByName returns a remote robot by name.
	RemoteByName(name string) (Robot, bool)

	// ResourceByName returns a resource by name
	ResourceByName(name resource.Name) (resource.Resource, error)

	// RemoteNames returns the names of all known remote robots.
	RemoteNames() []string

	// ResourceNames returns a list of all known resource names.
	ResourceNames() []resource.Name

	// ResourceRPCAPIs returns a list of all known resource RPC APIs.
	ResourceRPCAPIs() []resource.RPCAPI

	// ProcessManager returns the process manager for the robot.
	ProcessManager() pexec.ProcessManager

	// OperationManager returns the operation manager the robot is using.
	OperationManager() *operation.Manager

	// SessionManager returns the session manager the robot is using.
	SessionManager() session.Manager

	// PackageManager returns the package manager the robot is using.
	PackageManager() packages.Manager

	// Logger returns the logger the robot is using.
	Logger() logging.Logger

	// FrameSystemConfig returns the individual parts that make up a robot's frame system
	FrameSystemConfig(ctx context.Context) (*framesystem.Config, error)

	// TransformPose will transform the pose of the requested poseInFrame to the desired frame in the robot's frame system.
	TransformPose(
		ctx context.Context,
		pose *referenceframe.PoseInFrame,
		dst string,
		additionalTransforms []*referenceframe.LinkInFrame,
	) (*referenceframe.PoseInFrame, error)

	// TransformPointCloud will transform the pointcloud to the desired frame in the robot's frame system.
	// Do not move the robot between the generation of the initial pointcloud and the receipt
	// of the transformed pointcloud because that will make the transformations inaccurate.
	TransformPointCloud(ctx context.Context, srcpc pointcloud.PointCloud, srcName, dstName string) (pointcloud.PointCloud, error)

	// Status takes a list of resource names and returns their corresponding statuses. If no names are passed in, return all statuses.
	Status(ctx context.Context, resourceNames []resource.Name) ([]Status, error)

	// Close attempts to cleanly close down all constituent parts of the robot.
	Close(ctx context.Context) error

	// StopAll cancels all current and outstanding operations for the robot and stops all actuators and movement
	StopAll(ctx context.Context, extra map[resource.Name]map[string]interface{}) error
}

// A LocalRobot is a Robot that can have its parts modified.
type LocalRobot interface {
	Robot

	// Config returns a config representing the current state of the robot.
	Config() *config.Config

	// Reconfigure instructs the robot to safely reconfigure itself based
	// on the given new config.
	Reconfigure(ctx context.Context, newConfig *config.Config)

	// StartWeb starts the web server, will return an error if server is already up.
	StartWeb(ctx context.Context, o weboptions.Options) error

	// StopWeb stops the web server, will be a noop if server is not up.
	StopWeb()

	// WebAddress returns the address of the web service.
	WebAddress() (string, error)

	// ModuleAddress returns the address (path) of the unix socket modules use to contact the parent.
	ModuleAddress() (string, error)
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

// AllResourcesByName returns an array of all resources that have this short name.
// NOTE: this function queries by the shortname rather than the fully qualified resource name which is not recommended practice
// and may become deprecated in the future.
func AllResourcesByName(r Robot, name string) []resource.Resource {
	all := []resource.Resource{}

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

// NamesByAPI is a helper for getting all names from the given Robot given the API.
func NamesByAPI(r Robot, api resource.API) []string {
	names := []string{}
	for _, n := range r.ResourceNames() {
		if n.API == api {
			names = append(names, n.ShortName())
		}
	}
	return names
}

// TypeAndMethodDescFromMethod attempts to determine the resource API and its respective gRPC method information
// from the given robot and method path. If nothing can be found, grpc.UnimplementedError is returned.
func TypeAndMethodDescFromMethod(r Robot, method string) (*resource.RPCAPI, *desc.MethodDescriptor, error) {
	methodParts := strings.Split(method, "/")
	if len(methodParts) != 3 {
		return nil, nil, grpc.UnimplementedError
	}
	protoSvc := methodParts[1]
	protoMethod := methodParts[2]

	var foundType *resource.RPCAPI
	for _, resAPI := range r.ResourceRPCAPIs() {
		if resAPI.Desc.GetFullyQualifiedName() == protoSvc {
			apiCopy := resAPI
			foundType = &apiCopy
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
	api resource.API,
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

	fqName := resource.NewName(api, name)

	res, err := robot.ResourceByName(fqName)
	if err != nil {
		return nil, resource.Name{}, err
	}
	return res, fqName, nil
}

// ResourceFromRobot returns a resource from a robot.
func ResourceFromRobot[T resource.Resource](robot Robot, name resource.Name) (T, error) {
	var zero T
	res, err := robot.ResourceByName(name)
	if err != nil {
		return zero, err
	}

	part, ok := res.(T)

	if !ok {
		return zero, resource.TypeError[T](res)
	}
	return part, nil
}
