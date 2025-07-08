// Package framesystem defines the frame system service which is responsible for managing a stateful frame system
package framesystem

import (
	"context"
	"fmt"
	"sync"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/logging"
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

// InputEnabled is a standard interface for all things that interact with the frame system
// This allows us to figure out where they currently are, and then move them.
// Input units are always in meters or radians.
type InputEnabled interface {
	Kinematics(ctx context.Context) (referenceframe.Model, error)
	CurrentInputs(ctx context.Context) ([]referenceframe.Input, error)
	GoToInputs(context.Context, ...[]referenceframe.Input) error
}

// Service is an interface that wraps a RobotFrameSystem in a Resource
type Service interface {
	resource.Resource
	RobotFrameSystem
}

// TODO this comment needs a bunch of work
//
// GetPose example:
//
//	// Insert code to connect to your machine.
//	// (see CONNECT tab of your machine's page in the Viam app)
//
//	// Assumes a gripper configured with name "my_gripper" on the machine
//	gripperName := gripper.Named("my_gripper")
//
//	// Access the motion service
//	motionService, err := motion.FromRobot(machine, "builtin")
//	if err != nil {
//	  logger.Fatal(err)
//	}
//
//	myGripperPose, err := motionService.GetPose(context.Background(), gripperName, referenceframe.World, nil, nil)
//	if err != nil {
//	  logger.Fatal(err)
//	}
//	logger.Info("Position of my_gripper from the motion service:", myGripperPose.Pose().Point())
//	logger.Info("Orientation of my_gripper from the motion service:", myGripperPose.Pose().Orientation())
//
// For more information, see the [GetPose method docs].
//
// TransformPose example:
//
//	// Define a Pose coincident with the world reference frame
//	firstPose := spatialmath.NewZeroPose
//
//	// Establish the world as the reference for firstPose
//	firstPoseInFrame := referenceframe.NewPoseInFrame(referenceframe.World, firstPose)
//
//	// Calculate firstPoseInFrame from the perspective of the origin frame of myArm
//	transformedPoseInFrame, err := machine.TransformPose(context.Background(), firstPoseInFrame, "myArm", nil)
//
// TransformPointCloud example:
//
//	// Create an empty slice to store point cloud data.
//	pointClouds := make([]pointcloud.PointCloud, 0)
//
//	// Transform the first point cloud in the list from its reference frame to the frame of 'myArm'.
//	transformed, err := fsService.TransformPointCloud(context.Background(), pointClouds[0], referenceframe.World, "myArm")
type RobotFrameSystem interface {
	// FrameSystemConfig returns the individual parts that make up a robot's frame system
	FrameSystemConfig(ctx context.Context) (*Config, error)

	// GetPose returns the pose a component within a frame system.
	// It returns a `PoseInFrame` describing the pose of the specified component relative to the specified destination frame.
	// The `supplemental_transforms` argument can be used to augment the machine's existing frame system with additional frames.
	GetPose(
		ctx context.Context,
		componentName, destinationFrame string,
		supplementalTransforms []*referenceframe.LinkInFrame,
		extra map[string]interface{},
	) (*referenceframe.PoseInFrame, error)

	// TransformPose returns a transformed pose in the destination reference frame.
	// This method converts a given source pose from one reference frame to a specified destination frame.
	TransformPose(
		ctx context.Context,
		pose *referenceframe.PoseInFrame,
		dst string,
		supplementalTransforms []*referenceframe.LinkInFrame,
	) (*referenceframe.PoseInFrame, error)

	// TransformPointCloud returns a new point cloud with points adjusted from one reference frame to a specified destination frame.
	TransformPointCloud(ctx context.Context, srcpc pointcloud.PointCloud, srcName, dstName string) (pointcloud.PointCloud, error)
}

// FromDependencies is a helper for getting the framesystem from a collection of dependencies.
func FromDependencies(deps resource.Dependencies) (Service, error) {
	return resource.FromDependencies[Service](deps, InternalServiceName)
}

// New returns a new frame system service for the given robot.
func New(ctx context.Context, deps resource.Dependencies, logger logging.Logger) (Service, error) {
	fs := &frameSystemService{
		Named:      InternalServiceName.AsNamed(),
		components: make([]resource.Resource, 0),
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

// TODO: remove AdditionalTransforms since they aren't something that can really be configured
// Then make the frame system constructor take a config
// Config is a slice of *config.FrameSystemPart.
type Config struct {
	resource.TriviallyValidateConfig
	Parts []*referenceframe.FrameSystemPart
}

// String prints out a table of each frame in the system, with columns of name, parent, translation and orientation.
func (cfg Config) String() string {
	if len(cfg.Parts) == 0 {
		return "empty frame system"
	}
	t := table.NewWriter()
	t.AppendHeader(table.Row{"#", "Name", "Parent", "Translation", "Orientation", "Geometry"})
	t.AppendRow([]interface{}{"0", referenceframe.World, "", "", "", ""})
	for i, part := range cfg.Parts {
		pose := part.FrameConfig.Pose()
		tra := pose.Point()
		ori := pose.Orientation().EulerAngles()
		geomString := ""
		if gc := part.FrameConfig.Geometry(); gc != nil {
			geomString = fmt.Sprint(gc)
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
	return "\n" + t.Render()
}

// the frame system service collects all the relevant parts that make up the frame system from the robot
// configs, and the remote robot configs.
type frameSystemService struct {
	resource.Named
	resource.TriviallyCloseable
	components []resource.Resource
	logger     logging.Logger

	parts   []*referenceframe.FrameSystemPart
	partsMu sync.RWMutex
}

// Reconfigure will rebuild the frame system from the newly updated robot.
func (svc *frameSystemService) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	svc.partsMu.Lock()
	defer svc.partsMu.Unlock()

	_, span := trace.StartSpan(ctx, "services::framesystem::Reconfigure")
	defer span.End()

	seen := make(map[string]resource.Resource)
	components := make([]resource.Resource, 0)
	for name, r := range deps {
		short := name.ShortName()
		if _, present := seen[short]; present {
			return DuplicateResourceShortNameError(short)
		}
		seen[short] = r
		components = append(components, r)
	}
	svc.components = components

	fsCfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	sortedParts, err := referenceframe.TopologicallySortParts(fsCfg.Parts)
	if err != nil {
		return err
	}
	svc.parts = sortedParts
	svc.logger.Debugf("reconfigured robot frame system: %v", (&Config{Parts: sortedParts}).String())
	return nil
}

func (svc *frameSystemService) FrameSystemConfig(ctx context.Context) (*Config, error) {
	return &Config{Parts: svc.parts}, nil
}

// GetPose returns the pose of the specified component in the given destination frame
func (svc *frameSystemService) GetPose(
	ctx context.Context,
	componentName, destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	if destinationFrame == "" {
		destinationFrame = referenceframe.World
	}
	if componentName == "" {
		return nil, errors.New("must provide component name")
	}
	return svc.TransformPose(
		ctx,
		referenceframe.NewPoseInFrame(componentName, spatialmath.NewZeroPose()),
		destinationFrame,
		supplementalTransforms,
	)
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

	fs, err := referenceframe.NewFrameSystem(LocalFrameSystemName, svc.parts, additionalTransforms)
	if err != nil {
		return nil, err
	}

	svc.partsMu.RLock()
	defer svc.partsMu.RUnlock()

	input, err := CurrentInputs(ctx, svc.components)
	if err != nil {
		return nil, err
	}

	tf, err := fs.Transform(input, pose, dst)
	if err != nil {
		return nil, err
	}
	return tf.(*referenceframe.PoseInFrame), nil
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
	pc := srcpc.CreateNewRecentered(theTransform.Pose())
	err = pointcloud.ApplyOffset(srcpc, theTransform.Pose(), pc)
	if err != nil {
		return nil, err
	}
	return pc, nil
}

func NewFromService(
	ctx context.Context,
	service Service,
	supplementalTransforms []*referenceframe.LinkInFrame,
) (referenceframe.FrameSystem, error) {
	fsCfg, err := service.FrameSystemConfig(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.NewFrameSystem(service.Name().ShortName(), fsCfg.Parts, supplementalTransforms)
}

// CurrentInputs will get the inputs of all provided dependencies
func CurrentInputs(ctx context.Context, components []resource.Resource) (referenceframe.FrameSystemInputs, error) {
	input := make(referenceframe.FrameSystemInputs)
	for _, res := range components {
		inputEnabled, ok := res.(InputEnabled)
		if !ok {
			continue
		}
		pos, err := inputEnabled.CurrentInputs(ctx)
		if err != nil {
			return nil, err
		}
		input[res.Name().ShortName()] = pos
	}
	return input, nil
}

// PrefixRemoteParts applies prefixes to a list of FrameSystemParts appropriate to the remote they originate from.
func PrefixRemoteParts(parts []*referenceframe.FrameSystemPart, remoteName, remoteParent string) {
	for _, part := range parts {
		if part.FrameConfig.Parent() == referenceframe.World { // rename World of remote parts
			part.FrameConfig.SetParent(remoteParent)
		}
		// rename each non-world part with prefix
		part.FrameConfig.SetName(remoteName + ":" + part.FrameConfig.Name())
		if part.FrameConfig.Parent() != remoteParent {
			part.FrameConfig.SetParent(remoteName + ":" + part.FrameConfig.Parent())
		}
	}
}
