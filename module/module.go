package module

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fullstorydev/grpcurl"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pion/rtp"
	"github.com/pkg/errors"
	"github.com/viamrobotics/webrtc/v3"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	pb "go.viam.com/api/module/v1"
	robotpb "go.viam.com/api/robot/v1"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/utils"
	vprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"golang.org/x/exp/maps"
	"google.golang.org/grpc"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/config"
	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/services/discovery"
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

// errMaxSupportedWebRTCTrackLimit is the error returned when the MaxSupportedWebRTCTRacks limit is reached.
var errMaxSupportedWebRTCTrackLimit = fmt.Errorf("only %d WebRTC tracks are supported per peer connection", maxSupportedWebRTCTRacks)

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
		return "", errors.Errorf("module socket base path would result in a path greater than the OS limit of %d characters: %s",
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
		return "", errors.Errorf("failed to calculate a hash for %q while creating a truncated socket address", desiredName)
	}
	desiredNameHash := base32.StdEncoding.EncodeToString(desiredNameHashCreator.Sum(nil))
	if len(desiredNameHash) < socketHashSuffixLength {
		// sha256.Sum() should return 32 bytes so this shouldn't occur, but good to check instead of panicing
		return "", errors.Errorf("the encoded hash %q for %q is shorter than the minimum socket suffix length %v",
			desiredNameHash, desiredName, socketHashSuffixLength)
	}
	// Assemble the truncated socket address
	socketHashSuffix := desiredNameHash[:socketHashSuffixLength]
	truncatedName := desiredName[:(numRemainingChars - socketHashSuffixLength - 1)]
	return filepath.Join(baseAddr, fmt.Sprintf("%s-%s%s", truncatedName, socketHashSuffix, socketSuffix)), nil
}

// HandlerMap is the format for api->model pairs that the module will service.
// Ex: mymap["rdk:component:motor"] = ["acme:marine:thruster", "acme:marine:outboard"].
type HandlerMap map[resource.RPCAPI][]resource.Model

// ToProto converts the HandlerMap to a protobuf representation.
func (h HandlerMap) ToProto() *pb.HandlerMap {
	pMap := &pb.HandlerMap{}
	for s, models := range h {
		subtype := &robotpb.ResourceRPCSubtype{
			Subtype: protoutils.ResourceNameToProto(resource.Name{
				API:  s.API,
				Name: "",
			}),
			ProtoService: s.ProtoSvcName,
		}

		handler := &pb.HandlerDefinition{Subtype: subtype}
		for _, m := range models {
			handler.Models = append(handler.Models, m.String())
		}
		pMap.Handlers = append(pMap.Handlers, handler)
	}
	return pMap
}

// NewHandlerMapFromProto converts protobuf to HandlerMap.
func NewHandlerMapFromProto(ctx context.Context, pMap *pb.HandlerMap, conn rpc.ClientConn) (HandlerMap, error) {
	hMap := make(HandlerMap)
	refClient := grpcreflect.NewClientV1Alpha(ctx, reflectpb.NewServerReflectionClient(conn))
	defer refClient.Reset()
	reflSource := grpcurl.DescriptorSourceFromServer(ctx, refClient)

	var errs error
	for _, h := range pMap.GetHandlers() {
		api := protoutils.ResourceNameFromProto(h.Subtype.Subtype).API
		// due to how tagger is setup in the proto we cannot use reflection on the discovery service currently
		// for now we will add any registered models without a description,
		// and rely on the builtin registered discovery service instead.
		if api == discovery.API {
			rpcAPI := &resource.RPCAPI{
				API: api,
			}
			for _, m := range h.Models {
				model, err := resource.NewModelFromString(m)
				if err != nil {
					return nil, err
				}
				hMap[*rpcAPI] = append(hMap[*rpcAPI], model)
			}
			continue
		}
		symDesc, err := reflSource.FindSymbol(h.Subtype.ProtoService)
		if err != nil {
			errs = multierr.Combine(errs, err)
			if errors.Is(err, grpcurl.ErrReflectionNotSupported) {
				return nil, errs
			}
			continue
		}
		svcDesc, ok := symDesc.(*desc.ServiceDescriptor)
		if !ok {
			return nil, errors.Errorf("expected descriptor to be service descriptor but got %T", symDesc)
		}
		rpcAPI := &resource.RPCAPI{
			API:  api,
			Desc: svcDesc,
		}
		for _, m := range h.Models {
			model, err := resource.NewModelFromString(m)
			if err != nil {
				return nil, err
			}
			hMap[*rpcAPI] = append(hMap[*rpcAPI], model)
		}
	}
	return hMap, errs
}

type peerResourceState struct {
	// NOTE As I'm only suppporting video to start this will always be a single element
	// once we add audio we will need to make this a slice / map
	subID rtppassthrough.SubscriptionID
}

// Module represents an external resource module that services components/services.
type Module struct {
	shutdownCtx             context.Context
	shutdownFn              context.CancelFunc
	parent                  *client.RobotClient
	server                  rpc.Server
	logger                  logging.Logger
	mu                      sync.Mutex
	activeResourceStreams   map[resource.Name]peerResourceState
	streamSourceByName      map[resource.Name]rtppassthrough.Source
	operations              *operation.Manager
	ready                   bool
	addr                    string
	parentAddr              string
	activeBackgroundWorkers sync.WaitGroup
	handlers                HandlerMap
	collections             map[resource.API]resource.APIResourceCollection[resource.Resource]
	resLoggers              map[resource.Resource]logging.Logger
	closeOnce               sync.Once
	pc                      *webrtc.PeerConnection
	pcReady                 <-chan struct{}
	pcClosed                <-chan struct{}
	pcFailed                <-chan struct{}
	pb.UnimplementedModuleServiceServer
	streampb.UnimplementedStreamServiceServer
	robotpb.UnimplementedRobotServiceServer
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
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaries...)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streams...)),
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	m := &Module{
		shutdownCtx:           cancelCtx,
		shutdownFn:            cancel,
		logger:                logger,
		addr:                  address,
		operations:            opMgr,
		streamSourceByName:    map[resource.Name]rtppassthrough.Source{},
		activeResourceStreams: map[resource.Name]peerResourceState{},
		server:                NewServer(opts...),
		ready:                 true,
		handlers:              HandlerMap{},
		collections:           map[resource.API]resource.APIResourceCollection[resource.Resource]{},
		resLoggers:            map[resource.Resource]logging.Logger{},
	}
	if err := m.server.RegisterServiceServer(ctx, &pb.ModuleService_ServiceDesc, m); err != nil {
		return nil, err
	}
	if err := m.server.RegisterServiceServer(ctx, &streampb.StreamService_ServiceDesc, m); err != nil {
		return nil, err
	}
	// We register the RobotService API to supplement the ModuleService in order to serve select robot level methods from the module server
	// such as the DiscoverComponents API
	if err := m.server.RegisterServiceServer(ctx, &robotpb.RobotService_ServiceDesc, m); err != nil {
		return nil, err
	}

	// attempt to construct a PeerConnection
	pc, err := rgrpc.NewLocalPeerConnection(logger)
	if err != nil {
		logger.Debugw("Unable to create optional peer connection for module. Skipping WebRTC for module...", "err", err)
		return m, nil
	}

	// attempt to configure PeerConnection
	pcReady, pcClosed, err := rpc.ConfigureForRenegotiation(pc, rpc.PeerRoleServer, logger)
	if err != nil {
		msg := "Error creating renegotiation channel for module. Unable to " +
			"create optional peer connection for module. Skipping WebRTC for module..."
		logger.Debugw(msg, "err", err)
		return m, nil
	}

	m.pc = pc
	m.pcReady = pcReady
	m.pcClosed = pcClosed

	return m, nil
}

// NewModuleFromArgs directly parses the command line argument to get its address.
func NewModuleFromArgs(ctx context.Context) (*Module, error) {
	if len(os.Args) < 2 {
		return nil, errors.New("need socket path as command line argument")
	}
	return NewModule(ctx, os.Args[1], NewLoggerFromArgs(""))
}

// Start starts the module service and grpc server.
func (m *Module) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lis net.Listener
	prot := "unix"
	if rutils.TCPRegex.MatchString(m.addr) {
		prot = "tcp"
	}
	if err := MakeSelfOwnedFilesFunc(func() error {
		var err error
		lis, err = net.Listen(prot, m.addr)
		if err != nil {
			return errors.WithMessage(err, "failed to listen")
		}
		return nil
	}); err != nil {
		return err
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

// GetParentResource returns a resource from the parent robot by name.
func (m *Module) GetParentResource(ctx context.Context, name resource.Name) (resource.Resource, error) {
	// Refresh parent to ensure it has the most up-to-date resources before calling
	// ResourceByName.
	if err := m.parent.Refresh(ctx); err != nil {
		return nil, err
	}
	return m.parent.ResourceByName(name)
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
		if err := CheckSocketOwner(m.parentAddr); err != nil {
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
	rc, err := client.New(ctx, fullAddr, clientLogger, client.WithDisableSessions())
	if err != nil {
		return err
	}

	m.parent = rc
	return nil
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
		m.logger.Debugw("Unable to create optional peer connection for module. Skipping WebRTC for module...", "err", err)
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
	}

	resp.Ready = m.ready
	resp.Handlermap = m.handlers.ToProto()
	return resp, nil
}

// AddResource receives the component/service configuration from the parent.
func (m *Module) AddResource(ctx context.Context, req *pb.AddResourceRequest) (*pb.AddResourceResponse, error) {
	select {
	case <-m.pcReady:
	case <-m.pcFailed:
	}

	deps := make(resource.Dependencies)
	for _, c := range req.Dependencies {
		name, err := resource.NewFromString(c)
		if err != nil {
			return nil, err
		}
		c, err := m.GetParentResource(ctx, name)
		if err != nil {
			return nil, err
		}
		deps[name] = c
	}

	conf, err := config.ComponentConfigFromProto(req.Config)
	if err != nil {
		return nil, err
	}

	if err := addConvertedAttributes(conf); err != nil {
		return nil, errors.Wrapf(err, "unable to convert attributes when adding resource")
	}

	resInfo, ok := resource.LookupRegistration(conf.API, conf.Model)
	if !ok {
		return nil, errors.Errorf("do not know how to construct %q", conf.API)
	}
	if resInfo.Constructor == nil {
		return nil, errors.Errorf("invariant: no constructor for %q", conf.API)
	}
	resLogger := m.logger.Sublogger(conf.ResourceName().String())
	levelStr := req.Config.GetLogConfiguration().GetLevel()
	// An unset LogConfiguration will materialize as an empty string.
	if levelStr != "" {
		if level, err := logging.LevelFromString(levelStr); err == nil {
			resLogger.SetLevel(level)
		} else {
			m.logger.Warnw("LogConfiguration does not contain a valid level.", "resource", conf.ResourceName().Name, "level", levelStr)
		}
	}

	res, err := resInfo.Constructor(ctx, deps, *conf, resLogger)
	if err != nil {
		return nil, err
	}

	// If context has errored, even if construction succeeded we should close the resource and return the context error.
	// Use shutdownCtx because otherwise any Close operations that rely on the context will immediately fail.
	// The deadline associated with the context passed in to this function is rutils.GetResourceConfigurationTimeout,
	// which is propagated to AddResource through gRPC.
	if ctx.Err() != nil {
		m.logger.CDebugw(ctx, "resource successfully constructed but context is done, closing constructed resource", "err", ctx.Err().Error())
		return nil, multierr.Combine(ctx.Err(), res.Close(m.shutdownCtx))
	}

	var passthroughSource rtppassthrough.Source
	if p, ok := res.(rtppassthrough.Source); ok {
		passthroughSource = p
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	coll, ok := m.collections[conf.API]
	if !ok {
		return nil, errors.Errorf("module cannot service api: %s", conf.API)
	}

	// If adding the resource name to the collection fails, close the resource
	// and return an error
	if err := coll.Add(conf.ResourceName(), res); err != nil {
		return nil, multierr.Combine(err, res.Close(ctx))
	}

	m.resLoggers[res] = resLogger

	// add the video stream resources upon creation
	if passthroughSource != nil {
		m.streamSourceByName[res.Name()] = passthroughSource
	}
	return &pb.AddResourceResponse{}, nil
}

// DiscoverComponents takes a list of discovery queries and returns corresponding
// component configurations.
func (m *Module) DiscoverComponents(
	ctx context.Context,
	req *robotpb.DiscoverComponentsRequest,
) (*robotpb.DiscoverComponentsResponse, error) {
	var discoveries []*robotpb.Discovery

	for _, q := range req.Queries {
		// Handle triplet edge case i.e. if the subtype doesn't contain ':', add the "rdk:component:" prefix
		if !strings.ContainsRune(q.Subtype, ':') {
			q.Subtype = "rdk:component:" + q.Subtype
		}

		api, err := resource.NewAPIFromString(q.Subtype)
		if err != nil {
			return nil, fmt.Errorf("invalid subtype: %s: %w", q.Subtype, err)
		}
		model, err := resource.NewModelFromString(q.Model)
		if err != nil {
			return nil, fmt.Errorf("invalid model: %s: %w", q.Model, err)
		}

		resInfo, ok := resource.LookupRegistration(api, model)
		if !ok {
			return nil, fmt.Errorf("no registration found for API %s and model %s", api, model)
		}

		if resInfo.Discover == nil {
			return nil, fmt.Errorf("discovery not supported for API %s and model %s", api, model)
		}

		results, err := resInfo.Discover(ctx, m.logger, q.Extra.AsMap())
		if err != nil {
			return nil, fmt.Errorf("error discovering components for API %s and model %s: %w", api, model, err)
		}
		if results == nil {
			return nil, fmt.Errorf("error discovering components for API %s and model %s: results was nil", api, model)
		}

		pbResults, err := vprotoutils.StructToStructPb(results)
		if err != nil {
			return nil, fmt.Errorf("unable to convert discovery results to pb struct for query %v: %w", q, err)
		}

		pbDiscovery := &robotpb.Discovery{
			Query:   q,
			Results: pbResults,
		}
		discoveries = append(discoveries, pbDiscovery)
	}

	return &robotpb.DiscoverComponentsResponse{
		Discovery: discoveries,
	}, nil
}

// ReconfigureResource receives the component/service configuration from the parent.
func (m *Module) ReconfigureResource(ctx context.Context, req *pb.ReconfigureResourceRequest) (*pb.ReconfigureResourceResponse, error) {
	var res resource.Resource
	deps := make(resource.Dependencies)
	for _, c := range req.Dependencies {
		name, err := resource.NewFromString(c)
		if err != nil {
			return nil, err
		}
		c, err := m.GetParentResource(ctx, name)
		if err != nil {
			return nil, err
		}
		deps[name] = c
	}

	// it is assumed the caller robot has handled model differences
	conf, err := config.ComponentConfigFromProto(req.Config)
	if err != nil {
		return nil, err
	}

	if err := addConvertedAttributes(conf); err != nil {
		return nil, errors.Wrapf(err, "unable to convert attributes when reconfiguring resource")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	coll, ok := m.collections[conf.API]
	if !ok {
		return nil, errors.Errorf("no rpc service for %+v", conf)
	}
	res, err = coll.Resource(conf.ResourceName().Name)
	if err != nil {
		return nil, err
	}

	if logger, ok := m.resLoggers[res]; ok {
		levelStr := req.GetConfig().GetLogConfiguration().GetLevel()
		// An unset LogConfiguration will materialize as an empty string.
		if levelStr != "" {
			if level, err := logging.LevelFromString(levelStr); err == nil {
				logger.SetLevel(level)
			} else {
				m.logger.Warnw("LogConfiguration does not contain a valid level.", "resource", res.Name().Name, "level", levelStr)
			}
		}
	}

	reconfErr := res.Reconfigure(ctx, deps, *conf)
	if reconfErr == nil {
		return &pb.ReconfigureResourceResponse{}, nil
	}

	if !resource.IsMustRebuildError(reconfErr) {
		return nil, err
	}

	m.logger.Debugw("rebuilding", "name", conf.ResourceName())
	if err := res.Close(ctx); err != nil {
		m.logger.Error(err)
	}

	delete(m.activeResourceStreams, res.Name())
	resInfo, ok := resource.LookupRegistration(conf.API, conf.Model)
	if !ok {
		return nil, errors.Errorf("do not know how to construct %q", conf.API)
	}
	if resInfo.Constructor == nil {
		return nil, errors.Errorf("invariant: no constructor for %q", conf.API)
	}

	newRes, err := resInfo.Constructor(ctx, deps, *conf, m.logger)
	if err != nil {
		return nil, err
	}
	var passthroughSource rtppassthrough.Source
	if p, ok := newRes.(rtppassthrough.Source); ok {
		passthroughSource = p
	}

	if passthroughSource != nil {
		m.streamSourceByName[res.Name()] = passthroughSource
	}
	return &pb.ReconfigureResourceResponse{}, coll.ReplaceOne(conf.ResourceName(), newRes)
}

// ValidateConfig receives the validation request for a resource from the parent.
func (m *Module) ValidateConfig(ctx context.Context,
	req *pb.ValidateConfigRequest,
) (*pb.ValidateConfigResponse, error) {
	c, err := config.ComponentConfigFromProto(req.Config)
	if err != nil {
		return nil, err
	}

	if err := addConvertedAttributes(c); err != nil {
		return nil, errors.Wrapf(err, "unable to convert attributes for validation")
	}

	if c.ConvertedAttributes != nil {
		implicitDeps, err := c.ConvertedAttributes.Validate(c.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "error validating resource")
		}
		return &pb.ValidateConfigResponse{Dependencies: implicitDeps}, nil
	}

	// Resource configuration object does not implement Validate, but return an
	// empty response and no error to maintain backward compatibility.
	return &pb.ValidateConfigResponse{}, nil
}

// RemoveResource receives the request for resource removal.
func (m *Module) RemoveResource(ctx context.Context, req *pb.RemoveResourceRequest) (*pb.RemoveResourceResponse, error) {
	slowWatcher, slowWatcherCancel := utils.SlowGoroutineWatcher(
		30*time.Second, fmt.Sprintf("module resource %q is taking a while to remove", req.Name), m.logger)
	defer func() {
		slowWatcherCancel()
		<-slowWatcher
	}()
	m.mu.Lock()
	defer m.mu.Unlock()

	name, err := resource.NewFromString(req.Name)
	if err != nil {
		return nil, err
	}

	coll, ok := m.collections[name.API]
	if !ok {
		return nil, errors.Errorf("no grpc service for %+v", name)
	}
	res, err := coll.Resource(name.Name)
	if err != nil {
		return nil, err
	}

	if err := res.Close(ctx); err != nil {
		m.logger.Error(err)
	}

	delete(m.streamSourceByName, res.Name())
	delete(m.activeResourceStreams, res.Name())

	return &pb.RemoveResourceResponse{}, coll.Remove(name)
}

// addAPIFromRegistry adds a preregistered API (rpc API) to the module's services.
func (m *Module) addAPIFromRegistry(ctx context.Context, api resource.API) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.collections[api]
	if ok {
		return nil
	}

	apiInfo, ok := resource.LookupGenericAPIRegistration(api)
	if !ok {
		return errors.Errorf("invariant: registration does not exist for %q", api)
	}

	newColl := apiInfo.MakeEmptyCollection()
	m.collections[api] = newColl

	if !ok {
		return nil
	}
	return apiInfo.RegisterRPCService(ctx, m.server, newColl)
}

// AddModelFromRegistry adds a preregistered component or service model to the module's services.
func (m *Module) AddModelFromRegistry(ctx context.Context, api resource.API, model resource.Model) error {
	err := validateRegistered(api, model)
	if err != nil {
		return err
	}

	m.mu.Lock()
	_, ok := m.collections[api]
	m.mu.Unlock()
	if !ok {
		if err := m.addAPIFromRegistry(ctx, api); err != nil {
			return err
		}
	}

	apiInfo, ok := resource.LookupGenericAPIRegistration(api)
	if !ok {
		return errors.Errorf("invariant: registration does not exist for %q", api)
	}
	if apiInfo.ReflectRPCServiceDesc == nil {
		m.logger.Errorf("rpc subtype %s doesn't contain a valid ReflectRPCServiceDesc", api)
	}
	rpcAPI := resource.RPCAPI{
		API:          api,
		ProtoSvcName: apiInfo.RPCServiceDesc.ServiceName,
		Desc:         apiInfo.ReflectRPCServiceDesc,
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[rpcAPI] = append(m.handlers[rpcAPI], model)
	return nil
}

// OperationManager returns the operation manager for the module.
func (m *Module) OperationManager() *operation.Manager {
	return m.operations
}

// ListStreams lists the streams.
func (m *Module) ListStreams(ctx context.Context, req *streampb.ListStreamsRequest) (*streampb.ListStreamsResponse, error) {
	_, span := trace.StartSpan(ctx, "module::module::ListStreams")
	defer span.End()
	names := make([]string, 0, len(m.streamSourceByName))
	for _, n := range maps.Keys(m.streamSourceByName) {
		names = append(names, n.String())
	}
	return &streampb.ListStreamsResponse{Names: names}, nil
}

// AddStream adds a stream.
// Returns an error if:
// 1. there is no WebRTC peer connection with viam-sever
// 2. resource doesn't exist
// 3. the resource doesn't implement rtppassthrough.Source,
// 4. there are already the max number of supported tracks on the peer connection
// 5. SubscribeRTP returns an error
// 6. A webrtc track is unable to be created
// 7. Adding the track to the peer connection fails.
func (m *Module) AddStream(ctx context.Context, req *streampb.AddStreamRequest) (*streampb.AddStreamResponse, error) {
	ctx, span := trace.StartSpan(ctx, "module::module::AddStream")
	defer span.End()
	name, err := resource.NewFromString(req.GetName())
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pc == nil {
		return nil, errors.New("module has no peer connection")
	}
	vcss, ok := m.streamSourceByName[name]
	if !ok {
		err := errors.New("unknown stream for resource")
		m.logger.CWarnw(ctx, err.Error(), "name", name, "streamSourceByName", fmt.Sprintf("%#v", m.streamSourceByName))
		return nil, err
	}

	if _, ok = m.activeResourceStreams[name]; ok {
		m.logger.CWarnw(ctx, "AddStream called with when there is already a stream for peer connection. NoOp", "name", name)
		return &streampb.AddStreamResponse{}, nil
	}

	if len(m.activeResourceStreams) >= maxSupportedWebRTCTRacks {
		return nil, errMaxSupportedWebRTCTrackLimit
	}

	tlsRTP, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "video/H264"}, "video", name.String())
	if err != nil {
		return nil, errors.Wrap(err, "error creating a new TrackLocalStaticRTP")
	}

	sub, err := vcss.SubscribeRTP(ctx, rtpBufferSize, func(pkts []*rtp.Packet) {
		for _, pkt := range pkts {
			if err := tlsRTP.WriteRTP(pkt); err != nil {
				m.logger.CWarnw(ctx, "SubscribeRTP callback function WriteRTP", "err", err)
			}
		}
	})
	if err != nil {
		return nil, errors.Wrap(err, "error setting up stream subscription")
	}

	m.logger.CDebugw(ctx, "AddStream calling AddTrack", "name", name, "subID", sub.ID.String())
	sender, err := m.pc.AddTrack(tlsRTP)
	if err != nil {
		err = errors.Wrap(err, "error adding track")
		if unsubErr := vcss.Unsubscribe(ctx, sub.ID); unsubErr != nil {
			return nil, multierr.Combine(err, unsubErr)
		}
		return nil, err
	}

	removeTrackOnSubTerminate := func() {
		defer m.logger.Debugw("RemoveTrack called on ", "name", name, "subID", sub.ID.String())
		// wait until either the module is shutting down, or the subscription terminates
		var msg string
		select {
		case <-sub.Terminated.Done():
			msg = "rtp_passthrough subscription expired, calling RemoveTrack"
		case <-m.shutdownCtx.Done():
			msg = "module closing calling RemoveTrack"
		}
		// remove the track from the peer connection so that viam-server clients know that the stream has terminated
		m.mu.Lock()
		defer m.mu.Unlock()
		m.logger.Debugw(msg, "name", name, "subID", sub.ID.String())
		delete(m.activeResourceStreams, name)
		if err := m.pc.RemoveTrack(sender); err != nil {
			m.logger.Warnf("RemoveTrack returned error", "name", name, "subID", sub.ID.String(), "err", err)
		}
	}
	m.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(removeTrackOnSubTerminate, m.activeBackgroundWorkers.Done)

	m.activeResourceStreams[name] = peerResourceState{subID: sub.ID}
	return &streampb.AddStreamResponse{}, nil
}

// RemoveStream removes a stream.
func (m *Module) RemoveStream(ctx context.Context, req *streampb.RemoveStreamRequest) (*streampb.RemoveStreamResponse, error) {
	ctx, span := trace.StartSpan(ctx, "module::module::RemoveStream")
	defer span.End()
	name, err := resource.NewFromString(req.GetName())
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pc == nil {
		return nil, errors.New("module has no peer connection")
	}
	vcss, ok := m.streamSourceByName[name]
	if !ok {
		return nil, errors.Errorf("unknown stream for resource %s", name)
	}

	prs, ok := m.activeResourceStreams[name]
	if !ok {
		return nil, errors.Errorf("stream %s is not active", name)
	}

	if err := vcss.Unsubscribe(ctx, prs.subID); err != nil {
		m.logger.CWarnw(ctx, "RemoveStream > Unsubscribe", "name", name, "subID", prs.subID.String(), "err", err)
		return nil, err
	}

	delete(m.activeResourceStreams, name)
	return &streampb.RemoveStreamResponse{}, nil
}

// addConvertedAttributesToConfig uses the MapAttributeConverter to fill in the
// ConvertedAttributes field from the Attributes and AssociatedResourceConfigs.
func addConvertedAttributes(cfg *resource.Config) error {
	// Try to find map converter for a resource.
	reg, ok := resource.LookupRegistration(cfg.API, cfg.Model)
	if ok && reg.AttributeMapConverter != nil {
		converted, err := reg.AttributeMapConverter(cfg.Attributes)
		if err != nil {
			return errors.Wrapf(err, "error converting attributes for resource")
		}
		cfg.ConvertedAttributes = converted
	}

	// Also try for associated configs (will only succeed if module itself registers the associated config API).
	for subIdx, associatedConf := range cfg.AssociatedResourceConfigs {
		conv, ok := resource.LookupAssociatedConfigRegistration(associatedConf.API)
		if !ok {
			continue
		}
		if conv.AttributeMapConverter != nil {
			converted, err := conv.AttributeMapConverter(associatedConf.Attributes)
			if err != nil {
				return errors.Wrap(err, "error converting associated resource config attributes")
			}
			// associated resource configs for resources might be missing a resource name
			// which can be inferred from its resource config.
			converted.UpdateResourceNames(func(oldName resource.Name) resource.Name {
				return cfg.ResourceName()
			})
			cfg.AssociatedResourceConfigs[subIdx].ConvertedAttributes = converted
		}
	}
	return nil
}

// validateRegistered returns an error if the passed-in api and model have not
// yet been registered.
func validateRegistered(api resource.API, model resource.Model) error {
	resInfo, ok := resource.LookupRegistration(api, model)
	if ok && resInfo.Constructor != nil {
		return nil
	}

	return errors.Errorf("resource with API %s and model %s not yet registered", api, model)
}
