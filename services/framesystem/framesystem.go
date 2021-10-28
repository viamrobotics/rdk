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
	FrameSystemDAG(ctx context.Context) ([]*pb.Node, error)
	LocalFrameSystem(ctx context.Context) (referenceframe.FrameSystem, error)
	Frame(ctx context.Context, name string) (referenceframe.Frame, error)
	Close() error
}

// New returns a new frame system service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	fs, sortedFrameNames, err := createRobotFrameSystem(ctx, r, "robot")
	if err != nil {
		return nil, err
	}
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	fsSvc := &frameSystemService{
		r:                r,
		fs:               fs,
		sortedFrameNames: sortedFrameNames,
		logger:           logger,
		cancelCtx:        cancelCtx,
		cancelFunc:       cancelFunc,
	}
	return fsSvc, nil
}

type frameSystemService struct {
	mu               sync.RWMutex
	r                robot.Robot
	fs               referenceframe.FrameSystem
	sortedFrameNames []string

	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// FrameSystemDAG returns a DAG of the parent structure of the frame system
// The output of this function is to be sent over GRPC to the client, so the client can build the frame system.
// the slice should be returned topologically sorted, starting with the frames that are connected to the world node, and going up
func (svc *frameSystemService) FrameSystemDAG(ctx context.Context) ([]*pb.Node, error) {
	dag := make([]*pb.Node, len(svc.fs.FrameNames()))
	for i, name := range svc.sortedFrameNames { // the list is topologically sorted already
		parent, err := svc.fs.Parent(svc.fs.GetFrame(name))
		if err != nil {
			return nil, err
		}
		dag[i] = &pb.Node{Name: name, Parent: parent.Name()}
	}
	return dag, nil
}

// LocalFrameSystem returns just the local components of the robot (excludes any parts from remote robots)
func (svc *frameSystemService) LocalFrameSystem(ctx context.Context) (referenceframe.FrameSystem, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.fs, nil
}

// Frame returns a specific frame from the local frame system
func (svc *frameSystemService) Frame(ctx context.Context, name string) (referenceframe.Frame, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	frame := svc.fs.GetFrame(name)
	if frame == nil {
		return nil, errors.Errorf("frame %q not found in frame system %q", name, svc.fs.Name())
	}
	return frame, nil
}

func (svc *frameSystemService) Close() error {
	svc.cancelFunc()
	svc.activeBackgroundWorkers.Wait()
	return nil
}

// Linker has a method that returns all the information needed to add the component
// to a FrameSystem.
type Linker interface {
	FrameSystemLink() (*config.Frame, referenceframe.Frame)
}

// namedPart is used to collect the various named robot parts that could potentially have frame information
type namedPart struct {
	Name string
	Part Linker
}

func createRobotFrameSystem(ctx context.Context, r robot.Robot, robotName string) (referenceframe.FrameSystem, []string, error) {
	// collect the necessary robot parts (skipping remotes, services, etc)
	parts := collectRobotParts(r)
	// collect the frame info from each part that will be used to build the system
	children := map[string][]referenceframe.Frame{}
	names := map[string]bool{}
	names[referenceframe.World] = true
	for _, part := range parts {
		if part.Name == "" {
			return nil, nil, errors.New("part name cannot be empty")
		}
		err := createFrameFromPart(part, children, names)
		if err != nil {
			return nil, nil, err
		}
	}
	return buildFrameSystem(ctx, robotName, names, children, r.Logger())
}

// collectRobotParts collects the physical parts of the robot that may have frame info (excluding remote robots and services, etc)
// don't collect remote components by checking for *client.clientPart types
func collectRobotParts(r robot.Robot) []namedPart {
	logger := r.Logger()
	parts := []namedPart{}
	for _, name := range r.BaseNames() {
		part, ok := r.BaseByName(name)
		if !ok {
			continue
		}
		if fsl, ok := utils.UnwrapProxy(part).(Linker); ok {
			parts = append(parts, namedPart{name, fsl})
		} else {
			logger.Infof("part %q of type %T does not have FrameSystemLink() defined", name, utils.UnwrapProxy(part))
		}
	}
	for _, name := range r.BoardNames() {
		part, ok := r.BoardByName(name)
		if !ok {
			continue
		}
		if fsl, ok := utils.UnwrapProxy(part).(Linker); ok {
			parts = append(parts, namedPart{name, fsl})
		} else {
			logger.Infof("part %q of type %T does not have FrameSystemLink() defined", name, utils.UnwrapProxy(part))
		}
	}
	for _, name := range r.CameraNames() {
		part, ok := r.CameraByName(name)
		if !ok {
			continue
		}
		if fsl, ok := utils.UnwrapProxy(part).(Linker); ok {
			parts = append(parts, namedPart{name, fsl})
		} else {
			logger.Infof("part %q of type %T does not have FrameSystemLink() defined", name, utils.UnwrapProxy(part))
		}
	}
	for _, name := range r.GripperNames() {
		part, ok := r.GripperByName(name)
		if !ok {
			continue
		}
		if fsl, ok := utils.UnwrapProxy(part).(Linker); ok {
			parts = append(parts, namedPart{name, fsl})
		} else {
			logger.Infof("part %q of type %T does not have FrameSystemLink() defined", name, utils.UnwrapProxy(part))
		}
	}
	for _, name := range r.LidarNames() {
		part, ok := r.LidarByName(name)
		if !ok {
			continue
		}
		if fsl, ok := utils.UnwrapProxy(part).(Linker); ok {
			parts = append(parts, namedPart{name, fsl})
		} else {
			logger.Infof("part %q of type %T does not have FrameSystemLink() defined", name, utils.UnwrapProxy(part))
		}
	}
	for _, name := range r.SensorNames() {
		part, ok := r.SensorByName(name)
		if !ok {
			continue
		}
		if fsl, ok := utils.UnwrapProxy(part).(Linker); ok {
			parts = append(parts, namedPart{name, fsl})
		} else {
			logger.Infof("part %q of type %T does not have FrameSystemLink() defined", name, utils.UnwrapProxy(part))
		}
	}
	for _, name := range r.ServoNames() {
		part, ok := r.ServoByName(name)
		if !ok {
			continue
		}
		if fsl, ok := utils.UnwrapProxy(part).(Linker); ok {
			parts = append(parts, namedPart{name, fsl})
		} else {
			logger.Infof("part %q of type %T does not have FrameSystemLink() defined", name, utils.UnwrapProxy(part))
		}
	}
	for _, name := range r.MotorNames() {
		part, ok := r.MotorByName(name)
		if !ok {
			continue
		}
		if fsl, ok := utils.UnwrapProxy(part).(Linker); ok {
			parts = append(parts, namedPart{name, fsl})
		} else {
			logger.Infof("part %q of type %T does not have FrameSystemLink() defined", name, utils.UnwrapProxy(part))
		}
	}

	for _, name := range r.ResourceNames() {
		part, ok := r.ResourceByName(name)
		if !ok {
			continue
		}
		if fsl, ok := utils.UnwrapProxy(part).(Linker); ok {
			parts = append(parts, namedPart{name.Name, fsl})
		} else {
			logger.Infof("part %q of type %T does not have FrameSystemLink() defined", name, utils.UnwrapProxy(part))
		}
	}
	return parts
}

// createFrameFromPart will gather the frame information and build the frames from robot parts that have FrameSystemLink() defined.
func createFrameFromPart(part namedPart, children map[string][]referenceframe.Frame, names map[string]bool) error {
	frameConfig, modelFrame := part.Part.FrameSystemLink()
	// part must have FrameSystemLink() defined to be added to a FrameSystem
	// if a part has no frame config, skip over it
	if frameConfig == nil {
		return nil
	}
	// parent field is a necessary attribute
	if frameConfig.Parent == "" {
		return fmt.Errorf("parent field in frame config for part %q is empty", part.Name)
	}
	// use identity frame if no model frame defined
	if modelFrame == nil {
		modelFrame = referenceframe.NewZeroStaticFrame(part.Name)
	}
	// static frame defines an offset from the parent part-- if it is empty, a 0 offset frame will be applied.
	staticName := part.Name + "_offset"
	staticFrame, err := config.MakeStaticFrame(frameConfig, staticName)
	if err != nil {
		return err
	}
	// check to see if there are no repeated names
	if _, ok := names[staticFrame.Name()]; ok {
		return fmt.Errorf("cannot have more than one frame with name %s", staticFrame.Name())
	}
	names[staticFrame.Name()] = true
	if _, ok := names[modelFrame.Name()]; ok {
		return fmt.Errorf("cannot have more than one frame with name %s", modelFrame.Name())
	}
	names[modelFrame.Name()] = true
	// attach the static frame to the parent, then the model frame to the static frame
	children[frameConfig.Parent] = append(children[frameConfig.Parent], staticFrame)
	children[staticFrame.Name()] = append(children[staticFrame.Name()], modelFrame)
	return nil
}

func buildFrameSystem(ctx context.Context, name string, frameNames map[string]bool, children map[string][]referenceframe.Frame, logger golog.Logger) (referenceframe.FrameSystem, []string, error) {
	// use a stack to populate the frame system
	stack := make([]string, 0)
	topoSortedNames := make([]string, 0) // keep track of tree structure
	visited := make(map[string]bool)
	// check to see if world exists, and start with the frames attached to world
	if _, ok := children[referenceframe.World]; !ok {
		return nil, nil, errors.New("there are no frames that connect to a 'world' node. Root node must be named 'world'")
	}
	stack = append(stack, referenceframe.World)
	// begin adding frames to the frame system
	fs := referenceframe.NewEmptySimpleFrameSystem(name)
	for len(stack) != 0 {
		parent := stack[0] // pop the top element from the stack
		stack = stack[1:]
		if _, ok := visited[parent]; ok {
			return nil, nil, fmt.Errorf("the system contains a cycle, have already visited frame %s", parent)
		}
		visited[parent] = true
		for _, frame := range children[parent] { // add all the children to the frame system, and to the stack as new parents
			stack = append(stack, frame.Name())
			topoSortedNames = append(topoSortedNames, frame.Name())
			err := fs.AddFrame(frame, fs.GetFrame(parent))
			if err != nil {
				return nil, nil, err
			}
		}
	}
	// ensure that there are no disconnected frames
	if len(visited) != len(frameNames) {
		return nil, nil, fmt.Errorf("the frame system is not fully connected, expected %d frames but frame system has %d. Expected frames are: %v. Actual frames are: %v", len(frameNames), len(visited), mapKeys(frameNames), mapKeys(visited))
	}
	logger.Debugf("frames in robot frame system are: %v", frameNamesWithDof(ctx, fs))
	return fs, topoSortedNames, nil
}

func frameNamesWithDof(ctx context.Context, sys referenceframe.FrameSystem) []string {
	names := sys.FrameNames()
	nameDoFs := make([]string, len(names))
	for i, f := range names {
		fr := sys.GetFrame(f)
		nameDoFs[i] = fmt.Sprintf("%s(%d)", fr.Name(), len(fr.DoF(ctx)))
	}
	return nameDoFs
}

func mapKeys(fullmap map[string]bool) []string {
	keys := make([]string, len(fullmap))
	i := 0
	for k := range fullmap {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}
