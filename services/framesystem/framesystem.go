package framesystem

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
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
	FrameSystem(ctx context.Context, name string) (referenceframe.FrameSystem, error)
	Print(ctx context.Context) (string, error)
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

	fs, err := svc.FrameSystem(ctx, "")
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

// FrameSystem returns the frame system of the robot, building the system out of parts from the local robot
// and its remotes. Only local robots can call this function.
func (svc *frameSystemService) FrameSystem(ctx context.Context, name string) (referenceframe.FrameSystem, error) {
	ctx, span := trace.StartSpan(ctx, "services::framesystem::FrameSystem")
	defer span.End()
	parts, err := svc.gatherAllFrameSystemParts(ctx)
	if err != nil {
		return nil, err
	}
	baseFrameSys, err := NewFrameSystemFromParts(name, "", parts, svc.logger)
	if err != nil {
		return nil, err
	}
	svc.logger.Debugf("final frame system  %q has frames %v", baseFrameSys.Name(), baseFrameSys.FrameNames())
	return baseFrameSys, nil
}

// gatherAllFrameSystemParts is a helper function to get all parts from the local robot and all remote robots.
func (svc *frameSystemService) gatherAllFrameSystemParts(ctx context.Context) ([]*config.FrameSystemPart, error) {
	ctx, span := trace.StartSpan(ctx, "services::framesystem::gatherAllFrameSystemParts")
	defer span.End()
	localRobot, ok := svc.r.(robot.LocalRobot)
	if !ok {
		return nil, errors.New("only local robots can call gatherAllFrameSystemParts(), call Config() on remote robot for FrameSystem info")
	}
	// get the base parts and the the robot config to get frame info
	parts, err := svc.Config(ctx)
	if err != nil {
		return nil, err
	}
	conf, err := localRobot.Config(ctx)
	if err != nil {
		return nil, err
	}
	remoteNames := localRobot.RemoteNames()
	// get frame parts for each of its remotes
	for _, remoteName := range remoteNames {
		remote, ok := localRobot.RemoteByName(remoteName)
		if !ok {
			return nil, errors.Errorf("cannot find remote robot %s", remoteName)
		}
		remoteService, err := FromRobot(remote)
		if err != nil {
			svc.logger.Debugw("remote has frame system error, skipping", "remote", remoteName, "error", err)
			continue
		}
		remoteParts, err := remoteService.Config(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "remote %s", remoteName)
		}
		rConf, err := getRemoteConfig(remoteName, conf)
		if err != nil {
			return nil, errors.Wrapf(err, "remote %s", remoteName)
		}
		if rConf.Frame == nil { // skip over remote if it has no frame info
			svc.logger.Debugf("remote %s has no frame config info, skipping", remoteName)
			continue
		}
		remoteParts = renameRemoteParts(remoteParts, rConf)
		parts = append(parts, remoteParts...)
	}
	return parts, nil
}

// Print will print a table of the part names, parents, and static offsets. If the robot is a local robot,
// it will print out the complete frame system. If it is not a local robot, it will print a list of the frame parts.
func (svc *frameSystemService) Print(ctx context.Context) (string, error) {
	ctx, span := trace.StartSpan(ctx, "services::framesystem::Print")
	defer span.End()
	var err error
	var parts []*config.FrameSystemPart
	if _, ok := svc.r.(robot.LocalRobot); ok { // print entire frame system
		parts, err = svc.gatherAllFrameSystemParts(ctx)
		if err != nil {
			return "", err
		}
		parts, err = TopologicallySortParts(parts)
		if err != nil {
			return "", err
		}
	} else { // Don't have access to the local robot, just print the parts from the frame system service's Config
		parts, err = svc.Config(ctx)
		if err != nil {
			return "", err
		}
	}
	t := table.NewWriter()
	t.AppendHeader(table.Row{"#", "Name", "Parent", "Translation", "Orientation"})
	for i, part := range parts {
		tra := part.FrameConfig.Translation
		ori := part.FrameConfig.Orientation.OrientationVectorDegrees()
		t.AppendRow([]interface{}{
			i,
			part.Name,
			part.FrameConfig.Parent,
			fmt.Sprintf("X:%.2f, Y:%.2f, Z:%.2f", tra.X, tra.Y, tra.Z),
			fmt.Sprintf("OX:%.2f, OY:%.2f, OZ:%.2f, TH:%.2f", ori.OX, ori.OY, ori.OZ, ori.Theta),
		})
	}
	return t.Render(), nil
}
