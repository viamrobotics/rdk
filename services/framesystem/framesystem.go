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
	Config(ctx context.Context) ([]*config.FrameSystemPart, error)
	TransformPose(ctx context.Context, pose *referenceframe.PoseInFrame, dst string) (*referenceframe.PoseInFrame, error)
}

// New returns a new frame system service for the given robot.
func New(ctx context.Context, r robot.Robot, cfg config.Service, logger golog.Logger) (Service, error) {
	// collect the necessary robot parts (skipping remotes, services, etc)
	parts, err := CollectFrameSystemParts(ctx, r)
	if err != nil {
		return nil, err
	}
	// collect the frame info from each part that will be used to build the system
	children := make(map[string][]referenceframe.Frame)
	seen := make(map[string]bool)
	seen[referenceframe.World] = true
	for _, part := range parts {
		err := processPart(part, children, seen, logger)
		if err != nil {
			return nil, err
		}
	}
	sortedFrameNames, err := topologicallySortFrameNames(children)
	if err != nil {
		return nil, err
	}
	// If there are disconnected frames, inform the user.
	// Frames not seen may be in remote robots.
	if len(sortedFrameNames) != len(seen) {
		logger.Debugf(
			"found %d frames from config, but robot has %d local frames."+
				" Local frames are: %v. frames from config are: %v. Expected frames may be in remote robots",
			len(seen),
			len(sortedFrameNames),
			mapKeys(seen),
			sortedFrameNames,
		)
	}
	// ensure that at least one frame connects to world if the frame system is not empty
	if len(parts) != 0 && len(children[referenceframe.World]) == 0 {
		return nil, errors.New("there are no robot parts that connect to a 'world' node. Root node must be named 'world'")
	}

	fsSvc := &frameSystemService{
		r:                r,
		fsParts:          parts,
		sortedFrameNames: sortedFrameNames,
		logger:           logger,
	}
	logger.Debugf("frame system parts in robot: %v", sortedFrameNames)
	return fsSvc, nil
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
	mu               sync.RWMutex
	r                robot.Robot
	fsParts          map[string]*config.FrameSystemPart
	sortedFrameNames []string // topologically sorted frame names in the frame system, includes world frame
	logger           golog.Logger
}

// Config returns the info of each individual part that makes up the frame system
// The output of this function is to be sent over GRPC to the client, so the client
// can build the frame system. The parts are not guaranteed to be returned topologically sorted.
func (svc *frameSystemService) Config(ctx context.Context) ([]*config.FrameSystemPart, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	parts := make([]*config.FrameSystemPart, 0, len(svc.fsParts))
	for _, part := range svc.fsParts {
		parts = append(parts, part)
	}
	return parts, nil
}

// TransformPose will transform the pose of the requested poseInFrame to the desired frame in the robot's frame system.
func (svc *frameSystemService) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
) (*referenceframe.PoseInFrame, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	fs, err := svc.r.FrameSystem(ctx, "", "")
	if err != nil {
		return nil, err
	}
	// get the initial inputs
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
