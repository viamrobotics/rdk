// Package client contains a gRPC based robot.Robot client.
package client

import (
	"context"
	"runtime/debug"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	metadataclient "go.viam.com/rdk/grpc/metadata/client"
	pb "go.viam.com/rdk/proto/api/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
)

// errUnimplemented is used for any unimplemented methods that should
// eventually be implemented server side or faked client side.
var errUnimplemented = errors.New("unimplemented")

// RobotClient satisfies the robot.Robot interface through a gRPC based
// client conforming to the robot.proto contract.
type RobotClient struct {
	address        string
	conn           rpc.ClientConn
	client         pb.RobotServiceClient
	metadataClient *metadataclient.MetadataServiceClient

	namesMu       *sync.RWMutex
	functionNames []string
	serviceNames  []string
	resourceNames []resource.Name

	activeBackgroundWorkers *sync.WaitGroup
	cancelBackgroundWorkers func()
	logger                  golog.Logger

	cachingStatus  bool
	cachedStatus   *pb.Status
	cachedStatusMu *sync.Mutex

	closeContext context.Context
}

// New constructs a new RobotClient that is served at the given address. The given
// context can be used to cancel the operation.
func New(ctx context.Context, address string, logger golog.Logger, opts ...RobotClientOption) (*RobotClient, error) {
	var rOpts robotClientOpts
	for _, opt := range opts {
		opt.apply(&rOpts)
	}

	conn, err := grpc.Dial(ctx, address, logger, rOpts.dialOptions...)
	if err != nil {
		return nil, err
	}

	metadataClient, err := metadataclient.New(ctx, address, logger, rOpts.dialOptions...)
	if err != nil {
		return nil, err
	}

	client := pb.NewRobotServiceClient(conn)
	closeCtx, cancel := context.WithCancel(context.Background())
	rc := &RobotClient{
		address:                 address,
		conn:                    conn,
		client:                  client,
		metadataClient:          metadataClient,
		cancelBackgroundWorkers: cancel,
		namesMu:                 &sync.RWMutex{},
		activeBackgroundWorkers: &sync.WaitGroup{},
		logger:                  logger,
		cachedStatusMu:          &sync.Mutex{},
		closeContext:            closeCtx,
	}
	// refresh once to hydrate the robot.
	if err := rc.Refresh(ctx); err != nil {
		return nil, multierr.Combine(err, metadataClient.Close(), conn.Close())
	}
	if rOpts.refreshEvery != 0 {
		rc.cachingStatus = true
		rc.activeBackgroundWorkers.Add(1)
		utils.ManagedGo(func() {
			rc.RefreshEvery(closeCtx, rOpts.refreshEvery)
		}, rc.activeBackgroundWorkers.Done)
	}
	return rc, nil
}

// Close cleanly closes the underlying connections and stops the refresh goroutine
// if it is running.
func (rc *RobotClient) Close(ctx context.Context) error {
	rc.cancelBackgroundWorkers()
	rc.activeBackgroundWorkers.Wait()

	return multierr.Combine(rc.conn.Close(), rc.metadataClient.Close())
}

// RefreshEvery refreshes the robot on the interval given by every until the
// given context is done.
func (rc *RobotClient) RefreshEvery(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		if !utils.SelectContextOrWaitChan(ctx, ticker.C) {
			return
		}

		if err := rc.Refresh(ctx); err != nil {
			// we want to keep refreshing and hopefully the ticker is not
			// too fast so that we do not thrash.
			rc.Logger().Errorw("failed to refresh status", "error", err)
		}
	}
}

// storeStatus atomically stores the status response from a robot server if and only
// if we are automatically refreshing.
func (rc *RobotClient) storeStatus(status *pb.Status) {
	if !rc.cachingStatus {
		return
	}
	rc.cachedStatusMu.Lock()
	rc.cachedStatus = status
	rc.cachedStatusMu.Unlock()
}

// storeStatus atomically gets the status response from a robot server if and only
// if we are automatically refreshing.
func (rc *RobotClient) getCachedStatus() *pb.Status {
	if !rc.cachingStatus {
		return nil
	}
	rc.cachedStatusMu.Lock()
	defer rc.cachedStatusMu.Unlock()
	return rc.cachedStatus
}

// status actually gets the latest status from the server.
func (rc *RobotClient) status(ctx context.Context) (*pb.Status, error) {
	resp, err := rc.client.Status(ctx, &pb.StatusRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

// Status either gets a cached or latest version of the status of the remote
// robot.
func (rc *RobotClient) Status(ctx context.Context) (*pb.Status, error) {
	if status := rc.getCachedStatus(); status != nil {
		return status, nil
	}
	return rc.status(ctx)
}

// Config gets the config from the remote robot
// It is only partial a config, including the pieces relevant to remote robots,
// And not the pieces relevant to local configuration (pins, security keys, etc...)
func (rc *RobotClient) Config(ctx context.Context) (*config.Config, error) {
	remoteConfig, err := rc.client.Config(ctx, &pb.ConfigRequest{})
	if err != nil {
		return nil, err
	}

	var cfg config.Config
	for _, c := range remoteConfig.Components {
		cc := config.Component{
			Name: c.Name,
			Type: config.ComponentType(c.Type),
		}
		// check if component has frame attribute, leave as nil if it doesn't
		if c.Parent != "" {
			cc.Frame = &config.Frame{Parent: c.Parent}
		}
		if cc.Frame != nil && c.Pose != nil {
			cc.Frame.Translation = spatialmath.Translation{
				X: c.Pose.X,
				Y: c.Pose.Y,
				Z: c.Pose.Z,
			}
			cc.Frame.Orientation = &spatialmath.OrientationVectorDegrees{
				OX:    c.Pose.OX,
				OY:    c.Pose.OY,
				OZ:    c.Pose.OZ,
				Theta: c.Pose.Theta,
			}
		}
		cfg.Components = append(cfg.Components, cc)
	}
	return &cfg, nil
}

// RemoteByName returns a remote robot by name. It is assumed to exist on the
// other end. Right now this method is unimplemented.
func (rc *RobotClient) RemoteByName(name string) (robot.Robot, bool) {
	debug.PrintStack()
	panic(errUnimplemented)
}

// BaseByName returns a base by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) BaseByName(name string) (base.Base, bool) {
	resource, ok := rc.ResourceByName(base.Named(name))
	if !ok {
		return nil, false
	}
	actualBase, ok := resource.(base.Base)
	if !ok {
		return nil, false
	}
	return actualBase, true
}

// CameraByName returns a camera by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) CameraByName(name string) (camera.Camera, bool) {
	resource, ok := rc.ResourceByName(camera.Named(name))
	if !ok {
		return nil, false
	}
	actual, ok := resource.(camera.Camera)
	if !ok {
		return nil, false
	}
	return actual, true
}

// BoardByName returns a board by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) BoardByName(name string) (board.Board, bool) {
	resource, ok := rc.ResourceByName(board.Named(name))
	if !ok {
		return nil, false
	}
	actualBoard, ok := resource.(board.Board)
	if !ok {
		return nil, false
	}
	return actualBoard, true
}

// MotorByName returns a motor by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) MotorByName(name string) (motor.Motor, bool) {
	nameObj := motor.Named(name)
	resource, ok := rc.ResourceByName(nameObj)
	if !ok {
		return nil, false
	}
	actualMotor, ok := resource.(motor.Motor)
	if !ok {
		return nil, false
	}
	return actualMotor, true
}

// ResourceByName returns resource by name.
func (rc *RobotClient) ResourceByName(name resource.Name) (interface{}, bool) {
	c := registry.ResourceSubtypeLookup(name.Subtype)
	if c == nil || c.RPCClient == nil {
		// registration doesn't exist
		return nil, false
	}
	// pass in conn
	resourceClient := c.RPCClient(rc.closeContext, rc.conn, name.Name, rc.Logger())
	return resourceClient, true
}

// Refresh manually updates the underlying parts of the robot based
// on a status retrieved from the server.
// TODO(https://github.com/viamrobotics/rdk/issues/57) - do not use status
// as we plan on making it a more expensive request with more details than
// needed for the purposes of this method.
func (rc *RobotClient) Refresh(ctx context.Context) (err error) {
	status, err := rc.status(ctx)
	if err != nil {
		return errors.Wrap(err, "status call failed")
	}

	rc.storeStatus(status)
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()

	// TODO: placeholder implementation
	// call metadata service.
	names, err := rc.metadataClient.Resources(ctx)
	// only return if it is not unimplemented - means a bigger error came up
	if err != nil && grpcstatus.Code(err) != codes.Unimplemented {
		return err
	}
	if err == nil {
		rc.resourceNames = make([]resource.Name, 0, len(names))
		for _, name := range names {
			newName := resource.NewName(
				resource.Namespace(name.Namespace),
				resource.TypeName(name.Type),
				resource.SubtypeName(name.Subtype),
				name.Name,
			)
			rc.resourceNames = append(rc.resourceNames, newName)
		}
	}

	rc.functionNames = nil
	if len(status.Functions) != 0 {
		rc.functionNames = make([]string, 0, len(status.Functions))
		for name := range status.Functions {
			rc.functionNames = append(rc.functionNames, name)
		}
	}
	rc.serviceNames = nil
	if len(status.Services) != 0 {
		rc.serviceNames = make([]string, 0, len(status.Services))
		for name := range status.Services {
			rc.serviceNames = append(rc.serviceNames, name)
		}
	}
	return nil
}

// copyStringSlice is a helper to simply copy a string slice
// so that no one mutates it.
func copyStringSlice(src []string) []string {
	out := make([]string, len(src))
	copy(out, src)
	return out
}

// RemoteNames returns the names of all known remotes.
func (rc *RobotClient) RemoteNames() []string {
	return nil
}

// CameraNames returns the names of all known cameras.
func (rc *RobotClient) CameraNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	names := []string{}
	for _, v := range rc.ResourceNames() {
		if v.Subtype == camera.Subtype {
			names = append(names, v.Name)
		}
	}
	return copyStringSlice(names)
}

// BaseNames returns the names of all known bases.
func (rc *RobotClient) BaseNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	names := []string{}
	for _, v := range rc.ResourceNames() {
		if v.Subtype == base.Subtype {
			names = append(names, v.Name)
		}
	}
	return copyStringSlice(names)
}

// BoardNames returns the names of all known boards.
func (rc *RobotClient) BoardNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	names := []string{}
	for _, v := range rc.ResourceNames() {
		if v.Subtype == board.Subtype {
			names = append(names, v.Name)
		}
	}
	return copyStringSlice(names)
}

// MotorNames returns the names of all known motors.
func (rc *RobotClient) MotorNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	names := []string{}
	for _, res := range rc.ResourceNames() {
		if res.Subtype == motor.Subtype {
			names = append(names, res.Name)
		}
	}
	return copyStringSlice(names)
}

// FunctionNames returns the names of all known functions.
func (rc *RobotClient) FunctionNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	return copyStringSlice(rc.functionNames)
}

// ProcessManager returns a useless process manager for the sake of
// satisfying the robot.Robot interface. Maybe it should not be part
// of the interface!
func (rc *RobotClient) ProcessManager() pexec.ProcessManager {
	return pexec.NoopProcessManager
}

// ResourceNames returns all resource names.
func (rc *RobotClient) ResourceNames() []resource.Name {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	names := []resource.Name{}
	for _, v := range rc.resourceNames {
		names = append(
			names,
			resource.NewName(
				v.Namespace, v.ResourceType, v.ResourceSubtype, v.Name,
			),
		)
	}
	return names
}

// Logger returns the logger being used for this robot.
func (rc *RobotClient) Logger() golog.Logger {
	return rc.logger
}

// FrameSystem retrieves an ordered slice of the frame configs and then builds a FrameSystem from the configs.
func (rc *RobotClient) FrameSystem(ctx context.Context, name, prefix string) (referenceframe.FrameSystem, error) {
	fs := referenceframe.NewEmptySimpleFrameSystem(name)
	// request the full config from the remote robot's frame system service.FrameSystemConfig()
	resp, err := rc.client.FrameServiceConfig(ctx, &pb.FrameServiceConfigRequest{})
	if err != nil {
		return nil, err
	}
	configs := resp.FrameSystemConfigs
	// using the configs, build a FrameSystem using model frames and static offset frames, the configs slice should already be sorted.
	for _, conf := range configs {
		part, err := config.ProtobufToFrameSystemPart(conf)
		if err != nil {
			return nil, err
		}
		// rename everything with prefixes
		part.Name = prefix + part.Name
		if part.FrameConfig.Parent != referenceframe.World {
			part.FrameConfig.Parent = prefix + part.FrameConfig.Parent
		}
		// make the frames from the configs
		modelFrame, staticOffsetFrame, err := config.CreateFramesFromPart(part, rc.Logger())
		if err != nil {
			return nil, err
		}
		// attach static offset frame to parent, attach model frame to static offset frame
		err = fs.AddFrame(staticOffsetFrame, fs.GetFrame(part.FrameConfig.Parent))
		if err != nil {
			return nil, err
		}
		err = fs.AddFrame(modelFrame, staticOffsetFrame)
		if err != nil {
			return nil, err
		}
	}
	return fs, nil
}
