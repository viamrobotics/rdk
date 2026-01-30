package module

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/viamrobotics/webrtc/v3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	otelresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	pb "go.viam.com/api/module/v1"
	robotpb "go.viam.com/api/robot/v1"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/trace"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/camera/rtppassthrough"
	// Register component APIs.
	_ "go.viam.com/rdk/components/register_apis"
	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	// Register service APIs.
	_ "go.viam.com/rdk/services/register_apis"
	rutils "go.viam.com/rdk/utils"
)

const (
	socketSuffix = ".sock"
	// socketHashSuffixLength determines how many characters from the module's name's hash should be used when truncating the module socket.
	socketHashSuffixLength int = 5
	// maxSocketAddressLength is the length (-1 for null terminator) of the .sun_path field as used in kernel bind()/connect() syscalls.
	// Linux allows for a max length of 107 but to simplify this code, we truncate to the macOS limit of 103.
	socketMaxAddressLength int = 103
	rtpBufferSize          int = 512
	// https://viam.atlassian.net/browse/RSDK-7347
	// https://viam.atlassian.net/browse/RSDK-7521
	// maxSupportedWebRTCTRacks is the max number of WebRTC tracks that can be supported given wihout hitting the sctp SDP message size limit.
	maxSupportedWebRTCTRacks = 9

	// NoModuleParentEnvVar indicates whether there is a parent for a module being started.
	NoModuleParentEnvVar = "VIAM_NO_MODULE_PARENT"
)

// CreateSocketAddress returns a socket address of the form parentDir/desiredName.sock
// if it is shorter than the socketMaxAddressLength. If this path would be too long, this function
// truncates desiredName and returns parentDir/truncatedName-hashOfDesiredName.sock.
//
// Importantly, this function will return the same socket address as long as the desiredName doesn't change.
func CreateSocketAddress(parentDir, desiredName string) (string, error) {
	baseAddr := filepath.ToSlash(parentDir)
	numRemainingChars := socketMaxAddressLength -
		len(baseAddr) -
		len(socketSuffix) -
		1 // `/` between baseAddr and name
	if numRemainingChars < len(desiredName) && numRemainingChars < socketHashSuffixLength+1 {
		return "", fmt.Errorf("module socket base path would result in a path greater than the OS limit of %d characters: %s",
			socketMaxAddressLength, baseAddr)
	}
	// If possible, early-exit with a non-truncated socket path
	if numRemainingChars >= len(desiredName) {
		return filepath.Join(baseAddr, desiredName+socketSuffix), nil
	}
	// Hash the desiredName so that every invocation returns the same truncated address
	desiredNameHashCreator := sha256.New()
	_, err := desiredNameHashCreator.Write([]byte(desiredName))
	if err != nil {
		return "", fmt.Errorf("failed to calculate a hash for %q while creating a truncated socket address", desiredName)
	}
	desiredNameHash := base32.StdEncoding.EncodeToString(desiredNameHashCreator.Sum(nil))
	if len(desiredNameHash) < socketHashSuffixLength {
		// sha256.Sum() should return 32 bytes so this shouldn't occur, but good to check instead of panicing
		return "", fmt.Errorf("the encoded hash %q for %q is shorter than the minimum socket suffix length %v",
			desiredNameHash, desiredName, socketHashSuffixLength)
	}
	// Assemble the truncated socket address
	socketHashSuffix := desiredNameHash[:socketHashSuffixLength]
	truncatedName := desiredName[:(numRemainingChars - socketHashSuffixLength - 1)]
	return filepath.Join(baseAddr, fmt.Sprintf("%s-%s%s", truncatedName, socketHashSuffix, socketSuffix)), nil
}

// Module represents an external resource module that services components/services.
type Module struct {
	// The name of the module as per the robot config. This value is communicated via the
	// `VIAM_MODULE_NAME` env var.
	name string
	// mu protects high level operations. Specifically, reconfiguring resources, removing resources and shutdown.
	mu sync.Mutex

	// registerMu protects the maps immediately below as resources/streams come in and out of existence
	registerMu  sync.Mutex
	collections map[resource.API]resource.APIResourceCollection[resource.Resource]
	// internalDeps is keyed by a "child" resource and its values are "internal" resources that
	// depend on the child. We use a pointer for the value such that it's stable across map growth.
	// Similarly, the slice of `resConfigureArgs` can grow, hence we must use pointers such that
	// modifiying in place remains valid.
	internalDeps          map[resource.Resource][]resConfigureArgs
	resLoggers            map[resource.Resource]logging.Logger
	activeResourceStreams map[resource.Name]peerResourceState
	streamSourceByName    map[resource.Name]rtppassthrough.Source

	ready       bool
	shutdownCtx context.Context
	shutdownFn  context.CancelFunc

	activeBackgroundWorkers sync.WaitGroup
	closeOnce               sync.Once

	// operations is expected to manage concurrency internally.
	operations *operation.Manager
	server     rpc.Server
	handlers   HandlerMap
	pb.UnimplementedModuleServiceServer
	streampb.UnimplementedStreamServiceServer
	robotpb.UnimplementedRobotServiceServer

	addr                 string
	parent               *client.RobotClient
	parentAddr           string
	parentConnChangeFunc func(rc *client.RobotClient)
	pc                   *webrtc.PeerConnection
	pcReady              <-chan struct{}
	pcClosed             <-chan struct{}
	pcFailed             <-chan struct{}

	// for testing only
	parentClientOptions []client.RobotClientOption

	logger logging.Logger
}

// NewModuleFromArgs directly parses the command line argument to get its address.
func NewModuleFromArgs(ctx context.Context) (*Module, error) {
	if len(os.Args) < 2 {
		return nil, errors.New("need socket path as command line argument")
	}
	return NewModule(ctx, os.Args[1], NewLoggerFromArgs(""))
}

// NewModule returns the basic module framework/structure. Use ModularMain and NewModuleFromArgs unless
// you really know what you're doing.
func NewModule(ctx context.Context, address string, logger logging.Logger) (*Module, error) {
	// TODO(PRODUCT-343): session support likely means interceptors here
	opMgr := operation.NewManager(logger)
	unaries := []grpc.UnaryServerInterceptor{
		rgrpc.EnsureTimeoutUnaryServerInterceptor,
		opMgr.UnaryServerInterceptor,
	}
	streams := []grpc.StreamServerInterceptor{
		opMgr.StreamServerInterceptor,
	}

	cancelCtx, cancel := context.WithCancel(context.Background())

	// If the env variable does not exist, the empty string is returned.
	modName, _ := os.LookupEnv("VIAM_MODULE_NAME")
	tracingEnabledStr, _ := os.LookupEnv(rutils.ViamModuleTracingEnvVar)
	tracingEnabled := !slices.Contains([]string{"", "0", "false"}, tracingEnabledStr)

	m := &Module{
		name:                  modName,
		shutdownCtx:           cancelCtx,
		shutdownFn:            cancel,
		logger:                logger,
		addr:                  address,
		operations:            opMgr,
		streamSourceByName:    map[resource.Name]rtppassthrough.Source{},
		activeResourceStreams: map[resource.Name]peerResourceState{},
		ready:                 true,
		handlers:              HandlerMap{},
		collections:           map[resource.API]resource.APIResourceCollection[resource.Resource]{},
		resLoggers:            map[resource.Resource]logging.Logger{},
		internalDeps:          map[resource.Resource][]resConfigureArgs{},
	}

	if tracingEnabled {
		otlpClient := &moduleOtelExporter{mod: m}
		otelExporter, err := otlptrace.New(ctx, otlpClient)
		if err != nil {
			return nil, err
		}
		//nolint: errcheck
		trace.SetProvider(
			ctx,
			sdktrace.WithResource(
				otelresource.NewWithAttributes(
					semconv.SchemaURL,
					attribute.String("viam.module.name", modName),
					semconv.ServiceName(modName),
					semconv.ServiceNamespace("viam.com"),
					semconv.ServerAddress(address),
				),
			),
		)
		trace.AddExporters(otelExporter)
	}

	// MaxRecvMsgSize and MaxSendMsgSize by default are 4 MB & MaxInt32 (2.1 GB)
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(rpc.MaxMessageSize),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaries...)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streams...)),
	}
	m.server = NewServer(opts...)

	if err := m.server.RegisterServiceServer(ctx, &pb.ModuleService_ServiceDesc, m); err != nil {
		return nil, err
	}
	if err := m.server.RegisterServiceServer(ctx, &streampb.StreamService_ServiceDesc, m); err != nil {
		return nil, err
	}
	// We register the RobotService API to supplement the ModuleService in order to serve select robot level methods from the module server
	if err := m.server.RegisterServiceServer(ctx, &robotpb.RobotService_ServiceDesc, m); err != nil {
		return nil, err
	}

	// attempt to construct a PeerConnection
	pc, err := rgrpc.NewLocalPeerConnection(logger)
	if err != nil {
		logger.Debugw("Unable to create optional peer connection for module. Skipping WebRTC for module.", "err", err)
		return m, nil
	}

	// attempt to configure PeerConnection
	pcReady, pcClosed, err := rpc.ConfigureForRenegotiation(pc, rpc.PeerRoleServer, logger)
	if err != nil {
		logger.Debugw("Error creating renegotiation channel for module. Unable to create optional peer connection "+
			"for module. Skipping WebRTC for module.", "err", err)
		return m, nil
	}

	m.pc = pc
	m.pcReady = pcReady
	m.pcClosed = pcClosed

	return m, nil
}

// OperationManager returns the operation manager for the module.
func (m *Module) OperationManager() *operation.Manager {
	return m.operations
}

// Start starts the module service and grpc server.
func (m *Module) Start(ctx context.Context) error {
	prot := "unix"
	if rutils.TCPRegex.MatchString(m.addr) {
		prot = "tcp"
	}

	lis, err := net.Listen(prot, m.addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	if prot == "unix" {
		// If we are listening via a Unix socket, update the restrictions on the created sock
		// file to allow viam-server to access it. viam-server sometimes runs as a different
		// user than a module (e.g. if the module uses `sudo` to switch users in its
		// entrypoint script).
		//nolint:gosec
		err = os.Chmod(m.addr, 0o776)
		if err != nil {
			return fmt.Errorf("failed to update socket file permissions: %w", err)
		}
	}

	m.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer m.activeBackgroundWorkers.Done()
		// Attempt to remove module's .sock file.
		defer rutils.RemoveFileNoError(m.addr)
		m.logger.Infof("server listening at %v", lis.Addr())
		if err := m.server.Serve(lis); err != nil {
			m.logger.Errorf("failed to serve: %v", err)
		}
	})
	return nil
}

// Close shuts down the module and grpc server.
func (m *Module) Close(ctx context.Context) {
	// Dan: This feels unnecessary. The PR comment regarding windows and process shut down does not
	// give any insight in how module management code that calls `Close` a single time somehow
	// happens twice. I expect if this was real, it's because `Close` was called directly in process
	// signal handling that no longer exists.
	m.closeOnce.Do(func() {
		m.shutdownFn()
		m.mu.Lock()
		parent := m.parent
		if m.pc != nil {
			if err := m.pc.GracefulClose(); err != nil {
				m.logger.CErrorw(ctx, "WebRTC Peer Connection Close", "err", err)
			}
		}
		m.mu.Unlock()
		m.logger.Info("Shutting down gracefully.")
		if parent != nil {
			if err := parent.Close(ctx); err != nil {
				m.logger.Error(err)
			}
		}
		if err := m.server.Stop(); err != nil {
			m.logger.Error(err)
		}
		m.activeBackgroundWorkers.Wait()
	})
}

func (m *Module) connectParent(ctx context.Context) error {
	// If parent connection has already been made, do not make another one. Some
	// tests send two ReadyRequests sequentially, and if an rdk were to retry
	// sending a ReadyRequest to a module for any reason, we could feasibly make
	// a second connection back to the parent and leak the first, so disallow the
	// setting of parent more than once.
	if m.parent != nil {
		return nil
	}

	fullAddr := m.parentAddr
	if !rutils.TCPRegex.MatchString(m.parentAddr) {
		// If connecting over UDS, verify that the parent address actually exists before
		// attempting to connect.
		if _, err := os.Stat(m.parentAddr); err != nil {
			return err
		}
		fullAddr = "unix://" + m.parentAddr
	}

	// moduleLoggers may be creating the client connection below, so use a
	// different logger here to avoid a deadlock where the client connection
	// tries to recursively connect to the parent.
	clientLogger := logging.NewLogger("networking.module-connection")
	clientLogger.SetLevel(m.logger.GetLevel())
	// TODO(PRODUCT-343): add session support to modules

	connectOptions := []client.RobotClientOption{
		client.WithDisableSessions(),
		// These options are already automatically applied to unix domain socket connections (see [rpc.dial]),
		// adding these for TCP mode as well. This is because:
		// - The parent viam-server's module parent server does not spin up a signaling service
		// - Connections to the parent are always insecure
		client.WithDialOptions(rpc.WithForceDirectGRPC(), rpc.WithInsecure()),
	}
	if m.parentClientOptions != nil {
		connectOptions = append(connectOptions, m.parentClientOptions...)
	}

	// Modules compiled against newer SDKs may be running against older `viam-server`s that do not
	// provide the module name as an env variable.
	if m.name != "" {
		connectOptions = append(connectOptions, client.WithModName(m.name))
	}

	rc, err := client.New(ctx, fullAddr, m.logger, connectOptions...)
	if err != nil {
		return err
	}

	m.parent = rc
	if m.pc != nil {
		m.parent.SetPeerConnection(m.pc)
	}
	if m.parentConnChangeFunc != nil {
		rc.SetParentNotifier(func() { m.parentConnChangeFunc(rc) })
	}
	return nil
}

// RegisterParentConnectionChangeHandler is used to register a function to run whenever the connection
// back to the viam-server has changed.
func (m *Module) RegisterParentConnectionChangeHandler(f func(rc *client.RobotClient)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parentConnChangeFunc = f
}

// SetReady can be set to false if the module is not ready (ex. waiting on hardware).
func (m *Module) SetReady(ready bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready = ready
}

// PeerConnect returns the encoded answer string for the `ReadyResponse`.
func (m *Module) PeerConnect(encodedOffer string) (string, error) {
	if m.pc == nil {
		return "", errors.New("no PeerConnection object")
	}

	if encodedOffer == "" {
		//nolint
		return "", errors.New("Server not running with WebRTC enabled.")
	}

	offer := webrtc.SessionDescription{}
	if err := rpc.DecodeSDP(encodedOffer, &offer); err != nil {
		return "", err
	}
	if err := m.pc.SetRemoteDescription(offer); err != nil {
		return "", err
	}

	answer, err := m.pc.CreateAnswer(nil)
	if err != nil {
		return "", err
	}

	if err := m.pc.SetLocalDescription(answer); err != nil {
		return "", err
	}

	<-webrtc.GatheringCompletePromise(m.pc)
	return rpc.EncodeSDP(m.pc.LocalDescription())
}

// Ready receives the parent address and reports api/model combos the module is ready to service.
func (m *Module) Ready(ctx context.Context, req *pb.ReadyRequest) (*pb.ReadyResponse, error) {
	resp := &pb.ReadyResponse{}

	encodedAnswer, err := m.PeerConnect(req.WebrtcOffer)
	if err == nil {
		resp.WebrtcAnswer = encodedAnswer
	} else {
		m.logger.Debugw("Unable to create optional peer connection for module. Skipping WebRTC for module.", "err", err)
		pcFailed := make(chan struct{})
		close(pcFailed)
		m.pcFailed = pcFailed
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// we start the module without connecting to a parent since we
	// are only concerned with validation and extracting metadata.
	if os.Getenv(NoModuleParentEnvVar) != "true" {
		m.parentAddr = req.GetParentAddress()
		if err := m.connectParent(ctx); err != nil {
			// Return error back to parent if we cannot make a connection from module
			// -> parent. Something is wrong in that case and the module should not be
			// operational.
			return nil, err
		}
		// If logger is a moduleLogger, start gRPC logging.
		// Note that this logging assumes that a valid parent exists.
		if moduleLogger, ok := m.logger.(*moduleLogger); ok {
			moduleLogger.startLoggingViaGRPC(m)
		}
		m.logger.Debug("successfully created connection to parent")
	}

	resp.Ready = m.ready
	resp.Handlermap = m.handlers.ToProto()
	return resp, nil
}
