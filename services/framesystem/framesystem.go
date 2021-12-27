package framesystem

import (
	"context"
	"strings"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

// Type is the name of the type of service.
const Type = config.ServiceType("frame_system")

func init() {
	registry.RegisterService(Type, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	},
	)
}

// A Service that returns the frame system for a robot.
type Service interface {
	FrameSystemConfig(ctx context.Context) ([]*config.FrameSystemPart, error)
	LocalFrameSystem(ctx context.Context, name string) (referenceframe.FrameSystem, error)
	ModelFrame(ctx context.Context, name string) (referenceframe.Model, error)
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
		childrenMap:      children,
		logger:           logger,
	}
	logger.Debugf("frame system for robot: %v", sortedFrameNames)
	return fsSvc, nil
}

type frameSystemService struct {
	mu               sync.RWMutex
	r                robot.Robot
	fsParts          map[string]*config.FrameSystemPart
	sortedFrameNames []string // topologically sorted frame names in the frame system, includes world frame
	childrenMap      map[string][]referenceframe.Frame
	logger           golog.Logger
}

// FrameSystemConfig returns a directed acyclic graph of the structure of the frame system
// The output of this function is to be sent over GRPC to the client, so the client can build the frame system.
// the slice should be returned topologically sorted, starting with the frames that are connected to the world node, and going up.
func (svc *frameSystemService) FrameSystemConfig(ctx context.Context) ([]*config.FrameSystemPart, error) {
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

// LocalFrameSystem returns just the local components of the robot (excludes any parts from remote robots).
func (svc *frameSystemService) LocalFrameSystem(ctx context.Context, name string) (referenceframe.FrameSystem, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	fs, err := BuildFrameSystem(ctx, name, svc.childrenMap, svc.logger)
	if err != nil {
		return nil, err
	}
	return fs, nil
}

// ModelFrame returns the model frame for the named part in the local frame system.
// If the part does not have a model frame, nil will be returned.
func (svc *frameSystemService) ModelFrame(ctx context.Context, name string) (referenceframe.Model, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	if part, ok := svc.fsParts[name]; ok {
		return part.ModelFrame, nil
	}
	return nil, errors.Errorf("no part with name %q in frame system", name)
}
