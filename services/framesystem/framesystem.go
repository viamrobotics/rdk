package framesystem

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
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
	Print(ctx context.Context) (string, error)
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
	fs, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("framesystem.Service", resource)
	}
	return fs, nil
}

type frameSystemService struct {
	mu       sync.RWMutex
	r        robot.Robot
	allParts Parts
	fs       referenceframe.FrameSystem
	logger   golog.Logger
}

// Update will rebuild the frame system from the newly updated robot.
// TODO(RSDK-258): Does not update if a remote robot is updated. Remote updates need to re-trigger local updates.
func (svc *frameSystemService) Update(ctx context.Context, resources map[resource.Name]interface{}) error {
	fs, allParts, err := BuildFrameSystem(ctx, "robot", svc.r, svc.logger)
	if err != nil {
		return err
	}
	sortedParts, err := TopologicallySortParts(allParts)
	if err != nil {
		return err
	}
	svc.allParts = sortedParts
	svc.fs = fs
	svc.logger.Debugf("updated robot frame system:\n%v", svc.allParts.String())
	return nil
}

// Config returns the info of each individual part that makes up the frame system
// The output of this function is to be sent over GRPC to the client, so the client
// can build its frame system. The parts are not guaranteed to be returned topologically sorted.
func (svc *frameSystemService) Config(ctx context.Context) (Parts, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.allParts, nil
}

// FrameSystem returns the cached frame system of the robot.
func (svc *frameSystemService) FrameSystem(ctx context.Context) (referenceframe.FrameSystem, error) {
	// if it's not cached, then build the frame system now
	if svc.fs == nil {
		fs, allParts, err := BuildFrameSystem(ctx, "robot", svc.r, svc.logger)
		if err != nil {
			return nil, err
		}
		svc.allParts = allParts
		svc.fs = fs
	}
	return svc.fs, nil
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
	input := referenceframe.StartPositions(svc.fs)

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

	return svc.fs.TransformPose(input, pose.Pose(), pose.FrameName(), dst)
}

// Print will print a table of the part names, parents, and static offsets of the current frame system.
func (svc *frameSystemService) Print(ctx context.Context) (string, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.allParts.String(), nil
}
