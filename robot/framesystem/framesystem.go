package framesystem

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// LocalFrameSystemName is the default name of the frame system created by the service.
const LocalFrameSystemName = "robot"

// SubtypeName is a constant that identifies the internal frame system resource subtype string.
const SubtypeName = "frame_system"

// API is the fully qualified API for the internal frame system service.
var API = resource.APINamespaceRDKInternal.WithServiceType(SubtypeName)

// InternalServiceName is used to refer to/depend on this service internally.
var InternalServiceName = resource.NewName(API, "builtin")

// A Service that returns the frame system for a robot.
type Service interface {
	resource.Resource
	TransformPose(
		ctx context.Context,
		pose *referenceframe.PoseInFrame,
		dst string,
		additionalTransforms []*referenceframe.LinkInFrame,
	) (*referenceframe.PoseInFrame, error)
	TransformPointCloud(ctx context.Context, srcpc pointcloud.PointCloud, srcName, dstName string) (pointcloud.PointCloud, error)
	CurrentInputs(ctx context.Context) (map[string][]referenceframe.Input, map[string]referenceframe.InputEnabled, error)
	FrameSystem(ctx context.Context, additionalTransforms []*referenceframe.LinkInFrame) (referenceframe.FrameSystem, error)
}

// New returns a new frame system service for the given robot.
func New(ctx context.Context, deps resource.Dependencies, logger golog.Logger) (Service, error) {
	fs := &frameSystemService{
		Named:      InternalServiceName.AsNamed(),
		components: make(map[string]resource.Resource),
		logger:     logger,
	}
	if err := fs.Reconfigure(ctx, deps, resource.Config{}); err != nil {
		return nil, err
	}
	return fs, nil
}

var internalFrameSystemServiceName = resource.NewName(
	resource.APINamespaceRDKInternal.WithServiceType("framesystem"),
	"builtin",
)

func (svc *frameSystemService) Name() resource.Name {
	return internalFrameSystemServiceName
}

// the frame system service collects all the relevant parts that make up the frame system from the robot
// configs, and the remote robot configs.
type frameSystemService struct {
	resource.Named
	resource.TriviallyCloseable
	components map[string]resource.Resource
	logger     golog.Logger

	parts   Parts
	partsMu sync.RWMutex
}

// Reconfigure will rebuild the frame system from the newly updated robot.
// NOTE(RDK-258): If remotes can trigger a local robot to reconfigure, you can cache the remoteParts in svc as well.
func (svc *frameSystemService) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	svc.partsMu.Lock()
	defer svc.partsMu.Unlock()

	ctx, span := trace.StartSpan(ctx, "services::framesystem::Reconfigure")
	defer span.End()

	components := make(map[string]resource.Resource)
	for name, r := range deps {
		short := name.ShortName()
		// is this only for InputEnabled components or everything?
		if _, present := components[short]; present {
			DuplicateResourceShortNameError(short)
		}
		components[short] = r
	}

	// TODO(rb): should this be done in the validate function instead?
	fsCfg, ok := conf.ConvertedAttributes.(Config)
	if !ok {
		return errors.New("could not read frame config")
	}

	sortedParts, err := TopologicallySort(fsCfg.Parts)
	if err != nil {
		return err
	}
	svc.parts = sortedParts
	svc.logger.Debugf("updated robot frame system:\n%v", (&Config{Parts: sortedParts}).String())
	return nil
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

	fs, err := svc.FrameSystem(ctx, []*referenceframe.LinkInFrame{})
	if err != nil {
		return nil, err
	}
	input := referenceframe.StartPositions(fs)

	svc.partsMu.RLock()
	defer svc.partsMu.RUnlock()

	// build maps of relevant components and inputs from initial inputs
	for name, inputs := range input {
		// skip frame if it does not have input
		if len(inputs) == 0 {
			continue
		}

		// add component to map
		component, ok := svc.components[name]
		if !ok {
			return nil, DependencyNotFoundError(name)
		}
		inputEnabled, ok := component.(referenceframe.InputEnabled)
		if !ok {
			return nil, NotInputEnabledError(component)
		}

		// add input to map
		pos, err := inputEnabled.CurrentInputs(ctx)
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

// CurrentInputs will get present inputs for a framesystem from a robot and return a map of those inputs, as well as a map of the
// InputEnabled resources that those inputs came from.
func (svc *frameSystemService) CurrentInputs(
	ctx context.Context,
) (map[string][]referenceframe.Input, map[string]referenceframe.InputEnabled, error) {
	fs, err := svc.FrameSystem(ctx, []*referenceframe.LinkInFrame{})
	if err != nil {
		return nil, nil, err
	}
	input := referenceframe.StartPositions(fs)

	// build maps of relevant components and inputs from initial inputs
	resources := map[string]referenceframe.InputEnabled{}
	for name, original := range input {
		// skip frames with no input
		if len(original) == 0 {
			continue
		}

		// add component to map
		component, ok := svc.components[name]
		if !ok {
			return nil, nil, DependencyNotFoundError(name)
		}
		inputEnabled, ok := component.(referenceframe.InputEnabled)
		if !ok {
			return nil, nil, NotInputEnabledError(component)
		}
		resources[name] = inputEnabled

		// add input to map
		pos, err := inputEnabled.CurrentInputs(ctx)
		if err != nil {
			return nil, nil, err
		}
		input[name] = pos
	}

	return input, resources, nil
}

// FrameSystem returns the frame system of the robot.
func (svc *frameSystemService) FrameSystem(
	ctx context.Context,
	additionalTransforms []*referenceframe.LinkInFrame,
) (referenceframe.FrameSystem, error) {
	ctx, span := trace.StartSpan(ctx, "services::framesystem::FrameSystem")
	defer span.End()
	return NewFrameSystemFromConfig(LocalFrameSystemName, &Config{
		Parts:                svc.parts,
		AdditionalTransforms: additionalTransforms,
	})
}

// TransformPointCloud applies the same pose offset to each point in a single pointcloud and returns the transformed point cloud.
// if destination string is empty, defaults to transforming to the world frame.
// Do not move the robot between the generation of the initial pointcloud and the receipt
// of the transformed pointcloud because that will make the transformations inaccurate.
func (svc *frameSystemService) TransformPointCloud(ctx context.Context, srcpc pointcloud.PointCloud, srcName, dstName string,
) (pointcloud.PointCloud, error) {
	if dstName == "" {
		dstName = referenceframe.World
	}
	if srcName == "" {
		return nil, errors.New("srcName cannot be empty, must provide name of point cloud origin")
	}
	// get transform pose needed to get to destination frame
	sourceFrameZero := referenceframe.NewPoseInFrame(srcName, spatialmath.NewZeroPose())
	theTransform, err := svc.TransformPose(ctx, sourceFrameZero, dstName, nil)
	if err != nil {
		return nil, err
	}
	// returned the transformed pointcloud where the transform was applied to each point
	return pointcloud.ApplyOffset(ctx, srcpc, theTransform.Pose(), svc.logger)
}
