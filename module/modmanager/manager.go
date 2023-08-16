// Package modmanager provides the module manager for a robot.
package modmanager

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	pb "go.viam.com/api/module/v1"
	// "go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	rdkgrpc "go.viam.com/rdk/grpc"
	modlib "go.viam.com/rdk/module"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

var (
	validateConfigTimeout       = 5 * time.Second
	errMessageExitStatus143     = "exit status 143"
	logLevelArgumentTemplate    = "--log-level=%s"
	errModularResourcesDisabled = errors.New("modular resources disabled in untrusted environment")
)

type addedResource struct {
	conf resource.Config
	deps []string
}

// Implements resource.Resource and goes in the resource graph
// Manages a single module
type moduleManager struct {
	mu        sync.RWMutex
	name      string
	exe       string
	logLevel  string
	logger    golog.Logger
	process   pexec.ManagedProcess
	handles   modlib.HandlerMap
	conn      *grpc.ClientConn
	client    pb.ModuleServiceClient
	addr      string
	resources map[resource.Name]*addedResource

	// pendingRemoval allows delaying module close until after resources within it are closed
	pendingRemoval bool

	// inRecovery stores whether or not an OnUnexpectedExit function is trying
	// to recover a crash of this module; inRecoveryLock guards the execution of
	// an OnUnexpectedExit function for this module.
	//
	// NOTE(benjirewis): Using just an atomic boolean is not sufficient, as OUE
	// functions for the same module cannot overlap and should not continue after
	// another OUE has finished.
	inRecovery     atomic.Bool
	inRecoveryLock sync.Mutex
}

func RegisterModule(ctx context.Context, conf config.Module, parentAddr string, logger golog.Logger) error {
	if _, ok := resource.LookupGenericAPIRegistration(config.ModuleAPI); !ok {
		resource.RegisterAPI(
			config.ModuleAPI,
			resource.APIRegistration[resource.Resource]{},
		)
	}
	modMan, err := startModuleManager(ctx, conf, parentAddr, logger)
	if err != nil {
		return err
	}
	resource.Register(config.ModuleAPI, conf.AsResource().Model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			_ resource.Dependencies,
			_ resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			return modMan, nil
		},
	})
	return nil
}

func startModuleManager(ctx context.Context, conf config.Module, parentAddr string, logger golog.Logger) (*moduleManager, error) {
	mod := &moduleManager{
		name:      conf.Name,
		exe:       conf.ExePath,
		logLevel:  conf.LogLevel,
		logger:    logger,
		conn:      nil,
		resources: map[resource.Name]*addedResource{},
	}

	var success bool
	defer func() {
		if !success {
			if err := mod.stopProcess(); err != nil {
				//TODO mod.cleanupAfterStarupFailure
				mod.logger.Error(err)
			}
		}
	}()

	if err := mod.startProcess(ctx, parentAddr,
		mod.newOnUnexpectedExitHandler()); err != nil {
		return nil, errors.WithMessage(err, "error while starting module "+mod.name)
	}

	// dial will re-use mod.conn if it's non-nil (module being added in a Reconfigure).
	if err := mod.dial(); err != nil {
		return nil, errors.WithMessage(err, "error while dialing module "+mod.name)
	}

	if err := mod.checkReady(ctx, parentAddr); err != nil {
		return nil, errors.WithMessage(err, "error while waiting for module to be ready "+mod.name)
	}

	mod.registerResources()

	success = true
	return mod, nil
}

func (m *moduleManager) Name() resource.Name {
	return resource.NewName(config.ModuleAPI, m.name)
}

func (m *moduleManager) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	// TODO(pre-merge) implement this
	panic("tried to reconfigure a module")
}

func (m *moduleManager) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, resource.ErrDoUnimplemented
}

func (m *moduleManager) Close(ctx context.Context) error {
	for res := range m.resources {
		_, err := m.client.RemoveResource(context.Background(), &pb.RemoveResourceRequest{Name: res.String()})
		if err != nil {
			m.logger.Errorw("error removing resource", "module", m.name, "resource", res.Name, "error", err)
		}
	}
	err := m.stopProcess()
	m.deregisterResources()
	return err
}

type ModularResourceConfigWrapper struct {
	m          *moduleManager
	api        resource.API
	model      resource.Model
	attributes rutils.AttributeMap
}

func (wrapper *ModularResourceConfigWrapper) Validate(path string) ([]string, error) {
	wrappedConfig := resource.Config{
		API:        wrapper.api,
		Model:      wrapper.model,
		Attributes: wrapper.attributes,
	}
	confProto, err := config.ComponentConfigToProto(&wrappedConfig)
	if err != nil {
		return []string{}, err
	}

	// Override context with new timeout.
	// TODO(pre-merge) create a context on the module or something
	ctx, cancel := context.WithTimeout(context.TODO(), validateConfigTimeout)
	defer cancel()

	resp, err := wrapper.m.client.ValidateConfig(ctx, &pb.ValidateConfigRequest{Config: confProto})
	// Swallow "Unimplemented" gRPC errors from modules that lack ValidateConfig
	// receiving logic.
	if err != nil {

		return []string{}, err
	}
	if err != nil && status.Code(err) != codes.Unimplemented {
		return []string{}, err
	}
	return resp.Dependencies, nil
}

func (m *moduleManager) registerResources() {
	for api, models := range m.handles {
		if _, ok := resource.LookupGenericAPIRegistration(api.API); !ok {
			resource.RegisterAPI(
				api.API,
				resource.APIRegistration[resource.Resource]{ReflectRPCServiceDesc: api.Desc},
			)
		}
		if !(api.API.IsComponent() || api.API.IsService()) {
			m.logger.Errorf("invalid module api: %s. It must be a component or a service", api.API.Type)
			continue
		}
		for _, model := range models {
			m.logger.Debugw("registering resource definition from module", "module", m.name, "API", api.API, "model", model)
			modelClone := model
			apiClone := api.API
			resource.Register(api.API, model, resource.Registration[resource.Resource, *ModularResourceConfigWrapper]{
				Constructor: func(
					ctx context.Context,
					deps resource.Dependencies,
					conf resource.Config,
					logger golog.Logger,
				) (resource.Resource, error) {
					return m.AddResource(ctx, conf, DepsToNames(deps))
				},
				AttributeMapConverter: func(attributes rutils.AttributeMap) (*ModularResourceConfigWrapper, error) {
					return &ModularResourceConfigWrapper{
						m:          m,
						api:        apiClone,
						model:      modelClone,
						attributes: attributes,
					}, nil
				},
			})
		}
	}
}

func (m *moduleManager) AddResource(ctx context.Context, conf resource.Config, deps []string) (resource.Resource, error) {
	confProto, err := config.ComponentConfigToProto(&conf)
	if err != nil {
		return nil, err
	}

	_, err = m.client.AddResource(ctx, &pb.AddResourceRequest{Config: confProto, Dependencies: deps})
	if err != nil {
		return nil, err
	}

	//TODO(pre-merge) make this a debug or something
	m.logger.Warnf("Module %q added resource %q", m.name, conf.Name)
	apiInfo, ok := resource.LookupGenericAPIRegistration(conf.API)
	if !ok || apiInfo.RPCClient == nil {
		m.logger.Warnf("no built-in grpc client for modular resource %s", conf.ResourceName())
		return rdkgrpc.NewForeignResource(conf.ResourceName(), m.conn), nil
	}
	return apiInfo.RPCClient(ctx, m.conn, "", conf.ResourceName(), m.logger)
}

func (m *moduleManager) startProcess(
	ctx context.Context,
	parentAddr string,
	oue func(int) bool,
) error {
	m.addr = filepath.ToSlash(filepath.Join(filepath.Dir(parentAddr), m.name+".sock"))
	if err := modlib.CheckSocketAddressLength(m.addr); err != nil {
		return err
	}

	pconf := pexec.ProcessConfig{
		ID:               m.name,
		Name:             m.exe,
		Args:             []string{m.addr},
		Log:              true,
		OnUnexpectedExit: oue,
	}
	// Start module process with supplied log level or "debug" if none is
	// supplied and module manager has a DebugLevel logger.
	if m.logLevel != "" {
		pconf.Args = append(pconf.Args, fmt.Sprintf(logLevelArgumentTemplate, m.logLevel))
	} else if m.logger.Level().Enabled(zapcore.DebugLevel) {
		pconf.Args = append(pconf.Args, fmt.Sprintf(logLevelArgumentTemplate, "debug"))
	}

	m.process = pexec.NewManagedProcess(pconf, m.logger)

	err := m.process.Start(context.Background())
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, rutils.GetResourceConfigurationTimeout(m.logger))
	defer cancel()
	for {
		select {
		case <-ctxTimeout.Done():
			return errors.Errorf("timed out waiting for module %s to start listening", m.name)
		default:
		}
		err = modlib.CheckSocketOwner(m.addr)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return errors.WithMessage(err, "module startup failed")
		}
		break
	}
	return nil
}

func (m *moduleManager) stopProcess() error {
	if m.process == nil {
		return nil
	}
	// Attempt to remove module's .sock file if module did not remove it
	// already.
	defer rutils.RemoveFileNoError(m.addr)

	// TODO(RSDK-2551): stop ignoring exit status 143 once Python modules handle
	// SIGTERM correctly.
	if err := m.process.Stop(); err != nil &&
		!strings.Contains(err.Error(), errMessageExitStatus143) {
		return err
	}
	return nil
}

// dial will use m.conn to make a new module service client or Dial m.addr if
// m.conn is nil.
func (m *moduleManager) dial() error {
	if m.conn == nil {
		// TODO(PRODUCT-343): session support probably means interceptors here
		var err error
		m.conn, err = grpc.Dial(
			"unix://"+m.addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithChainUnaryInterceptor(
				grpc_retry.UnaryClientInterceptor(),
				operation.UnaryClientInterceptor,
			),
			grpc.WithChainStreamInterceptor(
				grpc_retry.StreamClientInterceptor(),
				operation.StreamClientInterceptor,
			),
		)
		if err != nil {
			return errors.WithMessage(err, "module startup failed")
		}
	}
	m.client = pb.NewModuleServiceClient(m.conn)
	return nil
}

func (m *moduleManager) checkReady(ctx context.Context, parentAddr string) error {
	ctxTimeout, cancelFunc := context.WithTimeout(ctx, rutils.GetResourceConfigurationTimeout(m.logger))
	defer cancelFunc()

	for {
		req := &pb.ReadyRequest{ParentAddress: parentAddr}
		// 5000 is an arbitrarily high number of attempts (context timeout should hit long before)
		resp, err := m.client.Ready(ctxTimeout, req, grpc_retry.WithMax(5000))
		if err != nil {
			return err
		}

		if resp.Ready {
			m.handles, err = modlib.NewHandlerMapFromProto(ctx, resp.Handlermap, m.conn)
			return err
		}
	}
}

func (m *moduleManager) deregisterResources() {
	for api, models := range m.handles {
		for _, model := range models {
			resource.Deregister(api.API, model)
		}
	}
	m.handles = nil
}

// func (m *module) cleanupAfterStartupFailure(mgr *Manager, afterCrash bool) {
// 	if err := m.stopProcess(); err != nil {
// 		msg := "error while stopping process of module that failed to start"
// 		if afterCrash {
// 			msg = "error while stopping process of crashed module"
// 		}
// 		mgr.logger.Errorw(msg, "module", m.name, "error", err)
// 	}
// 	if m.conn != nil {
// 		if err := m.conn.Close(); err != nil {
// 			msg := "error while closing connection to module that failed to start"
// 			if afterCrash {
// 				msg = "error while closing connection to crashed module"
// 			}
// 			mgr.logger.Errorw(msg, "module", m.name, "error", err)
// 		}
// 	}

// 	// Remove module from rMap and mgr.modules if startup failure was after crash.
// 	if afterCrash {
// 		for r, mod := range mgr.rMap {
// 			if mod == m {
// 				delete(mgr.rMap, r)
// 			}
// 		}
// 		delete(mgr.modules, m.name)
// 	}
// }

// DepsToNames converts a dependency list to a simple string slice.
func DepsToNames(deps resource.Dependencies) []string {
	var depStrings []string
	for dep := range deps {
		depStrings = append(depStrings, dep.String())
	}
	return depStrings
}

var (
	// oueTimeout is the length of time for which an OnUnexpectedExit function
	// can execute blocking calls.
	oueTimeout = 2 * time.Minute
	// oueRestartInterval is the interval of time at which an OnUnexpectedExit
	// function can attempt to restart the module process. Multiple restart
	// attempts will use basic backoff.
	oueRestartInterval = 5 * time.Second
)

// newOnUnexpectedExitHandler returns the appropriate OnUnexpectedExit function
// for the passed-in module to include in the pexec.ProcessConfig.
func (mod *moduleManager) newOnUnexpectedExitHandler() func(exitCode int) bool {
	return func(exitCode int) bool {
		// TODO bring over
		return false
	}
}

// TODO bring over
// func (mgr *Manager) attemptRestart(ctx context.Context, mod *module) []resource.Name {
