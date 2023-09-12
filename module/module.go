package module

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	pb "go.viam.com/api/module/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	rutils "go.viam.com/rdk/utils"
)

const (
	socketSuffix = ".sock"
	// If we assume each robot has 10 modules running on it, and they would all have colliding truncated names,
	// P(collision) = 1-(48^5)!/((48^5-10)!*(48^(5*10))) ~ 1.8E-7.
	socketRandomSuffixLen int = 5
	// maxSocketAddressLength is the length (-1 for null terminator) of the .sun_path field as used in kernel bind()/connect() syscalls.
	// Linux allows for a max length of 107 but to simplify this code, we truncate to the macOS limit of 103.
	socketMaxAddressLength int = 103
)

// TruncatedSocketAddress returns a socket address of the form parentDir/desiredName.sock
// if it is shorter than the max socket length on the given os. If this path would be too long, this function
// truncates desiredName and returns parentDir/truncatedName-randomStr.sock.
func TruncatedSocketAddress(parentDir, desiredName string) (string, error) {
	baseAddr := filepath.ToSlash(parentDir)
	numRemainingChars := socketMaxAddressLength -
		len(baseAddr) -
		len(socketSuffix) -
		1 // `/` between baseAddr and name
	if numRemainingChars < len(desiredName) && numRemainingChars < socketRandomSuffixLen+1 {
		return "", errors.Errorf("module socket base path would result in a path greater than the OS limit of %d characters: %s",
			socketMaxAddressLength, baseAddr)
	}
	if numRemainingChars >= len(desiredName) {
		return filepath.Join(baseAddr, desiredName+socketSuffix), nil
	}
	numRemainingChars -= socketRandomSuffixLen + 1 // save one character for the `-` between truncatedName and socketRandomSuffix
	socketRandomSuffix := utils.RandomAlphaString(socketRandomSuffixLen)
	truncatedName := desiredName[:numRemainingChars]
	return filepath.Join(baseAddr, fmt.Sprintf("%s-%s%s", truncatedName, socketRandomSuffix, socketSuffix)), nil
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
func NewHandlerMapFromProto(ctx context.Context, pMap *pb.HandlerMap, conn *grpc.ClientConn) (HandlerMap, error) {
	hMap := make(HandlerMap)
	refClient := grpcreflect.NewClientV1Alpha(ctx, reflectpb.NewServerReflectionClient(conn))
	defer refClient.Reset()
	reflSource := grpcurl.DescriptorSourceFromServer(ctx, refClient)

	var errs error
	for _, h := range pMap.GetHandlers() {
		api := protoutils.ResourceNameFromProto(h.Subtype.Subtype).API

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

// Module represents an external resource module that services components/services.
type Module struct {
	parent                  *client.RobotClient
	server                  rpc.Server
	logger                  *zap.SugaredLogger
	mu                      sync.Mutex
	operations              *operation.Manager
	ready                   bool
	addr                    string
	parentAddr              string
	activeBackgroundWorkers sync.WaitGroup
	handlers                HandlerMap
	collections             map[resource.API]resource.APIResourceCollection[resource.Resource]
	closeOnce               sync.Once
	pb.UnimplementedModuleServiceServer
}

// NewModule returns the basic module framework/structure.
func NewModule(ctx context.Context, address string, logger *zap.SugaredLogger) (*Module, error) {
	// TODO(PRODUCT-343): session support likely means interceptors here
	opMgr := operation.NewManager(logger)
	unaries := []grpc.UnaryServerInterceptor{
		opMgr.UnaryServerInterceptor,
	}
	streams := []grpc.StreamServerInterceptor{
		opMgr.StreamServerInterceptor,
	}
	m := &Module{
		logger:      logger,
		addr:        address,
		operations:  opMgr,
		server:      NewServer(unaries, streams),
		ready:       true,
		handlers:    HandlerMap{},
		collections: map[resource.API]resource.APIResourceCollection[resource.Resource]{},
	}
	if err := m.server.RegisterServiceServer(ctx, &pb.ModuleService_ServiceDesc, m); err != nil {
		return nil, err
	}
	return m, nil
}

// NewModuleFromArgs directly parses the command line argument to get its address.
func NewModuleFromArgs(ctx context.Context, logger *zap.SugaredLogger) (*Module, error) {
	if len(os.Args) < 2 {
		return nil, errors.New("need socket path as command line argument")
	}
	return NewModule(ctx, os.Args[1], logger)
}

// NewLoggerFromArgs can be used to create a golog.Logger at "DebugLevel" if
// "--log-level=debug" is the third argument in os.Args and at "InfoLevel"
// otherwise. See config.Module.LogLevel documentation for more info on how
// to start modules with a "log-level" commandline argument.
func NewLoggerFromArgs(moduleName string) golog.Logger {
	if len(os.Args) >= 3 && os.Args[2] == "--log-level=debug" {
		return golog.NewDebugLogger(moduleName)
	}
	return golog.NewDevelopmentLogger(moduleName)
}

// Start starts the module service and grpc server.
func (m *Module) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lis net.Listener
	if err := MakeSelfOwnedFilesFunc(func() error {
		var err error
		lis, err = net.Listen("unix", m.addr)
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
		m.mu.Lock()
		parent := m.parent
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
	if err := m.connectParent(ctx); err != nil {
		return nil, err
	}

	// Refresh parent to ensure it has the most up-to-date resources before calling
	// ResourceByName.
	if err := m.parent.Refresh(ctx); err != nil {
		return nil, err
	}
	return m.parent.ResourceByName(name)
}

func (m *Module) connectParent(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.parent == nil {
		if err := CheckSocketOwner(m.parentAddr); err != nil {
			return err
		}
		// TODO(PRODUCT-343): add session support to modules
		rc, err := client.New(ctx, "unix://"+m.parentAddr, m.logger, client.WithDisableSessions())
		if err != nil {
			return err
		}
		m.parent = rc
	}
	return nil
}

// SetReady can be set to false if the module is not ready (ex. waiting on hardware).
func (m *Module) SetReady(ready bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready = ready
}

// Ready receives the parent address and reports api/model combos the module is ready to service.
func (m *Module) Ready(ctx context.Context, req *pb.ReadyRequest) (*pb.ReadyResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parentAddr = req.GetParentAddress()

	return &pb.ReadyResponse{
		Ready:      m.ready,
		Handlermap: m.handlers.ToProto(),
	}, nil
}

// AddResource receives the component/service configuration from the parent.
func (m *Module) AddResource(ctx context.Context, req *pb.AddResourceRequest) (*pb.AddResourceResponse, error) {
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
	res, err := resInfo.Constructor(ctx, deps, *conf, m.logger)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	coll, ok := m.collections[conf.API]
	if !ok {
		return nil, errors.Errorf("module cannot service api: %s", conf.API)
	}

	return &pb.AddResourceResponse{}, coll.Add(conf.ResourceName(), res)
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

// addConvertedAttributesToConfig uses the MapAttributeConverter to fill in the
// ConvertedAttributes field from the Attributes.
func addConvertedAttributes(cfg *resource.Config) error {
	// Try to find map converter for a resource.
	reg, ok := resource.LookupRegistration(cfg.API, cfg.Model)
	if !ok || reg.AttributeMapConverter == nil {
		return nil
	}
	converted, err := reg.AttributeMapConverter(cfg.Attributes)
	if err != nil {
		return errors.Wrapf(err, "error converting attributes for resource")
	}
	cfg.ConvertedAttributes = converted
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
