// Package client contains a gRPC based robot.Robot client.
package client

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/fullstorydev/grpcurl"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/operation"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	"go.viam.com/rdk/session"
	rutils "go.viam.com/rdk/utils"
)

var (
	// ErrMissingClientRegistration is used when there is no resource client registered for the subtype.
	ErrMissingClientRegistration = errors.New("resource client registration doesn't exist")

	// errUnimplemented is used for any unimplemented methods that should
	// eventually be implemented server side or faked client side.
	errUnimplemented = errors.New("unimplemented")

	// resourcesTimeout is the default timeout for getting resources.
	resourcesTimeout = 5 * time.Second
)

// RobotClient satisfies the robot.Robot interface through a gRPC based
// client conforming to the robot.proto contract.
type RobotClient struct {
	remoteName      string
	address         string
	conn            rpc.ClientConn
	client          pb.RobotServiceClient
	refClient       *grpcreflect.Client
	dialOptions     []rpc.DialOption
	resourceClients map[resource.Name]interface{}
	remoteNameMap   map[resource.Name]resource.Name

	mu                  *sync.RWMutex
	resourceNames       []resource.Name
	resourceRPCSubtypes []resource.RPCSubtype

	connected  bool
	changeChan chan bool

	activeBackgroundWorkers *sync.WaitGroup
	cancelBackgroundWorkers func()
	logger                  golog.Logger

	notifyParent func()

	closeContext context.Context

	// sessions
	sessionsDisabled         bool
	sessionMu                sync.RWMutex
	sessionsSupported        *bool // when nil, we have not yet checked
	currentSessionID         string
	sessionHeartbeatInterval time.Duration
}

var exemptFromConnectionCheck = map[string]bool{
	"/proto.rpc.webrtc.v1.SignalingService/Call":                 true,
	"/proto.rpc.webrtc.v1.SignalingService/CallUpdate":           true,
	"/proto.rpc.webrtc.v1.SignalingService/OptionalWebRTCConfig": true,
	"/proto.rpc.v1.AuthService/Authenticate":                     true,
	"/proto.rpc.v1.ExternalAuthService/AuthenticateTo":           true,
}

func skipConnectionCheck(method string) bool {
	return exemptFromConnectionCheck[method]
}

func isClosedPipeError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), io.ErrClosedPipe.Error())
}

func (rc *RobotClient) notConnectedToRemoteError() error {
	return errors.Errorf("not connected to remote robot at %s", rc.address)
}

func (rc *RobotClient) handleUnaryDisconnect(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *googlegrpc.ClientConn,
	invoker googlegrpc.UnaryInvoker,
	opts ...googlegrpc.CallOption,
) error {
	if skipConnectionCheck(method) {
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	if err := rc.checkConnected(); err != nil {
		rc.Logger().Debugw("connection is down, skipping method call", "method", method)
		return status.Error(codes.Unavailable, err.Error())
	}

	err := invoker(ctx, method, req, reply, cc, opts...)
	// we might lose connection before our background check detects it - in this case we
	// should still surface a helpful error message.
	if isClosedPipeError(err) {
		return status.Error(codes.Unavailable, rc.notConnectedToRemoteError().Error())
	}
	return err
}

type handleDisconnectClientStream struct {
	googlegrpc.ClientStream
	*RobotClient
}

func (cs *handleDisconnectClientStream) RecvMsg(m interface{}) error {
	if err := cs.RobotClient.checkConnected(); err != nil {
		return status.Error(codes.Unavailable, err.Error())
	}

	// we might lose connection before our background check detects it - in this case we
	// should still surface a helpful error message.
	err := cs.ClientStream.RecvMsg(m)
	if isClosedPipeError(err) {
		return status.Error(codes.Unavailable, cs.RobotClient.notConnectedToRemoteError().Error())
	}

	return err
}

func (rc *RobotClient) handleStreamDisconnect(
	ctx context.Context,
	desc *googlegrpc.StreamDesc,
	cc *googlegrpc.ClientConn,
	method string,
	streamer googlegrpc.Streamer,
	opts ...googlegrpc.CallOption,
) (googlegrpc.ClientStream, error) {
	if skipConnectionCheck(method) {
		return streamer(ctx, desc, cc, method, opts...)
	}

	if err := rc.checkConnected(); err != nil {
		rc.Logger().Debugw("connection is down, skipping method call", "method", method)
		return nil, status.Error(codes.Unavailable, err.Error())
	}

	cs, err := streamer(ctx, desc, cc, method, opts...)
	// we might lose connection before our background check detects it - in this case we
	// should still surface a helpful error message.
	if isClosedPipeError(err) {
		return nil, status.Error(codes.Unavailable, rc.notConnectedToRemoteError().Error())
	}
	return &handleDisconnectClientStream{cs, rc}, err
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
		remoteName:              rOpts.remoteName,
		address:                 address,
		cancelBackgroundWorkers: cancel,
		mu:                      &sync.RWMutex{},
		activeBackgroundWorkers: &sync.WaitGroup{},
		logger:                  logger,
		closeContext:            closeCtx,
		dialOptions:             rOpts.dialOptions,
		notifyParent:            nil,
		resourceClients:         make(map[resource.Name]interface{}),
		remoteNameMap:           make(map[resource.Name]resource.Name),
		sessionsDisabled:        rOpts.disableSessions,
	}

	// interceptors are applied in order from first to last
	rc.dialOptions = append(
		rc.dialOptions,
		// error handling
		rpc.WithUnaryClientInterceptor(rc.handleUnaryDisconnect),
		rpc.WithStreamClientInterceptor(rc.handleStreamDisconnect),
		// sessions
		rpc.WithUnaryClientInterceptor(grpc_retry.UnaryClientInterceptor()),
		rpc.WithStreamClientInterceptor(grpc_retry.StreamClientInterceptor()),
		rpc.WithUnaryClientInterceptor(rc.sessionUnaryClientInterceptor),
		rpc.WithStreamClientInterceptor(rc.sessionStreamClientInterceptor),
		// operations
		rpc.WithUnaryClientInterceptor(operation.UnaryClientInterceptor),
		rpc.WithStreamClientInterceptor(operation.StreamClientInterceptor),
	)

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
	var checkConnectedTime time.Duration
	if rOpts.checkConnectedEvery == nil {
		checkConnectedTime = 10 * time.Second
	} else {
		checkConnectedTime = *rOpts.checkConnectedEvery
	}
	var reconnectTime time.Duration
	if rOpts.reconnectEvery == nil {
		reconnectTime = 1 * time.Second
	} else {
		reconnectTime = *rOpts.reconnectEvery
	}

	if refreshTime > 0 {
		rc.activeBackgroundWorkers.Add(1)
		utils.ManagedGo(func() {
			rc.RefreshEvery(closeCtx, refreshTime)
		}, rc.activeBackgroundWorkers.Done)
	}

	if checkConnectedTime > 0 && reconnectTime > 0 {
		rc.activeBackgroundWorkers.Add(1)
		utils.ManagedGo(func() {
			rc.checkConnection(closeCtx, checkConnectedTime, reconnectTime)
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
	rc.mu.RLock()
	defer rc.mu.RUnlock()
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
	if len(rc.resourceClients) != 0 {
		if err := rc.updateResources(ctx, updateReasonReconnect); err != nil {
			return err
		}
	}

	if rc.changeChan != nil {
		rc.changeChan <- true
	}
	if rc.notifyParent != nil {
		rc.notifyParent()
	}
	return nil
}

type updateReason byte

const (
	updateReasonReconnect updateReason = iota
	updateReasonRefresh
)

func (rc *RobotClient) updateResourceClients(ctx context.Context, reason updateReason) error {
	activeResources := make(map[resource.Name]bool)

	for _, name := range rc.resourceNames {
		activeResources[name] = true
		switch reason {
		case updateReasonRefresh:
		case updateReasonReconnect:
			fallthrough
		default:
			if client, ok := rc.resourceClients[name]; ok {
				newClient, err := rc.createClient(name)
				if err != nil {
					return err
				}
				currResource, err := resource.ReconfigureResource(ctx, client, newClient)
				if err != nil {
					return err
				}
				rc.resourceClients[name] = currResource
			}
		}
	}

	for resourceName, client := range rc.resourceClients {
		// check if no longer an active resource
		if !activeResources[resourceName] {
			if err := utils.TryClose(ctx, client); err != nil {
				rc.Logger().Error(err)
				continue
			}
			delete(rc.resourceClients, resourceName)
		}
	}

	return nil
}

// checkConnection either checks if the client is still connected, or attempts to reconnect to the remote.
func (rc *RobotClient) checkConnection(ctx context.Context, checkEvery, reconnectEvery time.Duration) {
	for {
		var waitTime time.Duration
		if rc.connected {
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
		if !rc.connected {
			rc.Logger().Debugw("trying to reconnect to remote at address", "address", rc.address)
			if err := rc.connect(ctx); err != nil {
				rc.Logger().Debugw("failed to reconnect remote", "error", err, "address", rc.address)
				continue
			}
			rc.Logger().Debugw("successfully reconnected remote at address", "address", rc.address)
		} else {
			check := func() error {
				if _, _, err := rc.resources(ctx); err != nil {
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
					if isClosedPipeError(err) {
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
	if !rc.connected {
		return rc.notConnectedToRemoteError()
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
			rc.Logger().Errorw("failed to refresh resources from remote", "error", err)
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
	rc.mu.RLock()
	if err := rc.checkConnected(); err != nil {
		rc.mu.RUnlock()
		return nil, err
	}

	// see if a remote name matches the name if so then return the remote client
	if val, ok := rc.remoteNameMap[name]; ok {
		name = val
	}
	if client, ok := rc.resourceClients[name]; ok {
		rc.mu.RUnlock()
		return client, nil
	}
	rc.mu.RUnlock()

	rc.mu.Lock()
	defer rc.mu.Unlock()
	// another check, this one with a stricter lock
	if client, ok := rc.resourceClients[name]; ok {
		return client, nil
	}

	// finally, before adding a new resource, make sure this name exists and is known
	for _, knownName := range rc.resourceNames {
		if name == knownName {
			resourceClient, err := rc.createClient(name)
			if err != nil {
				return nil, err
			}
			rc.resourceClients[name] = resourceClient
			return resourceClient, nil
		}
	}
	return nil, rutils.NewResourceNotFoundError(name)
}

func (rc *RobotClient) createClient(name resource.Name) (interface{}, error) {
	c := registry.ResourceSubtypeLookup(name.Subtype)
	if c == nil || c.RPCClient == nil {
		if name.Namespace != resource.ResourceNamespaceRDK {
			return grpc.NewForeignResource(name, rc.conn), nil
		}
		// At this point we checked that the 'name' is in the rc.resourceNames list
		// and it is in the RDK namespace, so it's likely we provide a package for
		// interacting with it.
		rc.logger.Errorw("the client registration for resource doesn't exist, you may need to import relevant client package",
			"resource", name,
			"import_guess", fmt.Sprintf("go.viam.com/rdk/%s/%s/register", name.ResourceType, name.Subtype))
		return nil, ErrMissingClientRegistration
	}
	// pass in conn
	nameR := name.ShortName()
	resourceClient := c.RPCClient(rc.closeContext, rc.conn, nameR, rc.Logger())
	if c.Reconfigurable == nil {
		return resourceClient, nil
	}
	return c.Reconfigurable(resourceClient, name.PrependRemote(resource.RemoteName(rc.remoteName)))
}

func (rc *RobotClient) resources(ctx context.Context) ([]resource.Name, []resource.RPCSubtype, error) {
	ctx, cancel := context.WithTimeout(ctx, resourcesTimeout)
	defer cancel()
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
				Subtype: rprotoutils.ResourceNameFromProto(resSubtype.Subtype).Subtype,
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
		newName := rprotoutils.ResourceNameFromProto(name)
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
	return rc.updateResources(ctx, updateReasonRefresh)
}

func (rc *RobotClient) updateResources(ctx context.Context, reason updateReason) error {
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

	rc.updateRemoteNameMap()

	return rc.updateResourceClients(ctx, reason)
}

func (rc *RobotClient) updateRemoteNameMap() {
	tempMap := make(map[resource.Name]resource.Name)
	dupMap := make(map[resource.Name]bool)
	for _, n := range rc.resourceNames {
		if err := n.Validate(); err != nil {
			rc.Logger().Error(err)
			continue
		}
		tempName := resource.RemoveRemoteName(n)
		// If the short name already exists in the map then there is a collision and we make the long name empty.
		if _, ok := tempMap[tempName]; ok {
			dupMap[tempName] = true
		} else {
			tempMap[tempName] = n
		}
	}
	for key := range dupMap {
		delete(tempMap, key)
	}
	rc.remoteNameMap = tempMap
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

// SessionManager returns nil.
func (rc *RobotClient) SessionManager() session.Manager {
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
			&pb.DiscoveryQuery{Subtype: q.API.String(), Model: q.Model.String()},
		)
	}

	resp, err := rc.client.DiscoverComponents(ctx, &pb.DiscoverComponentsRequest{Queries: pbQueries})
	if err != nil {
		return nil, err
	}

	discoveries := make([]discovery.Discovery, 0, len(resp.Discovery))
	for _, disc := range resp.Discovery {
		m, err := resource.NewModelFromString(disc.Query.Model)
		if err != nil {
			return nil, err
		}
		s, err := resource.NewSubtypeFromString(disc.Query.Subtype)
		if err != nil {
			return nil, err
		}
		q := discovery.Query{
			API:   s,
			Model: m,
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
func (rc *RobotClient) FrameSystemConfig(
	ctx context.Context,
	additionalTransforms []*referenceframe.LinkInFrame,
) (framesystemparts.Parts, error) {
	transforms, err := referenceframe.LinkInFramesToTransformsProtobuf(additionalTransforms)
	if err != nil {
		return nil, err
	}
	resp, err := rc.client.FrameSystemConfig(ctx, &pb.FrameSystemConfigRequest{SupplementalTransforms: transforms})
	if err != nil {
		return nil, err
	}
	cfgs := resp.GetFrameSystemConfigs()
	result := make([]*referenceframe.FrameSystemPart, 0, len(cfgs))
	for _, cfg := range cfgs {
		part, err := referenceframe.ProtobufToFrameSystemPart(cfg)
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
	additionalTransforms []*referenceframe.LinkInFrame,
) (*referenceframe.PoseInFrame, error) {
	transforms, err := referenceframe.LinkInFramesToTransformsProtobuf(additionalTransforms)
	if err != nil {
		return nil, err
	}
	resp, err := rc.client.TransformPose(ctx, &pb.TransformPoseRequest{
		Destination:            destination,
		Source:                 referenceframe.PoseInFrameToProtobuf(query),
		SupplementalTransforms: transforms,
	})
	if err != nil {
		return nil, err
	}
	return referenceframe.ProtobufToPoseInFrame(resp.Pose), nil
}

// Status takes a list of resource names and returns their corresponding statuses. If no names are passed in, return all statuses.
func (rc *RobotClient) Status(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
	names := make([]*commonpb.ResourceName, 0, len(resourceNames))
	for _, name := range resourceNames {
		names = append(names, rprotoutils.ResourceNameToProto(name))
	}

	resp, err := rc.client.GetStatus(ctx, &pb.GetStatusRequest{ResourceNames: names})
	if err != nil {
		return nil, err
	}

	statuses := make([]robot.Status, 0, len(resp.Status))
	for _, status := range resp.Status {
		statuses = append(
			statuses, robot.Status{
				Name:   rprotoutils.ResourceNameFromProto(status.Name),
				Status: status.Status.AsMap(),
			})
	}
	return statuses, nil
}

// StopAll cancels all current and outstanding operations for the robot and stops all actuators and movement.
func (rc *RobotClient) StopAll(ctx context.Context, extra map[resource.Name]map[string]interface{}) error {
	e := []*pb.StopExtraParameters{}
	for name, params := range extra {
		param, err := protoutils.StructToStructPb(params)
		if err != nil {
			rc.Logger().Warnf("failed to convert extra params for resource %s with error: %s", name.Name, err)
			continue
		}
		p := &pb.StopExtraParameters{
			Name:   rprotoutils.ResourceNameToProto(name),
			Params: param,
		}
		e = append(e, p)
	}
	_, err := rc.client.StopAll(ctx, &pb.StopAllRequest{Extra: e})
	return err
}
