// Package robot defines the robot which is the root of all robotic parts.
package robot

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/pkg/errors"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/cloud"
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

const (
	platform = "rdk"
)

// A Robot encompasses all functionality of some robot comprised
// of parts, local and remote.
//
// DiscoverComponents example:
//
//	// Define a new discovery query.
//	q := resource.NewDiscoveryQuery(camera.API, resource.Model{Name: "webcam", Family: resource.DefaultModelFamily})
//
//	// Define a list of discovery queries and get potential component configurations with these queries.
//	out, err := machine.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{q})
//
// ResourceNames example:
//
//	resource_names := machine.ResourceNames()
//
// FrameSystemConfig example:
//
//	// Print the frame system configuration
//	frameSystem, err := machine.FrameSystemConfig(context.Background())
//	fmt.Println(frameSystem)
//
// TransformPose example:
//
//	import (
//	  "go.viam.com/rdk/referenceframe"
//	  "go.viam.com/rdk/spatialmath"
//	)
//
//	baseOrigin := referenceframe.NewPoseInFrame("test-base", spatialmath.NewZeroPose())
//	movementSensorToBase, err := machine.TransformPose(context.Background(), baseOrigin, "my-movement-sensor", nil)
//
// CloudMetadata example:
//
//	metadata, err := machine.CloudMetadata(context.Background())
//	primary_org_id := metadata.PrimaryOrgID
//	location_id := metadata.LocationID
//	machine_id := metadata.MachineID
//	machine_part_id := metadata.MachinePartID
//
// Close example:
//
//	// Cleanly close the underlying connections and stop any periodic tasks,
//	err := machine.Close(context.Background())
//
// StopAll example:
//
//	// Cancel all current and outstanding operations for the machine and stop all actuators and movement.
//	err := machine.StopAll(context.Background(), nil)
//
// Shutdown example:
//
//	// Shut down the robot.
//	err := machine.Shutdown(context.Background())
type Robot interface {
	// DiscoverComponents returns discovered potential component configurations.
	// Only implemented for webcam cameras in builtin components.
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

	// CloudMetadata returns app-related information about the robot.
	CloudMetadata(ctx context.Context) (cloud.Metadata, error)

	// Close attempts to cleanly close down all constituent parts of the robot.
	Close(ctx context.Context) error

	// StopAll cancels all current and outstanding operations for the robot and stops all actuators and movement
	StopAll(ctx context.Context, extra map[resource.Name]map[string]interface{}) error

	// RestartModule reloads a module as if its config changed
	RestartModule(ctx context.Context, req RestartModuleRequest) error

	// Shutdown shuts down the robot.
	Shutdown(ctx context.Context) error

	// MachineStatus returns the current status of the robot.
	MachineStatus(ctx context.Context) (MachineStatus, error)

	// Version returns version information about the robot.
	Version(ctx context.Context) (VersionResponse, error)
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

	// ExportResourcesAsDot exports the resource graph as a DOT representation for
	// visualization.
	// DOT reference: https://graphviz.org/doc/info/lang.html
	ExportResourcesAsDot(index int) (resource.GetSnapshotInfo, error)

	// RestartAllowed returns whether the robot can safely be restarted.
	RestartAllowed() bool

	// Kill will attempt to kill any processes on the system started by the robot as quickly as possible.
	// This operation is not clean and will not wait for completion.
	// Only use this if comfortable with leaking resources (in cases where exiting the program as quickly as possible is desired).
	Kill()
}

// A RemoteRobot is a Robot that was created through a connection.
type RemoteRobot interface {
	Robot

	// Connected returns whether the remote is connected or not.
	Connected() bool
}

// RestartModuleRequest is a go mirror of a proto message.
type RestartModuleRequest struct {
	ModuleID   string
	ModuleName string
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
	protoSvc := methodParts[1]    // e.g. viam.component.arm.v1.ArmService
	protoMethod := methodParts[2] // e.g. DoCommand

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

// MatchesModule returns true if the passed-in module matches its name / ID.
func (rmr *RestartModuleRequest) MatchesModule(mod config.Module) bool {
	if len(rmr.ModuleID) > 0 {
		return mod.ModuleID == rmr.ModuleID
	}
	return mod.Name == rmr.ModuleName
}

// MachineState captures the state of a machine.
type MachineState uint8

const (
	// StateUnknown represents an unknown state.
	StateUnknown MachineState = iota
	// StateInitializing denotes a currently initializing machine. The first
	// reconfigure after initial creation has not completed.
	StateInitializing
	// StateRunning denotes a running machine. The first reconfigure after
	// initial creation has completed.
	StateRunning
)

// MachineStatus encapsulates the current status of the robot.
type MachineStatus struct {
	Resources []resource.Status
	Config    config.Revision
	State     MachineState
}

// VersionResponse encapsulates the version info of the robot.
type VersionResponse struct {
	Platform   string
	Version    string
	APIVersion string
}

// Version returns platform, version and API version of the robot.
// platform will always be `rdk`
// If built without a version tag,  will be dev-<git hash>.
func Version() (VersionResponse, error) {
	var result VersionResponse
	result.Platform = platform

	version := config.Version
	if version == "" {
		version = "dev-"
		if config.GitRevision != "" {
			version += config.GitRevision
		} else {
			version += "unknown"
		}
	}
	result.Version = version

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return result, errors.New("error reading build info")
	}
	deps := make(map[string]*debug.Module, len(info.Deps))
	for _, dep := range info.Deps {
		deps[dep.Path] = dep
	}

	apiVersion := "?"
	if dep, ok := deps["go.viam.com/api"]; ok {
		apiVersion = dep.Version
	}
	result.APIVersion = apiVersion

	return result, nil
}
