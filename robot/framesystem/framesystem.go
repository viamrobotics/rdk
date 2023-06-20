// Package framesystem defines the frame system service which is responsible for managing a stateful frame system
package framesystem

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
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
	CachedParts(ctx context.Context) ([]*referenceframe.FrameSystemPart, error)
	FrameSystem(ctx context.Context, additionalTransforms []*referenceframe.LinkInFrame) (referenceframe.FrameSystem, error)
}

// FromDependencies is a helper for getting the framesystem from a collection of dependencies.
func FromDependencies(deps resource.Dependencies) (Service, error) {
	return resource.FromDependencies[Service](deps, InternalServiceName)
}

// New returns a new frame system service for the given robot.
func New(ctx context.Context, deps resource.Dependencies, logger golog.Logger) (Service, error) {
	fs := &frameSystemService{
		Named:      InternalServiceName.AsNamed(),
		components: make(map[string]resource.Resource),
		logger:     logger,
	}
	if err := fs.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: &Config{}}); err != nil {
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

// Config is a slice of *config.FrameSystemPart.
type Config struct {
	resource.TriviallyValidateConfig
	PartConfigs []*PartConfig
}

// PartConfig is the structure that encodes all the information needed for a frame system part.
// Only one of the two fields between FrameConfig and PreprocessedPart should be non-nil as they each provide the same
// information in different formats.
type PartConfig struct {
	Name        string
	FrameConfig *referenceframe.LinkConfig

	// PreprocessedPart is a field that if filled means that no further processing of the config is necessary
	// This is efficient because parts that are returned from remotes should not have to be turned into configs only
	// to be converted back into FrameSystemConfig once in the framesystem service
	PreprocessedPart *referenceframe.FrameSystemPart
}

// the frame system service collects all the relevant parts that make up the frame system from the robot
// configs, and the remote robot configs.
type frameSystemService struct {
	resource.Named
	resource.TriviallyCloseable
	components map[string]resource.Resource
	logger     golog.Logger

	parts   []*referenceframe.FrameSystemPart
	partsMu sync.RWMutex

	lastReconfigureError error
}

func (svc *frameSystemService) ConfigToParts(cfg *Config) (parts []*referenceframe.FrameSystemPart, err error) {
	for _, component := range cfg.PartConfigs {
		// remote parts will be preprocessed since their own frame system needed to parse them
		if component.PreprocessedPart != nil {
			parts = append(parts, component.PreprocessedPart)
		} else { // part is local to the robot and needs to be parsed
			frame := component.FrameConfig
			if frame.ID == "" {
				frame = frame.Rename(component.Name)
			}
			link, err := frame.ParseConfig()
			if err != nil {
				return nil, err
			}

			model, err := svc.extractModelFrameJSON(component.Name)
			if err != nil && !errors.Is(err, referenceframe.ErrNoModelInformation) {
				// When we have non-nil errors here, it is because the resource is not yet available.
				// In this case, we will exclude it from the FS and when it becomes available, it will be included.
				continue
			}
			parts = append(parts, &referenceframe.FrameSystemPart{FrameConfig: link, ModelFrame: model})
		}
	}
	return parts, nil
}

// Reconfigure will rebuild the frame system from the newly updated robot.
func (svc *frameSystemService) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	svc.partsMu.Lock()
	defer svc.partsMu.Unlock()

	_, span := trace.StartSpan(ctx, "services::framesystem::Reconfigure")
	defer span.End()

	components := make(map[string]resource.Resource)
	for name, r := range deps {
		short := name.ShortName()
		if _, present := components[short]; present {
			svc.parts = nil
			svc.lastReconfigureError = DuplicateResourceShortNameError(short)
			return svc.lastReconfigureError
		}
		components[short] = r
	}
	svc.components = components

	fsCfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		svc.parts = nil
		svc.lastReconfigureError = err
		return err
	}

	parts, err := svc.ConfigToParts(fsCfg)
	if err != nil {
		svc.parts = nil
		svc.lastReconfigureError = err
		return err
	}

	sortedParts, err := referenceframe.TopologicallySortParts(parts)
	if err != nil {
		svc.parts = nil
		svc.lastReconfigureError = err
		return err
	}
	svc.parts = sortedParts
	svc.lastReconfigureError = nil
	svc.logger.Debugf("updated robot frame system:\n%v", svc.String())
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

	fs, err := svc.FrameSystem(ctx, additionalTransforms)
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
	_, span := trace.StartSpan(ctx, "services::framesystem::FrameSystem")
	defer span.End()
	if svc.lastReconfigureError != nil {
		return nil, svc.lastReconfigureError
	}
	return referenceframe.NewFrameSystem(LocalFrameSystemName, svc.parts, additionalTransforms)
}

// CachedConfig returns the config which was most recently used to configure the framesystem service.
func (svc *frameSystemService) CachedParts(ctx context.Context) ([]*referenceframe.FrameSystemPart, error) {
	svc.partsMu.Lock()
	defer svc.partsMu.Unlock()

	if svc.lastReconfigureError != nil {
		return nil, svc.lastReconfigureError
	}
	return svc.parts, nil
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

// PrefixRemoteParts applies prefixes to a list of FrameSystemConfig appropriate to the remote they originate from.
func PrefixRemoteParts(parts []*referenceframe.FrameSystemPart, remoteName, remoteParent string) []*PartConfig {
	partConfigs := make([]*PartConfig, 0, len(parts))
	for _, part := range parts {
		if part.FrameConfig.Parent() == referenceframe.World { // rename World of remote parts
			part.FrameConfig.SetParent(remoteParent)
		}
		// rename each non-world part with prefix
		part.FrameConfig.SetName(remoteName + ":" + part.FrameConfig.Name())
		if part.FrameConfig.Parent() != remoteParent {
			part.FrameConfig.SetParent(remoteName + ":" + part.FrameConfig.Parent())
		}
		partConfigs = append(partConfigs, &PartConfig{PreprocessedPart: part})
	}
	return partConfigs
}

// String prints out a table of each frame in the system, with columns of name, parent, translation and orientation.
func (svc *frameSystemService) String() string {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"#", "Name", "Parent", "Translation", "Orientation", "Geometry"})
	t.AppendRow([]interface{}{"0", referenceframe.World, "", "", "", ""})
	for i, part := range svc.parts {
		pose := part.FrameConfig.Pose()
		tra := pose.Point()
		ori := pose.Orientation().EulerAngles()
		geomString := ""
		if gc := part.FrameConfig.Geometry(); gc != nil {
			geomString = gc.String()
		}
		t.AppendRow([]interface{}{
			fmt.Sprintf("%d", i+1),
			part.FrameConfig.Name(),
			part.FrameConfig.Parent(),
			fmt.Sprintf("X:%.0f, Y:%.0f, Z:%.0f", tra.X, tra.Y, tra.Z),
			fmt.Sprintf(
				"Roll:%.2f, Pitch:%.2f, Yaw:%.2f",
				utils.RadToDeg(ori.Roll),
				utils.RadToDeg(ori.Pitch),
				utils.RadToDeg(ori.Yaw),
			),
			geomString,
		})
	}
	return t.Render()
}

// extractModelFrameJSON finds the robot part with a given name, checks to see if it implements ModelFrame, and returns the
// JSON []byte if it does, or nil if it doesn't.
func (svc *frameSystemService) extractModelFrameJSON(name string) (referenceframe.Model, error) {
	part, ok := svc.components[name]
	if !ok {
		return nil, DependencyNotFoundError(name)
	}
	if framer, ok := part.(referenceframe.ModelFramer); ok {
		return framer.ModelFrame(), nil
	}
	return nil, referenceframe.ErrNoModelInformation
}
