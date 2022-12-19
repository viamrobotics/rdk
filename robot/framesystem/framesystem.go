package framesystem

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
)

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("frame_system")

// LocalFrameSystemName is the default name of the frame system created by the service.
const LocalFrameSystemName = "robot"

// A Service that returns the frame system for a robot.
type Service interface {
	Config(ctx context.Context, additionalTransforms []*referenceframe.LinkInFrame) (framesystemparts.Parts, error)
	TransformPose(
		ctx context.Context, pose *referenceframe.PoseInFrame, dst string,
		additionalTransforms []*referenceframe.LinkInFrame,
	) (*referenceframe.PoseInFrame, error)
}

// RobotFsCurrentInputs will get present inputs for a framesystem from a robot and return a map of those inputs, as well as a map of the
// InputEnabled resources that those inputs came from.
func RobotFsCurrentInputs(
	ctx context.Context,
	r robot.Robot,
	fs referenceframe.FrameSystem,
) (map[string][]referenceframe.Input, map[string]referenceframe.InputEnabled, error) {
	input := referenceframe.StartPositions(fs)

	// build maps of relevant components and inputs from initial inputs
	allOriginals := map[string][]referenceframe.Input{}
	resources := map[string]referenceframe.InputEnabled{}
	for name, original := range input {
		// skip frames with no input
		if len(original) == 0 {
			continue
		}

		// add component to map
		allOriginals[name] = original
		components := robot.AllResourcesByName(r, name)
		if len(components) != 1 {
			return nil, nil, fmt.Errorf("got %d resources instead of 1 for (%s)", len(components), name)
		}
		component, ok := components[0].(referenceframe.InputEnabled)
		if !ok {
			return nil, nil, fmt.Errorf("%v(%T) is not InputEnabled", name, components[0])
		}
		resources[name] = component

		// add input to map
		pos, err := component.CurrentInputs(ctx)
		if err != nil {
			return nil, nil, err
		}
		input[name] = pos
	}

	return input, resources, nil
}

// New returns a new frame system service for the given robot.
func New(ctx context.Context, r robot.Robot, logger golog.Logger) Service {
	return &frameSystemService{
		r:      r,
		logger: logger,
	}
}

// the frame system service collects all the relevant parts that make up the frame system from the robot
// configs, and the remote robot configs.
type frameSystemService struct {
	mu          sync.RWMutex
	r           robot.Robot
	localParts  framesystemparts.Parts                     // gotten from the local robot's config.Config
	offsetParts map[string]*referenceframe.FrameSystemPart // gotten from local robot's config.Remote
	logger      golog.Logger
}

// Update will rebuild the frame system from the newly updated robot.
// NOTE(RDK-258): If remotes can trigger a local robot to reconfigure, you can cache the remoteParts in svc as well.
func (svc *frameSystemService) Update(ctx context.Context, resources map[resource.Name]interface{}) error {
	ctx, span := trace.StartSpan(ctx, "services::framesystem::Update")
	defer span.End()
	err := svc.updateLocalParts(ctx)
	if err != nil {
		return err
	}
	err = svc.updateOffsetParts(ctx)
	if err != nil {
		return err
	}
	remoteParts, err := svc.updateRemoteParts(ctx)
	if err != nil {
		return err
	}
	// combine the parts, sort, and print the result
	allParts := combineParts(svc.localParts, svc.offsetParts, remoteParts)
	sortedParts, err := framesystemparts.TopologicallySort(allParts)
	if err != nil {
		return err
	}
	svc.logger.Debugf("updated robot frame system:\n%v", sortedParts.String())
	return nil
}

// Config returns the info of each individual part that makes up the frame system
// The output of this function is to be sent over GRPC to the client, so the client
// can build its frame system. requests the remote components from the remote's frame system service.
// NOTE(RDK-258): If remotes can trigger a local robot to reconfigure, you don't need to update remotes in every call.
func (svc *frameSystemService) Config(
	ctx context.Context,
	additionalTransforms []*referenceframe.LinkInFrame,
) (framesystemparts.Parts, error) {
	ctx, span := trace.StartSpan(ctx, "services::framesystem::Config")
	defer span.End()
	// update parts from remotes
	remoteParts, err := svc.updateRemoteParts(ctx)
	if err != nil {
		return nil, err
	}
	// build the config
	allParts := combineParts(svc.localParts, svc.offsetParts, remoteParts)
	for _, transformMsg := range additionalTransforms {
		newPart, err := referenceframe.LinkInFrameToFrameSystemPart(transformMsg)
		if err != nil {
			return nil, err
		}
		allParts = append(allParts, newPart)
	}
	sortedParts, err := framesystemparts.TopologicallySort(allParts)
	if err != nil {
		return nil, err
	}
	return sortedParts, nil
}

// TransformPose will transform the pose of the requested poseInFrame to the desired frame in the robot's frame system.
func (svc *frameSystemService) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
	additionalTransforms []*referenceframe.LinkInFrame,
) (*referenceframe.PoseInFrame, error) {
	ctx, span := trace.StartSpan(ctx, "services::framesystem::TransformPose")
	defer span.End()
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	// get the frame system and initial inputs
	allParts, err := svc.Config(ctx, additionalTransforms)
	if err != nil {
		return nil, err
	}
	fs, err := NewFrameSystemFromParts(LocalFrameSystemName, "", allParts, svc.logger)
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

	tf, err := fs.Transform(input, pose, dst)
	if err != nil {
		return nil, err
	}
	pose, _ = tf.(*referenceframe.PoseInFrame)
	return pose, nil
}

// updateLocalParts collects the physical parts of the robot that may have frame info,
// excluding remote robots and services, etc from the robot's config.Config.
func (svc *frameSystemService) updateLocalParts(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "services::framesystem::updateLocalParts")
	defer span.End()
	parts := make(map[string]*referenceframe.FrameSystemPart)
	seen := make(map[string]bool)
	local, ok := svc.r.(robot.LocalRobot)
	if !ok {
		return robot.NewUnimplementedLocalInterfaceError(svc.r)
	}
	cfg, err := local.Config(ctx) // Eventually there will be another function that gathers the frame system config
	if err != nil {
		return err
	}
	for _, c := range cfg.Components {
		if c.Frame == nil { // no Frame means dont include in frame system.
			continue
		}
		if c.Name == referenceframe.World {
			return errors.Errorf("cannot give frame system part the name %s", referenceframe.World)
		}
		if c.Frame.Parent == "" {
			return errors.Errorf("parent field in frame config for part %q is empty", c.Name)
		}
		if c.Frame.ID == "" {
			c.Frame.ID = c.Name
		}
		seen[c.Name] = true
		model, err := extractModelFrameJSON(svc.r, c.ResourceName())
		if err != nil && !errors.Is(err, referenceframe.ErrNoModelInformation) {
			return err
		}
		lif, err := c.Frame.ParseConfig()
		if err != nil {
			return err
		}
		parts[c.Frame.ID] = &referenceframe.FrameSystemPart{FrameConfig: lif, ModelFrame: model}
	}
	svc.localParts = framesystemparts.PartMapToPartSlice(parts)
	return nil
}

// updateOffsetParts collects the frame offset information from the config.Remote of the local robot.
func (svc *frameSystemService) updateOffsetParts(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "services::framesystem::updateOffsetParts")
	defer span.End()
	local, ok := svc.r.(robot.LocalRobot)
	if !ok {
		return robot.NewUnimplementedLocalInterfaceError(svc.r)
	}
	conf, err := local.Config(ctx)
	if err != nil {
		return err
	}
	remoteNames := local.RemoteNames()
	offsetParts := make(map[string]*referenceframe.FrameSystemPart)
	for _, remoteName := range remoteNames {
		rConf, err := getRemoteRobotConfig(remoteName, conf)
		if err != nil {
			return errors.Wrapf(err, "remote %s", remoteName)
		}
		if rConf.Frame == nil { // skip over remote if it has no frame info
			svc.logger.Debugf("remote %s has no frame config info, skipping", remoteName)
			continue
		}

		lif, err := rConf.Frame.ParseConfig()
		if err != nil {
			return err
		}
		// build the frame system part that connects remote world to base world
		connection := &referenceframe.FrameSystemPart{
			FrameConfig: lif,
		}
		connection.FrameConfig.SetName(rConf.Name + "_" + referenceframe.World)
		offsetParts[remoteName] = connection
	}
	svc.offsetParts = offsetParts
	return nil
}

// updateRemoteParts is a helper function to get parts from the connected remote robots, and renames them.
func (svc *frameSystemService) updateRemoteParts(ctx context.Context) (map[string]framesystemparts.Parts, error) {
	ctx, span := trace.StartSpan(ctx, "services::framesystem::updateRemoteParts")
	defer span.End()
	// get frame parts for each remote robot, skip if not in remote offset map
	remoteParts := make(map[string]framesystemparts.Parts)
	remoteNames := svc.r.RemoteNames()
	for _, remoteName := range remoteNames {
		if _, ok := svc.offsetParts[remoteName]; !ok {
			continue // no remote frame info, skipping
		}
		remote, ok := svc.r.RemoteByName(remoteName)
		if !ok {
			return nil, errors.Errorf("cannot find remote robot %s", remoteName)
		}
		rParts, err := robotFrameSystemConfig(ctx, remote)
		if err != nil {
			return nil, errors.Wrapf(err, "remote %s", remoteName)
		}
		connectionName := remoteName + "_" + referenceframe.World
		rParts = framesystemparts.RenameRemoteParts(rParts, remoteName, connectionName)
		remoteParts[remoteName] = rParts
	}
	return remoteParts, nil
}
