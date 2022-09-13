// Package motion implements an motion service.
package motion

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	servicepb "go.viam.com/rdk/proto/api/service/motion/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.MotionService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterMotionServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &servicepb.MotionService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
		Reconfigurable: WrapWithReconfigurable,
	})
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	})
}

// A Service controls the flow of moving components.
type Service interface {
	Move(
		ctx context.Context,
		componentName resource.Name,
		destination *referenceframe.PoseInFrame,
		worldState *commonpb.WorldState,
	) (bool, error)
	MoveSingleComponent(
		ctx context.Context,
		componentName resource.Name,
		destination *referenceframe.PoseInFrame,
		worldState *commonpb.WorldState,
	) (bool, error)
	GetPose(
		ctx context.Context,
		componentName resource.Name,
		destinationFrame string,
		supplementalTransforms []*commonpb.Transform,
	) (*referenceframe.PoseInFrame, error)
}

var (
	_ = Service(&reconfigurableMotionService{})
	_ = resource.Reconfigurable(&reconfigurableMotionService{})
	_ = goutils.ContextCloser(&reconfigurableMotionService{})
)

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("motion")

// Subtype is a constant that identifies the motion service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named motion service's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((Service)(nil), actual)
}

// FromRobot is a helper for getting the named motion service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	resource, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(resource)
	}
	return svc, nil
}

// New returns a new move and grab service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	return &motionService{
		r:      r,
		logger: logger,
	}, nil
}

type motionService struct {
	r      robot.Robot
	logger golog.Logger
}

// Move takes a goal location and will plan and execute a movement to move a component specified by its name to that destination.
func (ms *motionService) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *commonpb.WorldState,
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, "motion-service")
	logger := ms.r.Logger()

	// get goal frame
	goalFrameName := destination.FrameName()
	logger.Debugf("goal given in frame of %q", goalFrameName)

	frameSys, err := framesystem.RobotFrameSystem(ctx, ms.r, worldState.GetTransforms())
	if err != nil {
		return false, err
	}
	solver := motionplan.NewSolvableFrameSystem(frameSys, logger)

	// build maps of relevant components and inputs from initial inputs
	fsInputs, resources, err := framesystem.RobotFsCurrentInputs(ctx, ms.r, solver)
	if err != nil {
		return false, err
	}

	logger.Debugf("frame system inputs: %v", fsInputs)

	// re-evaluate goalPose to be in the frame we're going to move in
	solvingFrame := referenceframe.World // TODO(erh): this should really be the parent of rootName
	tf, err := solver.Transform(fsInputs, destination, solvingFrame)
	if err != nil {
		return false, err
	}
	goalPose, _ := tf.(*referenceframe.PoseInFrame)

	// the goal is to move the component to goalPose which is specified in coordinates of goalFrameName
	output, err := solver.SolveWaypointsWithOptions(ctx,
		fsInputs,
		[]*referenceframe.PoseInFrame{goalPose},
		componentName.Name,
		worldState,
		[]map[string]interface{}{},
	)
	if err != nil {
		return false, err
	}

	// move all the components
	for _, step := range output {
		// TODO(erh): what order? parallel?
		for name, inputs := range step {
			if len(inputs) == 0 {
				continue
			}
			err := resources[name].GoToInputs(ctx, inputs)
			if err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

// MoveSingleComponent will pass through a move command to a component with a MoveToPosition method that takes a pose. Arms are the only
// component that supports this. This method will transform the destination pose, given in an arbitrary frame, into the pose of the arm.
// The arm will then move its most distal link to that pose. If you instead wish to move any other component than the arm end to that pose,
// then you must manually adjust the given destination by the transform from the arm end to the intended component.
func (ms *motionService) MoveSingleComponent(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *commonpb.WorldState,
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, "motion-service")
	logger := ms.r.Logger()

	components := robot.AllResourcesByName(ms.r, componentName.Name)
	if len(components) != 1 {
		return false, fmt.Errorf("got %d resources instead of 1 for (%s)", len(components), componentName.Name)
	}
	movableArm, ok := components[0].(arm.Arm)
	if !ok {
		return false, fmt.Errorf("%v(%T) is not an Arm and cannot MoveToPosition with a Pose", componentName.Name, components[0])
	}

	// get destination pose in frame of movable component
	goalPose := destination.Pose()
	if destination.FrameName() != componentName.Name {
		logger.Debugf("goal given in frame of %q", destination.FrameName())

		frameSys, err := framesystem.RobotFrameSystem(ctx, ms.r, worldState.GetTransforms())
		if err != nil {
			return false, err
		}
		// get the initial inputs
		fsInputs, _, err := framesystem.RobotFsCurrentInputs(ctx, ms.r, frameSys)
		if err != nil {
			return false, err
		}
		logger.Debugf("frame system inputs: %v", fsInputs)

		// re-evaluate goalPose to be in the frame we're going to move in
		tf, err := frameSys.Transform(fsInputs, destination, componentName.Name+"_origin")
		if err != nil {
			return false, err
		}
		goalPoseInFrame, _ := tf.(*referenceframe.PoseInFrame)
		goalPose = goalPoseInFrame.Pose()
		logger.Debugf("converted goal pose %q", spatialmath.PoseToProtobuf(goalPose))
	}

	err := movableArm.MoveToPosition(ctx, spatialmath.PoseToProtobuf(goalPose), worldState, nil)
	if err == nil {
		return true, nil
	}
	return false, err
}

func (ms *motionService) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	if destinationFrame == "" {
		destinationFrame = referenceframe.World
	}
	return ms.r.TransformPose(
		ctx,
		referenceframe.NewPoseInFrame(
			componentName.Name,
			spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0}),
		),
		destinationFrame,
		supplementalTransforms,
	)
}

type reconfigurableMotionService struct {
	mu     sync.RWMutex
	actual Service
}

func (svc *reconfigurableMotionService) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *commonpb.WorldState,
) (bool, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.Move(ctx, componentName, destination, worldState)
}

func (svc *reconfigurableMotionService) MoveSingleComponent(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *commonpb.WorldState,
) (bool, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.MoveSingleComponent(ctx, componentName, destination, worldState)
}

func (svc *reconfigurableMotionService) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetPose(ctx, componentName, destinationFrame, supplementalTransforms)
}

func (svc *reconfigurableMotionService) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return goutils.TryClose(ctx, svc.actual)
}

// Reconfigure replaces the old Motion Service with a new Motion Service.
func (svc *reconfigurableMotionService) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableMotionService)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := goutils.TryClose(ctx, svc.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a Motion Service as a Reconfigurable.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(s)
	}

	if reconfigurable, ok := s.(*reconfigurableMotionService); ok {
		return reconfigurable, nil
	}

	return &reconfigurableMotionService{actual: svc}, nil
}
