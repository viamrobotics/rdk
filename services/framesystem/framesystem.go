package framesystem

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	servicepb "go.viam.com/rdk/proto/api/service/framesystem/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("frame_system")

// FrameSystemName is the default name of the frame system created by the service.
const FrameSystemName = "robot"

// Subtype is a constant that identifies the frame system resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the FrameSystemService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.FrameSystemService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterFrameSystemServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	})
}

// A Service that returns the frame system for a robot.
type Service interface {
	Config(ctx context.Context) (Parts, error)
	FrameSystem(ctx context.Context) (referenceframe.FrameSystem, error)
	TransformPose(ctx context.Context, pose *referenceframe.PoseInFrame, dst string) (*referenceframe.PoseInFrame, error)
}

// New returns a new frame system service for the given robot.
func New(ctx context.Context, r robot.Robot, cfg config.Service, logger golog.Logger) (Service, error) {
	return &frameSystemService{
		r:      r,
		logger: logger,
	}, nil
}

// FromRobot retrieves the frame system service of a robot.
func FromRobot(r robot.Robot) (Service, error) {
	resource, err := r.ResourceByName(Name)
	if err != nil {
		return nil, err
	}
	fss, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("framesystem.Service", resource)
	}
	return fss, nil
}

// the frame system service collects all the relevant parts that make up the frame system from the robot
// configs, and the remote robot configs.
type frameSystemService struct {
	mu           sync.RWMutex
	r            robot.Robot
	localParts   Parts                              // gotten from the local robot's config.Config
	offsetParts  map[string]*config.FrameSystemPart // gotten from local robot's config.Remote
	remotePrefix map[string]bool                    // gotten from local robot's config.Remote
	remoteParts  map[string]Parts                   // gotten from the remote robot's frameservice.Config(ctx)
	logger       golog.Logger
}

// Update will rebuild the frame system from the newly updated robot.
func (svc *frameSystemService) Update(ctx context.Context, resources map[resource.Name]interface{}) error {
	localParts, offsetParts, remoteParts, remotePrefix, err := CollectPartsFromRobotConfig(ctx, svc.r, svc.logger)
	if err != nil {
		return err
	}
	svc.localParts = localParts
	svc.offsetParts = offsetParts
	svc.remotePrefix = remotePrefix
	// rename the remote parts according to the offsets and prefixes
	for remoteName, rParts := range remoteParts {
		connectionName := remoteName + "_" + referenceframe.World
		rParts = renameRemoteParts(
			rParts,
			remoteName,
			svc.remotePrefix[remoteName],
			connectionName,
		)
		remoteParts[remoteName] = rParts
	}
	svc.remoteParts = remoteParts
	// combine the parts and print the result
	allParts := CombineParts(svc.localParts, svc.offsetParts, svc.remoteParts)
	sortedParts, err := TopologicallySortParts(allParts)
	if err != nil {
		return err
	}
	svc.logger.Debugf("updated robot frame system:\n%v", sortedParts.String())
	return nil
}

// Config returns the info of each individual part that makes up the frame system
// The output of this function is to be sent over GRPC to the client, so the client
// can build its frame system. requests the remote components from the remote's frame system service.
func (svc *frameSystemService) Config(ctx context.Context) (Parts, error) {
	// update part from remotes
	remoteParts := make(map[string]Parts)
	for _, remoteName := range svc.r.RemoteNames() {
		if _, ok := svc.offsetParts[remoteName]; !ok {
			continue // remote robot has no offset information, skip it
		}
		remoteBot, ok := svc.r.RemoteByName(remoteName)
		if !ok {
			return nil, errors.Errorf("remote %s not found for frame system config", remoteName)
		}
		rParts, err := collectAllPartsFromService(ctx, remoteBot)
		if err != nil {
			return nil, err
		}
		connectionName := remoteName + "_" + referenceframe.World
		rParts = renameRemoteParts(
			rParts,
			remoteName,
			svc.remotePrefix[remoteName],
			connectionName,
		)
		remoteParts[remoteName] = rParts
	}
	svc.remoteParts = remoteParts
	// build the config
	allParts := CombineParts(svc.localParts, svc.offsetParts, svc.remoteParts)
	sortedParts, err := TopologicallySortParts(allParts)
	if err != nil {
		return nil, err
	}
	return sortedParts, nil
}

// FrameSystem returns the frame system of the robot.
func (svc *frameSystemService) FrameSystem(ctx context.Context) (referenceframe.FrameSystem, error) {
	// create the frame system
	allParts, err := svc.Config(ctx)
	if err != nil {
		return nil, err
	}
	fs, err := BuildFrameSystem(FrameSystemName, allParts, svc.logger)
	if err != nil {
		return nil, err
	}
	return fs, nil
}

// TransformPose will transform the pose of the requested poseInFrame to the desired frame in the robot's frame system.
func (svc *frameSystemService) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
) (*referenceframe.PoseInFrame, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	// get the initial inputs
	fs, err := svc.FrameSystem(ctx)
	if err != nil {
		return nil, err
	}
	input := referenceframe.StartPositions(fs)

	// build maps of relevant components and inputs from initial inputs
	for name, original := range input {
		// determine frames to skip
		if len(original) == 0 {
			continue
		}

		// add component to map
		components := robot.AllResourcesByName(svc.r, name)
		if len(components) != 1 {
			return nil, fmt.Errorf("got %d resources instead of 1 for (%s)", len(components), name)
		}
		component, ok := components[0].(referenceframe.InputEnabled)
		if !ok {
			return nil, fmt.Errorf("%v(%T) is not InputEnabled", name, components[0])
		}

		// add input to map
		pos, err := component.CurrentInputs(ctx)
		if err != nil {
			return nil, err
		}
		input[name] = pos
	}

	return fs.TransformPose(input, pose.Pose(), pose.FrameName(), dst)
}
