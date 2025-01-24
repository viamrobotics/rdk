// Package client contains a gRPC based robot.Robot client.
package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fullstorydev/grpcurl"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/cloud"
	"go.viam.com/rdk/config"
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

	// DoNotWaitForRunning should be set only in tests to allow connecting to
	// still-initializing machines. Note that robot clients in production (not in
	// a testing environment) will already allow connecting to still-initializing
	// machines.
	DoNotWaitForRunning = atomic.Bool{}
)

// RobotClient satisfies the robot.Robot interface through a gRPC based
// client conforming to the robot.proto contract.
type RobotClient struct {
	resource.Named
	remoteName  string
	address     string
	dialOptions []rpc.DialOption

	mu                       sync.RWMutex
	resourceNames            []resource.Name
	resourceRPCAPIs          []resource.RPCAPI
	resourceClients          map[resource.Name]resource.Resource
	remoteNameMap            map[resource.Name]resource.Name
	changeChan               chan bool
	notifyParent             func()
	conn                     grpc.ReconfigurableClientConn
	client                   pb.RobotServiceClient
	refClient                *grpcreflect.Client
	connected                atomic.Bool
	rpcSubtypesUnimplemented bool

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

	// If we ever connect to a server using webrtc, we want all subsequent connections to force
	// webrtc. Some operations such as video streaming are much more performant when using
	// webrtc. We don't want a network disconnect to result in reconnecting over tcp such that
	// performance would be impacted.
	serverIsWebrtcEnabled bool
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

// TODO(RSDK-9333): Better account for possible gRPC interaction errors
// and transform them appropriately.
//
// NOTE(benjirewis): I believe gRPC interactions can fail in
// three broad ways, each with one to three error paths:
//
//  1. Creation of stream
//     a. `rpc.ErrDisconnected` returned due to underlying channel closure
//  2. Sending of headers/message representing request
//     a. Proto marshal failure
//     b. `io.ErrClosedPipe` due to write to a closed socket
//     c. Possible SCTP errors `ErrStreamClosed` and `ErrOutboundPacketTooLarge`
//  3. Receiving of response
//     a. Proto unmarshal failure
//     b. `io.EOF` due to reading from a closed socket
//     c. Context deadline exceeded due to timeout
//     d. Context canceled due to client cancelation
//
// Ideally, these paths would all be represented in a single error type that a
// Golang SDK user could treat as one error, and we could examine the message
// of the error to see _where_ exactly the failure was in the interaction.
func isDisconnectedError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, rpc.ErrDisconnected) ||
		strings.Contains(err.Error(), io.ErrClosedPipe.Error())
}

func (rc *RobotClient) notConnectedToRemoteError() error {
	return fmt.Errorf("not connected to remote robot at %s", rc.address)
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
	if isDisconnectedError(err) {
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
	if isDisconnectedError(err) {
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
	if isDisconnectedError(err) {
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
		rpc.WithUnaryClientInterceptor(logging.UnaryClientInterceptor),
		// sending version metadata
		rpc.WithUnaryClientInterceptor(unaryClientInterceptor()),
		rpc.WithStreamClientInterceptor(streamClientInterceptor()),
	)

	if err := rc.Connect(ctx); err != nil {
		return nil, err
	}

	// If running in a testing environment, wait for machine to report a state of
	// running. We often establish connections in tests and expect resources to
	// be immediately available once the web service has started; resources will
	// not be available when the machine is still initializing.
	//
	// It is expected that golang SDK users will handle lack of resource
	// availability due to the machine being in an initializing state themselves.
	//
	// Allow this behavior to be turned off in some tests that specifically want
	// to examine the behavior of a machine in an initializing state through the
	// use of a global variable.
	if testing.Testing() && !DoNotWaitForRunning.Load() {
		for {
			if ctx.Err() != nil {
				return nil, multierr.Combine(ctx.Err(), rc.conn.Close())
			}

			mStatus, err := rc.MachineStatus(ctx)
			if err != nil {
				// Allow for MachineStatus to not be injected/implemented in some tests.
				if status.Code(err) == codes.Unimplemented {
					break
				}
				// Ignore error from Close and just return original machine status error.
				utils.UncheckedError(rc.conn.Close())
				return nil, err
			}

			if mStatus.State == robot.StateRunning {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
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

// Connect will close any existing connection and try to reconnect to the remote.
func (rc *RobotClient) Connect(ctx context.Context) error {
	if err := rc.connectWithLock(ctx); err != nil {
		return err
	}
	rc.Logger().CInfow(ctx, "successfully (re)connected to remote at address", "address", rc.address)
	if rc.notifyParent != nil {
		rc.notifyParent()
		rc.Logger().CDebugw(ctx, "successfully notified parent after (re)connection", "address", rc.address)
	}
	return nil
}

func (rc *RobotClient) connectWithLock(ctx context.Context) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if err := rc.conn.Close(); err != nil {
		return err
	}

	// Try forcing a webrtc connection.
	dialOptionsWebRTCOnly := make([]rpc.DialOption, len(rc.dialOptions)+1)
	// Put our "disable GRPC" option in front and the user input values at the end. This ensures
	// user inputs take precedence.
	copy(dialOptionsWebRTCOnly[1:], rc.dialOptions)
	dialOptionsWebRTCOnly[0] = rpc.WithDisableDirectGRPC()

	dialLogger := rc.logger.Sublogger("networking")
	conn, err := grpc.Dial(ctx, rc.address, dialLogger, dialOptionsWebRTCOnly...)
	if err == nil {
		// If we succeed with a webrtc connection, flip the `serverIsWebrtcEnabled` to force all future
		// connections to use webrtc.
		if !rc.serverIsWebrtcEnabled {
			rc.logger.Info("A WebRTC connection was made to the robot. ",
				"Reconnects will disallow direct gRPC connections.")
			rc.serverIsWebrtcEnabled = true
		}
	} else if !rc.serverIsWebrtcEnabled {
		// If we failed to connect via webrtc and* we've never previously connected over webrtc, try
		// to connect with a grpc over a tcp connection.
		//
		// Put our "force GRPC" option in front and the user input values at the end. This ensures
		// user inputs take precedence.
		dialOptionsGRPCOnly := make([]rpc.DialOption, len(rc.dialOptions)+2)
		copy(dialOptionsGRPCOnly[2:], rc.dialOptions)
		dialOptionsGRPCOnly[0] = rpc.WithForceDirectGRPC()

		// Using `WithForceDirectGRPC` disables mdns lookups. This is not the same behavior as a
		// webrtc dial which will* fallback to a direct grpc connection with* the mdns address. So
		// we add this flag to partially override the above override.
		dialOptionsGRPCOnly[1] = rpc.WithDialMulticastDNSOptions(rpc.DialMulticastDNSOptions{Disable: false})

		grpcConn, grpcErr := grpc.Dial(ctx, rc.address, dialLogger, dialOptionsGRPCOnly...)
		if grpcErr == nil {
			conn = grpcConn
			err = nil
		} else {
			err = multierr.Combine(err, grpcErr)
		}
	}
	if err != nil {
		return err
	}

	client := pb.NewRobotServiceClient(conn)

	refClient := grpcreflect.NewClientV1Alpha(rc.backgroundCtx, reflectpb.NewServerReflectionClient(conn))

	rc.conn.ReplaceConn(conn)
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
			rc.logger.Infow("Removing resource from remote client", "resourceName", resourceName)
			if err := client.Close(ctx); err != nil {
				rc.Logger().CError(ctx, err)
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
			if err := rc.Connect(ctx); err != nil {
				rc.Logger().CErrorw(ctx, "failed to reconnect remote", "error", err, "address", rc.address)
				continue
			}
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
					if isDisconnectedError(err) {
						break
					}
					// otherwise retry
					continue
				}
				outerError = nil
				break
			}
			if outerError != nil {
				rc.Logger().CErrorw(ctx,
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

// Close closes the underlying client connections to the machine and stops any periodic tasks running in the client.
//
//	err := machine.Close(ctx.Background())
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

// RefreshEvery refreshes the machine on the interval given by every until the
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
			rc.Logger().CErrorw(ctx, "failed to refresh resources from remote", "error", err)
		}
	}
}

// RemoteByName returns a remote machine by name. It is assumed to exist on the
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
		return grpc.NewForeignResource(name, &rc.conn), nil
	}
	logger := rc.Logger().Sublogger(resource.RemoveRemoteName(name).ShortName())
	return apiInfo.RPCClient(rc.backgroundCtx, &rc.conn, rc.remoteName, name, logger)
}

func (rc *RobotClient) resources(ctx context.Context) ([]resource.Name, []resource.RPCAPI, error) {
	// RSDK-5356 If we are in a testing environment, never apply
	// defaultResourcesTimeout. Tests run in parallel, and if execution of a test
	// pauses for longer than 5s, below calls to ResourceNames or
	// ResourceRPCSubtypes can result in context errors that appear in client.New
	// and remote logic.
	if !testing.Testing() {
		var cancel func()
		ctx, cancel = contextutils.ContextWithTimeoutIfNoDeadline(ctx, defaultResourcesTimeout)
		defer cancel()
	}

	resp, err := rc.client.ResourceNames(ctx, &pb.ResourceNamesRequest{})
	if err != nil {
		return nil, nil, err
	}

	var resTypes []resource.RPCAPI

	resources := make([]resource.Name, 0, len(resp.Resources))
	for _, name := range resp.Resources {
		newName := rprotoutils.ResourceNameFromProto(name)
		resources = append(resources, newName)
	}

	// resource has previously returned an unimplemented response, skip rpc call
	if rc.rpcSubtypesUnimplemented {
		return resources, resTypes, nil
	}

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
				return nil, nil, fmt.Errorf("expected descriptor to be service descriptor but got %T", symDesc)
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
		// prevent future calls to ResourceRPCSubtypes
		rc.rpcSubtypesUnimplemented = true
	}

	return resources, resTypes, nil
}

// Refresh manually updates the underlying parts of this machine.
//
//	err := machine.Refresh(ctx)
func (rc *RobotClient) Refresh(ctx context.Context) (err error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.updateResources(ctx)
}

func (rc *RobotClient) updateResources(ctx context.Context) error {
	// call metadata service.

	names, rpcAPIs, err := rc.resources(ctx)
	if err != nil && status.Code(err) != codes.Unimplemented {
		return fmt.Errorf("error updating resources: %w", err)
	}

	rc.resourceNames = make([]resource.Name, 0, len(names))
	rc.resourceNames = append(rc.resourceNames, names...)
	rc.resourceRPCAPIs = rpcAPIs

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

// ResourceNames returns a list of all known resource names connected to this machine.
//
//	resource_names := machine.ResourceNames()
func (rc *RobotClient) ResourceNames() []resource.Name {
	if err := rc.checkConnected(); err != nil {
		rc.Logger().Errorw("failed to get remote resource names", "error", err.Error())
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

// DiscoverComponents is DEPRECATED!!! Please use the Discovery Service instead.
// DiscoverComponents takes a list of discovery queries and returns corresponding
// component configurations.
//
//	// Define a new discovery query.
//	q := resource.NewDiscoveryQuery(acme.API, resource.Model{Name: "some model"})
//
//	// Define a list of discovery queries.
//	qs := []resource.DiscoverQuery{q}
//
//	// Get component configurations with these queries.
//	component_configs, err := machine.DiscoverComponents(ctx.Background(), qs)
//
//nolint:deprecated,staticcheck
func (rc *RobotClient) DiscoverComponents(ctx context.Context, qs []resource.DiscoveryQuery) ([]resource.Discovery, error) {
	rc.logger.Warn(
		"DiscoverComponents is deprecated and will be removed on March 10th 2025. Please use the Discovery Service instead.")
	pbQueries := make([]*pb.DiscoveryQuery, 0, len(qs))
	for _, q := range qs {
		extra, err := structpb.NewStruct(q.Extra)
		if err != nil {
			return nil, err
		}
		pbQueries = append(
			pbQueries,
			&pb.DiscoveryQuery{
				Subtype: q.API.String(),
				Model:   q.Model.String(),
				Extra:   extra,
			},
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
			Extra: disc.Query.Extra.AsMap(),
		}
		discoveries = append(
			discoveries, resource.Discovery{
				Query:   q,
				Results: disc.Results.AsMap(),
			})
	}
	return discoveries, nil
}

// FrameSystemConfig  returns the configuration of the frame system of a given machine.
//
//	frameSystem, err := machine.FrameSystemConfig(context.Background(), nil)
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
//
//	  import (
//		  "go.viam.com/rdk/referenceframe"
//		  "go.viam.com/rdk/spatialmath"
//	  )
//
//	  baseOrigin := referenceframe.NewPoseInFrame("test-base", spatialmath.NewZeroPose())
//	  movementSensorToBase, err := robot.TransformPose(ctx, baseOrigin, "my-movement-sensor", nil)
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

// StopAll cancels all current and outstanding operations for the machine and stops all actuators and movement.
//
//	err := machine.StopAll(ctx.Background())
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

// Log sends a log entry to the server. To be used by Golang modules wanting to
// log over gRPC and not by normal Golang SDK clients.
func (rc *RobotClient) Log(ctx context.Context, log zapcore.Entry, fields []zap.Field) error {
	message := fmt.Sprintf("%v\t%v", log.Caller.TrimmedPath(), log.Message)

	fieldsP := make([]*structpb.Struct, 0, len(fields))
	for _, field := range fields {
		fieldP, err := logging.FieldToProto(field)
		if err != nil {
			return err
		}
		fieldsP = append(fieldsP, fieldP)
	}

	logRequest := &pb.LogRequest{
		// No batching for now (one LogEntry at a time).
		Logs: []*commonpb.LogEntry{{
			// Leave out Host; Host is not currently meaningful.
			Level:      log.Level.String(),
			Time:       timestamppb.New(log.Time),
			LoggerName: log.LoggerName,
			Message:    message,
			// Leave out Caller; Caller is already in Message field above. We put
			// the Caller in Message as other languages may also do this in the
			// future. We do not want other languages to have to force their caller
			// information into a struct that looks like zapcore.EntryCaller.
			Stack:  log.Stack,
			Fields: fieldsP,
		}},
	}

	_, err := rc.client.Log(ctx, logRequest)
	return err
}

// CloudMetadata returns app-related information about the machine.
//
//	metadata, err := machine.CloudMetadata(ctx.Background())
func (rc *RobotClient) CloudMetadata(ctx context.Context) (cloud.Metadata, error) {
	req := &pb.GetCloudMetadataRequest{}
	resp, err := rc.client.GetCloudMetadata(ctx, req)
	if err != nil {
		return cloud.Metadata{}, err
	}
	return rprotoutils.MetadataFromProto(resp), nil
}

// RestartModule restarts a running module by name or ID.
func (rc *RobotClient) RestartModule(ctx context.Context, req robot.RestartModuleRequest) error {
	reqPb := &pb.RestartModuleRequest{}
	if len(req.ModuleID) > 0 {
		reqPb.IdOrName = &pb.RestartModuleRequest_ModuleId{ModuleId: req.ModuleID}
	} else {
		reqPb.IdOrName = &pb.RestartModuleRequest_ModuleName{ModuleName: req.ModuleName}
	}
	_, err := rc.client.RestartModule(ctx, reqPb)
	if err != nil {
		return err
	}
	return nil
}

// Shutdown shuts down the robot. May return DeadlineExceeded error if shutdown request times out,
// or if robot server shuts down before having a chance to send a response. May return Unavailable error
// if server is unavailable, or if robot server is in the process of shutting down when response is ready.
func (rc *RobotClient) Shutdown(ctx context.Context) error {
	reqPb := &pb.ShutdownRequest{}
	_, err := rc.client.Shutdown(ctx, reqPb)
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() { //nolint:exhaustive
			case codes.Internal, codes.Unknown:
				break
			case codes.Unavailable:
				rc.Logger().CWarnw(ctx, "server unavailable, likely due to successful robot shutdown")
				return err
			case codes.DeadlineExceeded:
				rc.Logger().CWarnw(ctx, "request timeout, robot shutdown may still be successful")
				return err
			default:
				return err
			}
		} else {
			return err
		}
	}
	rc.Logger().CDebug(ctx, "robot shutdown successful")
	return nil
}

// MachineStatus returns the current status of the robot.
func (rc *RobotClient) MachineStatus(ctx context.Context) (robot.MachineStatus, error) {
	mStatus := robot.MachineStatus{}

	req := &pb.GetMachineStatusRequest{}
	resp, err := rc.client.GetMachineStatus(ctx, req)
	if err != nil {
		return mStatus, err
	}

	if resp.Config != nil {
		mStatus.Config = config.Revision{
			Revision:    resp.Config.Revision,
			LastUpdated: resp.Config.LastUpdated.AsTime(),
		}
	}

	mStatus.Resources = make([]resource.Status, 0, len(resp.Resources))
	for _, pbResStatus := range resp.Resources {
		resStatus := resource.Status{
			NodeStatus: resource.NodeStatus{
				Name:        rprotoutils.ResourceNameFromProto(pbResStatus.Name),
				LastUpdated: pbResStatus.LastUpdated.AsTime(),
				Revision:    pbResStatus.Revision,
			},
			CloudMetadata: rprotoutils.MetadataFromProto(pbResStatus.CloudMetadata),
		}

		switch pbResStatus.State {
		case pb.ResourceStatus_STATE_UNSPECIFIED:
			rc.logger.CErrorw(ctx, "received resource in an unspecified state", "resource", resStatus.Name.String())
			resStatus.State = resource.NodeStateUnknown
		case pb.ResourceStatus_STATE_UNCONFIGURED:
			resStatus.State = resource.NodeStateUnconfigured
		case pb.ResourceStatus_STATE_CONFIGURING:
			resStatus.State = resource.NodeStateConfiguring
		case pb.ResourceStatus_STATE_READY:
			resStatus.State = resource.NodeStateReady
		case pb.ResourceStatus_STATE_REMOVING:
			resStatus.State = resource.NodeStateRemoving
		case pb.ResourceStatus_STATE_UNHEALTHY:
			resStatus.State = resource.NodeStateUnhealthy
			if pbResStatus.Error != "" {
				resStatus.Error = errors.New(pbResStatus.Error)
			}
		}

		mStatus.Resources = append(mStatus.Resources, resStatus)
	}

	switch resp.State {
	case pb.GetMachineStatusResponse_STATE_UNSPECIFIED:
		rc.logger.CError(ctx, "received unspecified machine state")
		mStatus.State = robot.StateUnknown
	case pb.GetMachineStatusResponse_STATE_INITIALIZING:
		mStatus.State = robot.StateInitializing
	case pb.GetMachineStatusResponse_STATE_RUNNING:
		mStatus.State = robot.StateRunning
	}

	return mStatus, nil
}

// Version returns version information about the machine.
func (rc *RobotClient) Version(ctx context.Context) (robot.VersionResponse, error) {
	mVersion := robot.VersionResponse{}

	resp, err := rc.client.GetVersion(ctx, &pb.GetVersionRequest{})
	if err != nil {
		return mVersion, err
	}

	mVersion.Platform = resp.Platform
	mVersion.Version = resp.Version
	mVersion.APIVersion = resp.ApiVersion

	return mVersion, nil
}

func unaryClientInterceptor() googlegrpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *googlegrpc.ClientConn,
		invoker googlegrpc.UnaryInvoker,
		opts ...googlegrpc.CallOption,
	) error {
		md, err := robot.Version()
		if err != nil {
			ctx = metadata.AppendToOutgoingContext(ctx, "viam_client", "go;unknown;unknown")
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		stringMd := fmt.Sprintf("go;%s;%s", md.Version, md.APIVersion)
		ctx = metadata.AppendToOutgoingContext(ctx, "viam_client", stringMd)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func streamClientInterceptor() googlegrpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *googlegrpc.StreamDesc,
		cc *googlegrpc.ClientConn,
		method string,
		streamer googlegrpc.Streamer,
		opts ...googlegrpc.CallOption,
	) (cs googlegrpc.ClientStream, err error) {
		md, err := robot.Version()
		if err != nil {
			ctx = metadata.AppendToOutgoingContext(ctx, "viam_client", "go;unknown;unknown")
			return streamer(ctx, desc, cc, method, opts...)
		}
		stringMd := fmt.Sprintf("go;%s;%s", md.Version, md.APIVersion)
		ctx = metadata.AppendToOutgoingContext(ctx, "viam_client", stringMd)
		return streamer(ctx, desc, cc, method, opts...)
	}
}
