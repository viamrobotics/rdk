// Package client contains a gRPC based robot.Robot client.
package client

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
)

// errUnimplemented is used for any unimplemented methods that should
// eventually be implemented server side or faked client side.
var errUnimplemented = errors.New("unimplemented")

// RobotClient satisfies the robot.Robot interface through a gRPC based
// client conforming to the robot.proto contract.
type RobotClient struct {
	address         string
	conn            rpc.ClientConn
	client          pb.RobotServiceClient
	refClient       *grpcreflect.Client
	dialOptions     []rpc.DialOption
	children        map[resource.Name]interface{}
	checkedChildren map[resource.Name]bool

	mu                  *sync.RWMutex
	resourceNames       []resource.Name
	resourceRPCSubtypes []resource.RPCSubtype

	connected       bool
	changeChan      chan bool
	connectedBefore bool

	activeBackgroundWorkers *sync.WaitGroup
	cancelBackgroundWorkers func()
	logger                  golog.Logger

	notifyParent func()

	closeContext context.Context
}

// New constructs a new RobotClient that is served at the given address. The given
// context can be used to cancel the operation.
func New(ctx context.Context, address string, logger golog.Logger, opts ...RobotClientOption) (*RobotClient, error) {
	var rOpts robotClientOpts
	for _, opt := range opts {
		opt.apply(&rOpts)
	}
	closeCtx, cancel := context.WithCancel(ctx)

	rc := &RobotClient{
		address:                 address,
		cancelBackgroundWorkers: cancel,
		mu:                      &sync.RWMutex{},
		activeBackgroundWorkers: &sync.WaitGroup{},
		logger:                  logger,
		closeContext:            closeCtx,
		dialOptions:             rOpts.dialOptions,
		notifyParent:            nil,
		children:                make(map[resource.Name]interface{}),
	}
	if err := rc.connect(ctx); err != nil {
		return nil, err
	}

	// refresh once to hydrate the robot.
	if err := rc.Refresh(ctx); err != nil {
		return nil, multierr.Combine(err, rc.conn.Close())
	}

	var refreshTime time.Duration
	if rOpts.refreshEvery == nil {
		refreshTime = 10 * time.Second
	} else {
		refreshTime = *rOpts.refreshEvery
	}

	if refreshTime > 0 {
		rc.activeBackgroundWorkers.Add(1)
		utils.ManagedGo(func() {
			rc.RefreshEvery(closeCtx, refreshTime)
		}, rc.activeBackgroundWorkers.Done)
	}

	if rOpts.checkConnectedEvery != 0 {
		rc.activeBackgroundWorkers.Add(1)
		utils.ManagedGo(func() {
			rc.checkConnection(closeCtx, rOpts.checkConnectedEvery, rOpts.reconnectEvery)
		}, rc.activeBackgroundWorkers.Done)
	}

	return rc, nil
}

// SetParentNotifier set the notifier function, robot client will use that the relay changes.
func (rc *RobotClient) SetParentNotifier(f func()) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.notifyParent = f
}

// Connected exposes whether a robot client is connected to the remote.
func (rc *RobotClient) Connected() bool {
	return rc.connected
}

// Changed watches for whether the remote has changed.
func (rc *RobotClient) Changed() <-chan bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.changeChan == nil {
		rc.changeChan = make(chan bool)
	}
	return rc.changeChan
}

func (rc *RobotClient) connect(ctx context.Context) error {
	if rc.conn != nil {
		if err := rc.conn.Close(); err != nil {
			return err
		}
	}
	conn, err := grpc.Dial(ctx, rc.address, rc.logger, rc.dialOptions...)
	if err != nil {
		return err
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	client := pb.NewRobotServiceClient(conn)

	refClient := grpcreflect.NewClient(rc.closeContext, reflectpb.NewServerReflectionClient(conn))

	rc.conn = conn
	rc.client = client
	rc.refClient = refClient
	rc.connected = true
	if rc.connectedBefore {
		// refresh first to get the new resources
		err = rc.updateResources(ctx)
		if err != nil {
			return err
		}
		// 1. get new resourceNames that are actually in the config now
		// 2. check those against the clients that were previously created
		// 3. remove the remaining clients that were not touched/no longer exist
		for _, name := range rc.resourceNames {
			if client := rc.children[name]; client != nil {
				// that means this client was previously initiated and called
				newClient, err := rc.createClient(name)
				if err != nil {
					return err
				}
				reconfigurableNewClient, ok := newClient.(resource.Reconfigurable)
				if !ok {
					rc.logger.Errorw("new client is not reconfigurable")
					continue
				}
				reconfigurableClient, ok := client.(resource.Reconfigurable)
				if !ok {
					rc.logger.Errorw("original client is not reconfigurable")
					continue
				}
				err = reconfigurableClient.Reconfigure(ctx, reconfigurableNewClient)
				if err != nil {
					return err
				}
			}
		}

		for childName, checked := range rc.checkedChildren {
			if !checked {
				child := rc.children[childName]
				if err := utils.TryClose(ctx, child); err != nil {
					return err
				}
				rc.children[childName] = nil
				rc.checkedChildren[childName] = false
			}
		}
	} else {
		rc.connectedBefore = true
	}

	if rc.changeChan != nil {
		rc.changeChan <- true
	}
	if rc.notifyParent != nil {
		rc.notifyParent()
	}
	return nil
}

// checkConnection either checks if the client is still connected, or attempts to reconnect to the remote.
func (rc *RobotClient) checkConnection(ctx context.Context, checkEvery time.Duration, reconnectEvery time.Duration) {
	for {
		var waitTime time.Duration
		if rc.Connected() {
			waitTime = checkEvery
		} else {
			if reconnectEvery != 0 {
				waitTime = reconnectEvery
			} else {
				// if reconnectEvery is unset, we will not attempt to reconnect
				return
			}
		}
		if !utils.SelectContextOrWait(ctx, waitTime) {
			return
		}
		if !rc.Connected() {
			rc.Logger().Debugw("trying to reconnect to remote at address", "address", rc.address)
			if err := rc.connect(ctx); err != nil {
				rc.Logger().Debugw("failed to reconnect remote", "error", err, "address", rc.address)
				continue
			}
			rc.Logger().Debugw("successfully reconnected remote at address", "address", rc.address)
		} else {
			check := func() error {
				timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				if _, _, err := rc.resources(timeoutCtx); err != nil {
					return err
				}
				return nil
			}
			var outerError error
			for attempt := 0; attempt < 3; attempt++ {
				err := check()
				if err != nil {
					outerError = err
					// if pipe is closed, we know for sure we lost connection
					if strings.Contains(err.Error(), "read/write on closed pipe") {
						break
					} else {
						// otherwise retry
						continue
					}
				} else {
					outerError = nil
					break
				}
			}
			if outerError != nil {
				rc.Logger().Errorw(
					"lost connection to remote",
					"error", outerError,
					"address", rc.address,
					"reconnect_interval", reconnectEvery.Seconds(),
				)
				rc.mu.Lock()
				rc.connected = false
				if rc.changeChan != nil {
					rc.changeChan <- true
				}
				rc.Logger().Debugf("connection was lost for remote %q", rc.address)
				if rc.notifyParent != nil {
					rc.Logger().Debugf("connection was lost for remote %q", rc.address)
					rc.notifyParent()
				}
				rc.mu.Unlock()
			}
		}
	}
}

// Close cleanly closes the underlying connections and stops the refresh goroutine
// if it is running.
func (rc *RobotClient) Close(ctx context.Context) error {
	rc.cancelBackgroundWorkers()
	rc.activeBackgroundWorkers.Wait()
	if rc.changeChan != nil {
		close(rc.changeChan)
		rc.changeChan = nil
	}
	rc.refClient.Reset()
	return rc.conn.Close()
}

func (rc *RobotClient) checkConnected() error {
	if !rc.Connected() {
		return errors.Errorf("not connected to remote robot at %s", rc.address)
	}
	return nil
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

// RemoteByName returns a remote robot by name. It is assumed to exist on the
// other end. Right now this method is unimplemented.
func (rc *RobotClient) RemoteByName(name string) (robot.Robot, bool) {
	panic(errUnimplemented)
}

// ResourceByName returns resource by name.
func (rc *RobotClient) ResourceByName(name resource.Name) (interface{}, error) {
	if err := rc.checkConnected(); err != nil {
		return nil, err
	}
	if client := rc.children[name]; client != nil {
		return client, nil
	}
	resourceClient, err := rc.createClient(name)
	if err != nil {
		return nil, err
	}
	rc.children[name] = resourceClient
	return resourceClient, nil
}

func (rc *RobotClient) createClient(name resource.Name) (interface{}, error) {
	c := registry.ResourceSubtypeLookup(name.Subtype)
	if c == nil || c.RPCClient == nil {
		if name.Namespace != resource.ResourceNamespaceRDK {
			return grpc.NewForeignResource(name, rc.conn), nil
		}
		// registration doesn't exist
		return nil, errors.New("resource client registration doesn't exist")
	}
	// pass in conn
	nameR := name.ShortName()
	resourceClient := c.RPCClient(rc.closeContext, rc.conn, nameR, rc.Logger())
	if c.Reconfigurable == nil {
		return resourceClient, nil
	}
	return c.Reconfigurable(resourceClient)
}

func (rc *RobotClient) resources(ctx context.Context) ([]resource.Name, []resource.RPCSubtype, error) {
	resp, err := rc.client.ResourceNames(ctx, &pb.ResourceNamesRequest{})
	if err != nil {
		return nil, nil, err
	}

	var resTypes []resource.RPCSubtype
	typesResp, err := rc.client.ResourceRPCSubtypes(ctx, &pb.ResourceRPCSubtypesRequest{})
	if err == nil {
		reflSource := grpcurl.DescriptorSourceFromServer(ctx, rc.refClient)

		resTypes = make([]resource.RPCSubtype, 0, len(typesResp.ResourceRpcSubtypes))
		for _, resSubtype := range typesResp.ResourceRpcSubtypes {
			symDesc, err := reflSource.FindSymbol(resSubtype.ProtoService)
			if err != nil {
				// Note: This happens right now if a client is talking to a main server
				// that has a remote or similarly if a server is talking to a remote that
				// has a remote. This can be solved by either integrating reflection into
				// robot.proto or by overriding the gRPC reflection service to return
				// reflection results from its remotes.
				rc.Logger().Debugw("failed to find symbol for resource subtype", "subtype", resSubtype, "error", err)
				continue
			}
			svcDesc, ok := symDesc.(*desc.ServiceDescriptor)
			if !ok {
				return nil, nil, errors.Errorf("expected descriptor to be service descriptor but got %T", symDesc)
			}
			resTypes = append(resTypes, resource.RPCSubtype{
				Subtype: protoutils.ResourceNameFromProto(resSubtype.Subtype).Subtype,
				Desc:    svcDesc,
			})
		}
	} else {
		if s, ok := status.FromError(err); !(ok && (s.Code() == codes.Unimplemented)) {
			return nil, nil, err
		}
	}

	resources := make([]resource.Name, 0, len(resp.Resources))

	for _, name := range resp.Resources {
		newName := protoutils.ResourceNameFromProto(name)
		resources = append(resources, newName)
	}

	return resources, resTypes, nil
}

// Refresh manually updates the underlying parts of the robot based
// on its metadata response.
func (rc *RobotClient) Refresh(ctx context.Context) (err error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if err := rc.checkConnected(); err != nil {
		return err
	}
	return rc.updateResources(ctx)
}

func (rc *RobotClient) updateResources(ctx context.Context) error {
	// call metadata service.
	names, rpcSubtypes, err := rc.resources(ctx)
	// only return if it is not unimplemented - means a bigger error came up
	if err != nil && status.Code(err) != codes.Unimplemented {
		return err
	}
	if err == nil {
		rc.resourceNames = make([]resource.Name, 0, len(names))
		rc.resourceNames = append(rc.resourceNames, names...)
		rc.resourceRPCSubtypes = rpcSubtypes
	}

	return nil
}

// RemoteNames returns the names of all known remotes.
func (rc *RobotClient) RemoteNames() []string {
	return nil
}

// ProcessManager returns a useless process manager for the sake of
// satisfying the robot.Robot interface. Maybe it should not be part
// of the interface!
func (rc *RobotClient) ProcessManager() pexec.ProcessManager {
	return pexec.NoopProcessManager
}

// OperationManager returns nil.
func (rc *RobotClient) OperationManager() *operation.Manager {
	return nil
}

// ResourceNames returns all resource names.
func (rc *RobotClient) ResourceNames() []resource.Name {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if err := rc.checkConnected(); err != nil {
		rc.Logger().Errorw("failed to get remote resource names", "error", err)
		return nil
	}
	names := make([]resource.Name, 0, len(rc.resourceNames))
	for _, v := range rc.resourceNames {
		rName := resource.NewName(v.Namespace, v.ResourceType, v.ResourceSubtype, v.Name)
		names = append(
			names,
			rName.PrependRemote(v.Remote),
		)
	}
	return names
}

// ResourceRPCSubtypes returns a list of all known resource subtypes.
func (rc *RobotClient) ResourceRPCSubtypes() []resource.RPCSubtype {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if err := rc.checkConnected(); err != nil {
		rc.Logger().Errorw("failed to get remote resource types", "error", err)
		return nil
	}
	subtypes := make([]resource.RPCSubtype, 0, len(rc.resourceRPCSubtypes))
	for _, v := range rc.resourceRPCSubtypes {
		vCopy := v
		subtypes = append(
			subtypes,
			vCopy,
		)
	}
	return subtypes
}

// Logger returns the logger being used for this robot.
func (rc *RobotClient) Logger() golog.Logger {
	return rc.logger
}

// DiscoverComponents takes a list of discovery queries and returns corresponding
// component configurations.
func (rc *RobotClient) DiscoverComponents(ctx context.Context, qs []discovery.Query) ([]discovery.Discovery, error) {
	pbQueries := make([]*pb.DiscoveryQuery, 0, len(qs))
	for _, q := range qs {
		pbQueries = append(
			pbQueries,
			&pb.DiscoveryQuery{Subtype: string(q.SubtypeName), Model: q.Model},
		)
	}

	resp, err := rc.client.DiscoverComponents(ctx, &pb.DiscoverComponentsRequest{Queries: pbQueries})
	if err != nil {
		return nil, err
	}

	discoveries := make([]discovery.Discovery, 0, len(resp.Discovery))
	for _, disc := range resp.Discovery {
		q := discovery.Query{
			SubtypeName: resource.SubtypeName(disc.Query.Subtype),
			Model:       disc.Query.Model,
		}
		discoveries = append(
			discoveries, discovery.Discovery{
				Query:   q,
				Results: disc.Results.AsMap(),
			})
	}
	return discoveries, nil
}

// FrameSystemConfig returns the info of each individual part that makes up the frame system.
func (rc *RobotClient) FrameSystemConfig(ctx context.Context, additionalTransforms []*commonpb.Transform) (framesystemparts.Parts, error) {
	resp, err := rc.client.FrameSystemConfig(ctx, &pb.FrameSystemConfigRequest{
		SupplementalTransforms: additionalTransforms,
	})
	if err != nil {
		return nil, err
	}
	cfgs := resp.GetFrameSystemConfigs()
	result := make([]*config.FrameSystemPart, 0, len(cfgs))
	for _, cfg := range cfgs {
		part, err := config.ProtobufToFrameSystemPart(cfg)
		if err != nil {
			return nil, err
		}
		result = append(result, part)
	}
	return framesystemparts.Parts(result), nil
}

// TransformPose will transform the pose of the requested poseInFrame to the desired frame in the robot's frame system.
func (rc *RobotClient) TransformPose(
	ctx context.Context,
	query *referenceframe.PoseInFrame,
	destination string,
	additionalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	resp, err := rc.client.TransformPose(ctx, &pb.TransformPoseRequest{
		Destination:            destination,
		Source:                 referenceframe.PoseInFrameToProtobuf(query),
		SupplementalTransforms: additionalTransforms,
	})
	if err != nil {
		return nil, err
	}
	return referenceframe.ProtobufToPoseInFrame(resp.Pose), nil
}

// GetStatus takes a list of resource names and returns their corresponding statuses. If no names are passed in, return all statuses.
func (rc *RobotClient) GetStatus(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
	names := make([]*commonpb.ResourceName, 0, len(resourceNames))
	for _, name := range resourceNames {
		names = append(names, protoutils.ResourceNameToProto(name))
	}

	resp, err := rc.client.GetStatus(ctx, &pb.GetStatusRequest{ResourceNames: names})
	if err != nil {
		return nil, err
	}

	statuses := make([]robot.Status, 0, len(resp.Status))
	for _, status := range resp.Status {
		statuses = append(
			statuses, robot.Status{
				Name:   protoutils.ResourceNameFromProto(status.Name),
				Status: status.Status.AsMap(),
			})
	}
	return statuses, nil
}

// StopAll cancels all current and outstanding operations for the robot and stops all actuators and movement.
func (rc *RobotClient) StopAll(ctx context.Context, extra map[resource.Name]map[string]interface{}) error {
	e := []*pb.StopExtraParameters{}
	for name, params := range extra {
		param, err := structpb.NewStruct(params)
		if err != nil {
			rc.Logger().Warnf("failed to convert extra params for resource %s with error: %s", name.Name, err)
			continue
		}
		p := &pb.StopExtraParameters{
			Name:   protoutils.ResourceNameToProto(name),
			Params: param,
		}
		e = append(e, p)
	}
	_, err := rc.client.StopAll(ctx, &pb.StopAllRequest{Extra: e})
	return err
}
