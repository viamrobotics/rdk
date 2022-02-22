package framesystem

import (
	"context"
	"strings"
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
	// ensure that there are no disconnected frames
	if len(sortedFrameNames) != len(seen) {
		return nil, errors.Errorf(
			"the frame system is not fully connected, expected %d frames but frame system has %d."+
				" Expected frames are: %v. Actual frames are: %v",
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
	logger.Debugf("frame system for robot: %v", sortedFrameNames)
	return fsSvc, nil
}

// FromRobot retrieves the frame system service of a robot.
func FromRobot(r robot.Robot) (Service, error) {
	resource, ok := r.ResourceByName(Name)
	if !ok {
		return nil, errors.Errorf("resource %q not found", Name)
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

// Config returns a directed acyclic graph of the structure of the frame system
// The output of this function is to be sent over GRPC to the client, so the client can build the frame system.
// the slice should be returned topologically sorted, starting with the frames that are connected to the world node, and going up.
func (svc *frameSystemService) Config(ctx context.Context) ([]*config.FrameSystemPart, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	sortedFrameNames := svc.sortedFrameNames[1:] // skip the world frame at the beginning
	fsConfig := []*config.FrameSystemPart{}
	for _, name := range sortedFrameNames { // the list is topologically sorted already
		if strings.Contains(name, "_offset") { // skip offset frames, they will created again from the part config
			continue
		}
		if part, ok := svc.fsParts[name]; ok {
			fsConfig = append(fsConfig, part)
		} else {
			return nil, errors.Errorf("part %q not found in map of robot parts in the frame system service", name)
		}
	}
	return fsConfig, nil
}
