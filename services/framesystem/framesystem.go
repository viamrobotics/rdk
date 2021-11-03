package framesystem

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/utils"
)

// Type is the name of the type of service
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
	FrameSystemConfig(ctx context.Context) ([]*pb.FrameConfig, error)
	LocalFrameSystem(ctx context.Context) (referenceframe.FrameSystem, error)
	ModelFrame(ctx context.Context, name string) ([]byte, error)
	Close() error
}

type frameSystemService struct {
	mu               sync.RWMutex
	r                robot.Robot
	fs               referenceframe.FrameSystem
	fsParts          map[string]config.FrameSystemPart
	sortedFrameNames []string // topologically sorted frame names in the frame system

	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// New returns a new frame system service for the given robot.
func New(ctx context.Context, r robot.Robot, cfg config.Service, logger golog.Logger) (Service, error) {
	// collect the necessary robot parts (skipping remotes, services, etc)
	parts, err := config.CollectFrameSystemParts(ctx, r)
	if err != nil {
		return nil, err
	}
	// collect the frame info from each part that will be used to build the system
	children := make(map[string][]referenceframe.Frame)
	names := make(map[string]bool)
	names[referenceframe.World] = true
	for _, part := range parts {
		err := processPart(part, children, names)
		if err != nil {
			return nil, err
		}
	}
	frameSystem, err := referenceframe.BuildFrameSystem(ctx, "robot", names, children, r.Logger())
	if err != nil {
		return nil, err
	}
	sortedFrameNames, err := topologicallySortFrameNames(ctx, names, children)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	fsSvc := &frameSystemService{
		r:                r,
		fs:               frameSystem,
		fsParts:          parts,
		sortedFrameNames: sortedFrameNames,
		logger:           logger,
		cancelCtx:        cancelCtx,
		cancelFunc:       cancelFunc,
	}
	return fsSvc, nil
}

// FrameSystemConfig returns a directed acyclic graph of the structure of the frame system
// The output of this function is to be sent over GRPC to the client, so the client can build the frame system.
// the slice should be returned topologically sorted, starting with the frames that are connected to the world node, and going up
func (svc *frameSystemService) FrameSystemConfig(ctx context.Context) ([]config.FrameSystemPart, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	fsConfig := make([]config.FrameSystemPart, len(svc.fs.FrameNames()))
	for i, name := range svc.sortedFrameNames { // the list is topologically sorted already
		if part, ok := svc.fsParts[name]; ok {
			fsConfig[i] = part
		} else {
			return nil, errors.Errorf("part %q not found in fsParts in the frame system service", name)
		}
	}
	return fsConfig, nil
}

// LocalFrameSystem returns just the local components of the robot (excludes any parts from remote robots)
func (svc *frameSystemService) LocalFrameSystem(ctx context.Context) (referenceframe.FrameSystem, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.fs, nil
}

// ModelFrame returns the model frame for the named part in the local frame system.
// If the part does not have a model frame, nil will be returned
func (svc *frameSystemService) ModelFrame(ctx context.Context, name string) ([]byte, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	if part, ok := svc.fsParts[name]; ok {
		return part.ModelFrameConfig, nil
	}
	return nil, errors.Errorf("no part with name %q in frame system", name)
}

// Close closes the robot service
func (svc *frameSystemService) Close() error {
	svc.cancelFunc()
	svc.activeBackgroundWorkers.Wait()
	return nil
}

// processPart will gather the frame information and build the frames from the given robot part
func processPart(part config.FrameSystemPart, children map[string][]referenceframe.Frame, names map[string]bool) error {
	// if a part has no frame config, skip over it
	if part.FrameConfig == nil {
		return nil
	}
	// parent field is a necessary attribute
	if part.FrameConfig.Parent == "" {
		return fmt.Errorf("parent field in frame config for part %q is empty", part.Name)
	}
	// build the frames from the part config
	modelFrame, staticOffsetFrame, err := config.CreateFramesFromPart(part)
	if err != nil {
		return err
	}
	// check to see if there are no repeated names
	if _, ok := names[staticOffsetFrame.Name()]; ok {
		return fmt.Errorf("cannot have more than one frame with name %s", staticOffsetFrame.Name())
	}
	names[staticOffsetFrame.Name()] = true
	if _, ok := names[modelFrame.Name()]; ok {
		return fmt.Errorf("cannot have more than one frame with name %s", modelFrame.Name())
	}
	names[modelFrame.Name()] = true
	// attach the static frame to the parent, then the model frame to the static frame
	children[frameConfig.Parent] = append(children[frameConfig.Parent], staticOffsetFrame)
	children[staticOffsetFrame.Name()] = append(children[staticOffsetFrame.Name()], modelFrame)
	return nil
}

func topologicallySortFrameNames(ctx context.Context, frameNames map[string]bool, children map[string][]referenceframe.Frame) ([]string, error) {
	topoSortedNames := make([]string, 0) // keep track of tree structure
	stack := make([]string, 0)
	visited := make(map[string]bool)
	if _, ok := children[referenceframe.World]; !ok {
		return nil, errors.New("there are no frames that connect to a 'world' node. Root node must be named 'world'")
	}
	stack = append(stack, referenceframe.World)
	// begin adding frames to the frame system
	for len(stack) != 0 {
		parent := stack[0] // pop the top element from the stack
		stack = stack[1:]
		if _, ok := visited[parent]; ok {
			return nil, fmt.Errorf("the system contains a cycle, have already visited frame %s", parent)
		}
		visited[parent] = true
		for _, frame := range children[parent] { // add all the children to the frame system, and to the stack as new parents
			stack = append(stack, frame.Name())
			topoSortedNames = append(topoSortedNames, frame.Name())
		}
	}
	return topoSortedNames, nil
}
