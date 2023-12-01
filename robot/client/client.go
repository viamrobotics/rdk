// Package client contains a gRPC based robot.Robot client.
package client

import (
	"context"
	"flag"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/pointcloud"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/packages"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils/contextutils"
)

var (
	// ErrMissingClientRegistration is used when there is no resource client registered for the API.
	ErrMissingClientRegistration = errors.New("resource client registration doesn't exist")

	// errUnimplemented is used for any unimplemented methods that should
	// eventually be implemented server side or faked client side.
	errUnimplemented = errors.New("unimplemented")

	// defaultResourcesTimeout is the default timeout for getting resources.
	defaultResourcesTimeout = 5 * time.Second
)

type reconfigurableClientConn struct {
	connMu sync.RWMutex
	conn   rpc.ClientConn
}

func (c *reconfigurableClientConn) Invoke(
	ctx context.Context,
	method string,
	args, reply interface{},
	opts ...googlegrpc.CallOption,
) error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn == nil {
		return errors.New("not connected")
	}
	return conn.Invoke(ctx, method, args, reply, opts...)
}

func (c *reconfigurableClientConn) NewStream(
	ctx context.Context,
	desc *googlegrpc.StreamDesc,
	method string,
	opts ...googlegrpc.CallOption,
) (googlegrpc.ClientStream, error) {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn == nil {
		return nil, errors.New("not connected")
	}
	return conn.NewStream(ctx, desc, method, opts...)
}

func (c *reconfigurableClientConn) replaceConn(conn rpc.ClientConn) {
	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()
}

func (c *reconfigurableClientConn) Close() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn == nil {
		return nil
	}
	conn := c.conn
	c.conn = nil
	return conn.Close()
}

// RobotClient satisfies the robot.Robot interface through a gRPC based
// client conforming to the robot.proto contract.
type RobotClient struct {
	resource.Named
	remoteName  string
	address     string
	dialOptions []rpc.DialOption

	mu              sync.RWMutex
	resourceNames   []resource.Name
	resourceRPCAPIs []resource.RPCAPI
	resourceClients map[resource.Name]resource.Resource
	remoteNameMap   map[resource.Name]resource.Name
	changeChan      chan bool
	notifyParent    func()
	conn            reconfigurableClientConn
	client          pb.RobotServiceClient
	refClient       *grpcreflect.Client
	connected       atomic.Bool

	activeBackgroundWorkers sync.WaitGroup
	backgroundCtx           context.Context
	backgroundCtxCancel     func()
	logger                  logging.Logger

	// sessions
	sessionsDisabled bool

	sessionMu                sync.RWMutex
	sessionsSupported        *bool // when nil, we have not yet checked
	currentSessionID         string
	sessionHeartbeatInterval time.Duration

	heartbeatWorkers   sync.WaitGroup
	heartbeatCtx       context.Context
	heartbeatCtxCancel func()
}

// RemoteTypeName is the type name used for a remote. This is for internal use.
const RemoteTypeName = string("remote")

// RemoteAPI is the fully qualified API for a remote. This is for internal use.
var RemoteAPI = resource.APINamespaceRDK.WithType(RemoteTypeName).WithSubtype("")

// Reconfigure always returns an unsupported error.
func (rc *RobotClient) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	return errors.New("unsupported")
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
		rc.Logger().CDebugw(ctx, "connection is down, skipping method call", "method", method)
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
		rc.Logger().CDebugw(ctx, "connection is down, skipping method call", "method", method)
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
func New(ctx context.Context, address string, clientLogger logging.ZapCompatibleLogger, opts ...RobotClientOption) (*RobotClient, error) {
	logger := logging.FromZapCompatible(clientLogger)
	var rOpts robotClientOpts

	for _, opt := range opts {
		opt.apply(&rOpts)
	}
	backgroundCtx, backgroundCtxCancel := context.WithCancel(context.Background())
	heartbeatCtx, heartbeatCtxCancel := context.WithCancel(context.Background())

	rc := &RobotClient{
		Named:               resource.NewName(RemoteAPI, rOpts.remoteName).AsNamed(),
		remoteName:          rOpts.remoteName,
		address:             address,
		backgroundCtx:       backgroundCtx,
		backgroundCtxCancel: backgroundCtxCancel,
		logger:              logger,
		dialOptions:         rOpts.dialOptions,
		notifyParent:        nil,
		resourceClients:     make(map[resource.Name]resource.Resource),
		remoteNameMap:       make(map[resource.Name]resource.Name),
		sessionsDisabled:    rOpts.disableSessions,
		heartbeatCtx:        heartbeatCtx,
		heartbeatCtxCancel:  heartbeatCtxCancel,
	}

	// interceptors are applied in order from first to last
	rc.dialOptions = append(
		rc.dialOptions,
		rpc.WithUnaryClientInterceptor(contextutils.ContextWithMetadataUnaryClientInterceptor),
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

	if checkConnectedTime > 0 && reconnectTime > 0 {
		refresh := checkConnectedTime == refreshTime
		rc.activeBackgroundWorkers.Add(1)
		utils.ManagedGo(func() {
			rc.checkConnection(backgroundCtx, checkConnectedTime, reconnectTime, refresh)
		}, rc.activeBackgroundWorkers.Done)

		// If checkConnection() is running refresh, there is no need to create a separate
		// RefreshEvery thread, so end the function here.
		if refresh {
			return rc, nil
		}
	}

	if refreshTime > 0 {
		rc.activeBackgroundWorkers.Add(1)
		utils.ManagedGo(func() {
			rc.RefreshEvery(backgroundCtx, refreshTime)
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
	return rc.connected.Load()
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
	if err := rc.connectWithLock(ctx); err != nil {
		return err
	}

	if rc.notifyParent != nil {
		rc.notifyParent()
	}
	return nil
}

func (rc *RobotClient) connectWithLock(ctx context.Context) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if err := rc.conn.Close(); err != nil {
		return err
	}
	conn, err := grpc.Dial(ctx, rc.address, rc.logger, rc.dialOptions...)
	if err != nil {
		return err
	}

	client := pb.NewRobotServiceClient(conn)

	refClient := grpcreflect.NewClientV1Alpha(rc.backgroundCtx, reflectpb.NewServerReflectionClient(conn))

	rc.conn.replaceConn(conn)
	rc.client = client
	rc.refClient = refClient
	rc.connected.Store(true)
	if len(rc.resourceClients) != 0 {
		if err := rc.updateResources(ctx); err != nil {
			return err
		}
	}

	if rc.changeChan != nil {
		rc.changeChan <- true
	}
	return nil
}

func (rc *RobotClient) updateResourceClients(ctx context.Context) error {
	activeResources := make(map[resource.Name]bool)

	for _, name := range rc.resourceNames {
		activeResources[name] = true
	}

	for resourceName, client := range rc.resourceClients {
		// check if no longer an active resource
		if !activeResources[resourceName] {
			if err := client.Close(ctx); err != nil {
				rc.Logger().Error(err)
				continue
			}
			delete(rc.resourceClients, resourceName)
		}
	}

	return nil
}

// checkConnection either checks if the client is still connected, or attempts to reconnect to the remote.
func (rc *RobotClient) checkConnection(ctx context.Context, checkEvery, reconnectEvery time.Duration, refresh bool) {
	for {
		var waitTime time.Duration
		if rc.connected.Load() {
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
		if !rc.connected.Load() {
			rc.Logger().CInfow(ctx, "trying to reconnect to remote at address", "address", rc.address)
			if err := rc.connect(ctx); err != nil {
				rc.Logger().Errorw("failed to reconnect remote", "error", err, "address", rc.address)
				continue
			}
			rc.Logger().CInfow(ctx, "successfully reconnected remote at address", "address", rc.address)
		} else {
			check := func() error {
				if refresh {
					if err := rc.Refresh(ctx); err != nil {
						return err
					}
				} else {
					if _, _, err := rc.resources(ctx); err != nil {
						return err
					}
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
				rc.connected.Store(false)
				if rc.changeChan != nil {
					rc.changeChan <- true
				}

				var notifyParentFn func()
				if rc.notifyParent != nil {
					rc.Logger().CDebugf(ctx, "connection was lost for remote %q", rc.address)
					// RSDK-3670: This callback may ultimately acquire the `robotClient.mu`
					// mutex. Execute the function after releasing the mutex.
					notifyParentFn = rc.notifyParent
				}
				rc.mu.Unlock()
				if notifyParentFn != nil {
					notifyParentFn()
				}
			}
		}
	}
}

// Close cleanly closes the underlying connections and stops the refresh goroutine
// if it is running.
func (rc *RobotClient) Close(ctx context.Context) error {
	rc.backgroundCtxCancel()
	rc.activeBackgroundWorkers.Wait()
	if rc.changeChan != nil {
		close(rc.changeChan)
		rc.changeChan = nil
	}
	rc.refClient.Reset()
	rc.heartbeatCtxCancel()
	rc.heartbeatWorkers.Wait()
	return rc.conn.Close()
}

func (rc *RobotClient) checkConnected() error {
	if !rc.connected.Load() {
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
func (rc *RobotClient) ResourceByName(name resource.Name) (resource.Resource, error) {
	if err := rc.checkConnected(); err != nil {
		return nil, err
	}

	rc.mu.RLock()

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
	return nil, resource.NewNotFoundError(name)
}

func (rc *RobotClient) createClient(name resource.Name) (resource.Resource, error) {
	apiInfo, ok := resource.LookupGenericAPIRegistration(name.API)
	if !ok || apiInfo.RPCClient == nil {
		if name.API.Type.Namespace != resource.APINamespaceRDK {
			return grpc.NewForeignResource(name, &rc.conn), nil
		}
		return nil, ErrMissingClientRegistration
	}
	return apiInfo.RPCClient(rc.backgroundCtx, &rc.conn, rc.remoteName, name, rc.Logger())
}

func (rc *RobotClient) resources(ctx context.Context) ([]resource.Name, []resource.RPCAPI, error) {
	// RSDK-5356 If we are in a testing environment, never apply
	// defaultResourcesTimeout. Tests run in parallel, and if execution of a test
	// pauses for longer than 5s, below calls to ResourceNames or
	// ResourceRPCSubtypes can result in context errors that appear in client.New
	// and remote logic.
	//
	// TODO(APP-2917): Once we upgrade to go 1.21, replace this if check with if
	// !testing.Testing().
	if flag.Lookup("test.v") == nil {
		var cancel func()
		ctx, cancel = contextutils.ContextWithTimeoutIfNoDeadline(ctx, defaultResourcesTimeout)
		defer cancel()
	}
	resp, err := rc.client.ResourceNames(ctx, &pb.ResourceNamesRequest{})
	if err != nil {
		return nil, nil, err
	}

	var resTypes []resource.RPCAPI
	typesResp, err := rc.client.ResourceRPCSubtypes(ctx, &pb.ResourceRPCSubtypesRequest{})
	if err == nil {
		reflSource := grpcurl.DescriptorSourceFromServer(ctx, rc.refClient)

		resTypes = make([]resource.RPCAPI, 0, len(typesResp.ResourceRpcSubtypes))
		for _, resAPI := range typesResp.ResourceRpcSubtypes {
			symDesc, err := reflSource.FindSymbol(resAPI.ProtoService)
			if err != nil {
				// Note: This happens right now if a client is talking to a main server
				// that has a remote or similarly if a server is talking to a remote that
				// has a remote. This can be solved by either integrating reflection into
				// robot.proto or by overriding the gRPC reflection service to return
				// reflection results from its remotes.
				rc.Logger().CDebugw(ctx, "failed to find symbol for resource API", "api", resAPI, "error", err)
				continue
			}
			svcDesc, ok := symDesc.(*desc.ServiceDescriptor)
			if !ok {
				return nil, nil, errors.Errorf("expected descriptor to be service descriptor but got %T", symDesc)
			}
			resTypes = append(resTypes, resource.RPCAPI{
				API:  rprotoutils.ResourceNameFromProto(resAPI.Subtype).API,
				Desc: svcDesc,
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
	return rc.updateResources(ctx)
}

func (rc *RobotClient) updateResources(ctx context.Context) error {
	// call metadata service.
	names, rpcAPIs, err := rc.resources(ctx)
	// only return if it is not unimplemented - means a bigger error came up
	if err != nil && status.Code(err) != codes.Unimplemented {
		return err
	}
	if err == nil {
		rc.resourceNames = make([]resource.Name, 0, len(names))
		rc.resourceNames = append(rc.resourceNames, names...)
		rc.resourceRPCAPIs = rpcAPIs
	}

	rc.updateRemoteNameMap()

	return rc.updateResourceClients(ctx)
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

// PackageManager returns nil.
func (rc *RobotClient) PackageManager() packages.Manager {
	return nil
}

// ResourceNames returns all resource names.
func (rc *RobotClient) ResourceNames() []resource.Name {
	if err := rc.checkConnected(); err != nil {
		rc.Logger().Errorw("failed to get remote resource names", "error", err)
		return nil
	}
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	names := make([]resource.Name, 0, len(rc.resourceNames))
	names = append(names, rc.resourceNames...)
	return names
}

// ResourceRPCAPIs returns a list of all known resource APIs.
func (rc *RobotClient) ResourceRPCAPIs() []resource.RPCAPI {
	if err := rc.checkConnected(); err != nil {
		rc.Logger().Errorw("failed to get remote resource types", "error", err)
		return nil
	}
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	apis := make([]resource.RPCAPI, 0, len(rc.resourceRPCAPIs))
	for _, v := range rc.resourceRPCAPIs {
		vCopy := v
		apis = append(
			apis,
			vCopy,
		)
	}
	return apis
}

// Logger returns the logger being used for this robot.
func (rc *RobotClient) Logger() logging.Logger {
	return rc.logger
}

// DiscoverComponents takes a list of discovery queries and returns corresponding
// component configurations.
func (rc *RobotClient) DiscoverComponents(ctx context.Context, qs []resource.DiscoveryQuery) ([]resource.Discovery, error) {
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

	discoveries := make([]resource.Discovery, 0, len(resp.Discovery))
	for _, disc := range resp.Discovery {
		m, err := resource.NewModelFromString(disc.Query.Model)
		if err != nil {
			return nil, err
		}
		s, err := resource.NewAPIFromString(disc.Query.Subtype)
		if err != nil {
			return nil, err
		}
		q := resource.DiscoveryQuery{
			API:   s,
			Model: m,
		}
		discoveries = append(
			discoveries, resource.Discovery{
				Query:   q,
				Results: disc.Results.AsMap(),
			})
	}
	return discoveries, nil
}

// FrameSystemConfig returns the info of each individual part that makes up the frame system.
func (rc *RobotClient) FrameSystemConfig(ctx context.Context) (*framesystem.Config, error) {
	resp, err := rc.client.FrameSystemConfig(ctx, &pb.FrameSystemConfigRequest{})
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
	return &framesystem.Config{Parts: result}, nil
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

// TransformPointCloud will transform the pointcloud to the desired frame in the robot's frame system.
// Do not move the robot between the generation of the initial pointcloud and the receipt
// of the transformed pointcloud because that will make the transformations inaccurate.
// TODO(RSDK-1197): Rather than having to apply a transform to every point using ApplyOffset,
// implementing the suggested ticket would mean simply adding the transform to a field in the
// point cloud struct, and then returning the updated struct. Would be super fast.
func (rc *RobotClient) TransformPointCloud(ctx context.Context, srcpc pointcloud.PointCloud, srcName, dstName string,
) (pointcloud.PointCloud, error) {
	if dstName == "" {
		dstName = referenceframe.World
	}
	if srcName == "" {
		return nil, errors.New("srcName cannot be empty, must provide name of point cloud origin")
	}
	// get the offset pose from a TransformPose request
	sourceFrameZero := referenceframe.NewPoseInFrame(srcName, spatialmath.NewZeroPose())
	resp, err := rc.client.TransformPose(ctx, &pb.TransformPoseRequest{
		Destination:            dstName,
		Source:                 referenceframe.PoseInFrameToProtobuf(sourceFrameZero),
		SupplementalTransforms: []*commonpb.Transform{},
	})
	if err != nil {
		return nil, err
	}
	transformPose := referenceframe.ProtobufToPoseInFrame(resp.Pose).Pose()
	return pointcloud.ApplyOffset(ctx, srcpc, transformPose, rc.Logger())
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
				Name:             rprotoutils.ResourceNameFromProto(status.Name),
				LastReconfigured: status.LastReconfigured.AsTime(),
				Status:           status.Status.AsMap(),
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
			rc.Logger().CWarnf(ctx, "failed to convert extra params for resource %s with error: %s", name.Name, err)
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
