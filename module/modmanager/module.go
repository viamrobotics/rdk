package modmanager

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	pb "go.viam.com/api/module/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/ftdc"
	"go.viam.com/rdk/ftdc/sys"
	rdkgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	modlib "go.viam.com/rdk/module"
	"go.viam.com/rdk/module/modmaninterface"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/packages"
	rutils "go.viam.com/rdk/utils"
)

type module struct {
	cfg        config.Module
	dataDir    string
	process    pexec.ManagedProcess
	handles    modlib.HandlerMap
	sharedConn rdkgrpc.SharedConn
	client     pb.ModuleServiceClient
	// robotClient supplements the ModuleServiceClient client to serve select robot level methods from the module server
	robotClient robotpb.RobotServiceClient
	addr        string
	resources   map[resource.Name]*addedResource
	// resourcesMu must be held if the `resources` field is accessed without
	// write-locking the module manager.
	resourcesMu sync.Mutex

	// pendingRemoval allows delaying module close until after resources within it are closed
	pendingRemoval bool

	logger logging.Logger
	ftdc   *ftdc.FTDC
}

// dial will Dial the module and replace the underlying connection (if it exists) in m.conn.
func (m *module) dial() error {
	// TODO(PRODUCT-343): session support probably means interceptors here
	var err error
	addrToDial := m.addr
	if !rutils.TCPRegex.MatchString(addrToDial) {
		addrToDial = "unix://" + addrToDial
	}
	conn, err := grpc.Dial( //nolint:staticcheck
		addrToDial,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(rpc.MaxMessageSize)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			rdkgrpc.EnsureTimeoutUnaryClientInterceptor,
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

	// Take the grpc over unix socket connection and add it to this `module`s `SharedConn`
	// object. This `m.sharedConn` object is referenced by all resources/components. `Client`
	// objects communicating with the module. If we're re-dialing after a restart, there may be
	// existing resource `Client`s objects. Rather than recreating clients with new information, we
	// choose to "swap out" the underlying connection object for those existing `Client`s.
	//
	// Resetting the `SharedConn` will also create a new WebRTC PeerConnection object. `dial`ing to
	// a module is followed by doing a `ReadyRequest` `ReadyResponse` exchange. If that exchange
	// contains a working WebRTC offer and answer, the PeerConnection will succeed in connecting. If
	// there is an error exchanging offers and answers, the PeerConnection object will be nil'ed
	// out.
	m.sharedConn.ResetConn(rpc.GrpcOverHTTPClientConn{ClientConn: conn}, m.logger)
	m.client = pb.NewModuleServiceClient(m.sharedConn.GrpcConn())
	m.robotClient = robotpb.NewRobotServiceClient(m.sharedConn.GrpcConn())
	return nil
}

// checkReady sends a `ReadyRequest` and waits for either a `ReadyResponse`, or a context
// cancelation.
func (m *module) checkReady(ctx context.Context, parentAddr string) error {
	ctxTimeout, cancelFunc := context.WithTimeout(ctx, rutils.GetModuleStartupTimeout(m.logger))
	defer cancelFunc()

	m.logger.CInfow(ctx, "Waiting for module to respond to ready request", "module", m.cfg.Name)

	req := &pb.ReadyRequest{ParentAddress: parentAddr}

	// Wait for gathering to complete. Pass the entire SDP as an offer to the `ReadyRequest`.
	var err error
	req.WebrtcOffer, err = m.sharedConn.GenerateEncodedOffer()
	if err != nil {
		m.logger.CWarnw(ctx, "Unable to generate offer for module PeerConnection. Ignoring.", "err", err)
	}

	for {
		// 5000 is an arbitrarily high number of attempts (context timeout should hit long before)
		resp, err := m.client.Ready(ctxTimeout, req, grpc_retry.WithMax(5000))
		if err != nil {
			return err
		}

		if !resp.Ready {
			// Module's can express that they are in a state:
			// - That is Capable of receiving and responding to gRPC commands
			// - But is "not ready" to take full responsibility of being a module
			//
			// Our behavior is to busy-poll until a module declares it is ready. But we otherwise do
			// not adjust timeouts based on this information.
			continue
		}

		err = m.sharedConn.ProcessEncodedAnswer(resp.WebrtcAnswer)
		if err != nil {
			m.logger.CWarnw(ctx, "Unable to create PeerConnection with module. Ignoring.", "err", err)
		}

		// The `ReadyRespones` also includes the Viam `API`s and `Model`s the module provides. This
		// will be used to construct "generic Client" objects that can execute gRPC commands for
		// methods that are not part of the viam-server's API proto.
		m.handles, err = modlib.NewHandlerMapFromProto(ctx, resp.Handlermap, m.sharedConn.GrpcConn())
		return err
	}
}

func (m *module) startProcess(
	ctx context.Context,
	parentAddr string,
	oue func(int) bool,
	viamHomeDir string,
	packagesDir string,
) error {
	var err error

	if rutils.ViamTCPSockets() {
		if addr, err := getAutomaticPort(); err != nil {
			return err
		} else { //nolint:revive
			m.addr = addr
		}
	} else {
		// append a random alpha string to the module name while creating a socket address to avoid conflicts
		// with old versions of the module.
		if m.addr, err = modlib.CreateSocketAddress(
			filepath.Dir(parentAddr), fmt.Sprintf("%s-%s", m.cfg.Name, utils.RandomAlphaString(5))); err != nil {
			return err
		}
		m.addr, err = cleanWindowsSocketPath(runtime.GOOS, m.addr)
		if err != nil {
			return err
		}
	}

	// We evaluate the Module's ExePath absolutely in the viam-server process so that
	// setting the CWD does not cause issues with relative process names
	absoluteExePath, err := m.cfg.EvaluateExePath(packages.LocalPackagesDir(packagesDir))
	if err != nil {
		return err
	}
	moduleEnvironment := m.getFullEnvironment(viamHomeDir)
	// Prefer VIAM_MODULE_ROOT as the current working directory if present but fallback to the directory of the exepath
	moduleWorkingDirectory, ok := moduleEnvironment["VIAM_MODULE_ROOT"]
	if !ok {
		moduleWorkingDirectory = filepath.Dir(absoluteExePath)
		m.logger.CDebugw(ctx, "VIAM_MODULE_ROOT was not passed to module. Defaulting to module's working directory",
			"module", m.cfg.Name, "dir", moduleWorkingDirectory)
	} else {
		m.logger.CInfow(ctx, "Starting module in working directory", "module", m.cfg.Name, "dir", moduleWorkingDirectory)
	}

	// Create STDOUT and STDERR loggers for the module and turn off log deduplication for
	// both. Module output through these loggers may contain data like stack traces, which
	// are repetitive but are not actually "noisy."
	stdoutLogger := m.logger.Sublogger("StdOut")
	stderrLogger := m.logger.Sublogger("StdErr")
	stdoutLogger.NeverDeduplicate()
	stderrLogger.NeverDeduplicate()

	pconf := pexec.ProcessConfig{
		ID:               m.cfg.Name,
		Name:             absoluteExePath,
		Args:             []string{m.addr},
		CWD:              moduleWorkingDirectory,
		Environment:      moduleEnvironment,
		Log:              true,
		OnUnexpectedExit: oue,
		StdOutLogger:     stdoutLogger,
		StdErrLogger:     stderrLogger,
	}
	// Start module process with supplied log level or "debug" if none is
	// supplied and module manager has a DebugLevel logger.
	if m.cfg.LogLevel != "" {
		pconf.Args = append(pconf.Args, fmt.Sprintf(logLevelArgumentTemplate, m.cfg.LogLevel))
	} else if m.logger.Level().Enabled(zapcore.DebugLevel) {
		pconf.Args = append(pconf.Args, fmt.Sprintf(logLevelArgumentTemplate, "debug"))
	}

	m.process = pexec.NewManagedProcess(pconf, m.logger)

	if err := m.process.Start(context.Background()); err != nil {
		return errors.WithMessage(err, "module startup failed")
	}

	// Turn on process cpu/memory diagnostics for the module process. If there's an error, we
	// continue normally, just without FTDC.
	m.registerProcessWithFTDC()

	checkTicker := time.NewTicker(100 * time.Millisecond)
	defer checkTicker.Stop()

	m.logger.CInfow(ctx, "Starting up module", "module", m.cfg.Name)
	rutils.LogViamEnvVariables("Starting module with following Viam environment variables", moduleEnvironment, m.logger)

	ctxTimeout, cancel := context.WithTimeout(ctx, rutils.GetModuleStartupTimeout(m.logger))
	defer cancel()
	for {
		select {
		case <-ctxTimeout.Done():
			if errors.Is(ctxTimeout.Err(), context.DeadlineExceeded) {
				return rutils.NewModuleStartUpTimeoutError(m.cfg.Name)
			}
			return ctxTimeout.Err()
		case <-checkTicker.C:
			if errors.Is(m.process.Status(), os.ErrProcessDone) {
				return fmt.Errorf(
					"module %s exited too quickly after attempted startup; it might have a fatal runtime issue",
					m.cfg.Name,
				)
			}
		}
		if !rutils.TCPRegex.MatchString(m.addr) {
			// note: we don't do this check in TCP mode because TCP addresses are not file paths and will fail check.
			err = modlib.CheckSocketOwner(m.addr)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return errors.WithMessage(err, "module startup failed")
			}
		}
		break
	}
	return nil
}

func (m *module) stopProcess() error {
	if m.process == nil {
		return nil
	}

	m.logger.Infof("Stopping module: %s process", m.cfg.Name)

	// Attempt to remove module's .sock file if module did not remove it
	// already.
	defer func() {
		rutils.RemoveFileNoError(m.addr)

		// The system metrics "statser" is resilient to the process dying under the hood. An empty set
		// of metrics will be reported. Therefore it is safe to continue monitoring the module process
		// while it's in shutdown.
		if m.ftdc != nil {
			m.ftdc.Remove(m.getFTDCName())
		}
	}()

	// TODO(RSDK-2551): stop ignoring exit status 143 once Python modules handle
	// SIGTERM correctly.
	// Also ignore if error is that the process no longer exists.
	if err := m.process.Stop(); err != nil {
		if strings.Contains(err.Error(), errMessageExitStatus143) || strings.Contains(err.Error(), "no such process") {
			return nil
		}
		return err
	}

	return nil
}

func (m *module) killProcessGroup() {
	if m.process == nil {
		return
	}
	m.logger.Infof("Killing module: %s process", m.cfg.Name)
	m.process.KillGroup()
}

func (m *module) registerResources(mgr modmaninterface.ModuleManager) {
	for api, models := range m.handles {
		if _, ok := resource.LookupGenericAPIRegistration(api.API); !ok {
			resource.RegisterAPI(
				api.API,
				resource.APIRegistration[resource.Resource]{ReflectRPCServiceDesc: api.Desc},
			)
		}

		switch {
		case api.API.IsComponent():
			for _, model := range models {
				m.logger.Infow("Registering component API and model from module", "module", m.cfg.Name, "API", api.API, "model", model)
				resource.RegisterComponent(api.API, model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
					Constructor: func(
						ctx context.Context,
						deps resource.Dependencies,
						conf resource.Config,
						logger logging.Logger,
					) (resource.Resource, error) {
						return mgr.AddResource(ctx, conf, DepsToNames(deps))
					},
				})
			}
		case api.API.IsService():
			for _, model := range models {
				m.logger.Infow("Registering service API and model from module", "module", m.cfg.Name, "API", api.API, "model", model)
				resource.RegisterService(api.API, model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
					Constructor: func(
						ctx context.Context,
						deps resource.Dependencies,
						conf resource.Config,
						logger logging.Logger,
					) (resource.Resource, error) {
						return mgr.AddResource(ctx, conf, DepsToNames(deps))
					},
				})
			}
		default:
			m.logger.Errorw("Invalid module type", "API type", api.API.Type)
		}
	}
}

func (m *module) deregisterResources() {
	for api, models := range m.handles {
		for _, model := range models {
			resource.Deregister(api.API, model)
		}
	}
	m.handles = nil
}

func (m *module) cleanupAfterStartupFailure() {
	if err := m.stopProcess(); err != nil {
		msg := "Error while stopping process of module that failed to start"
		m.logger.Errorw(msg, "module", m.cfg.Name, "error", err)
	}
	utils.UncheckedError(m.sharedConn.Close())
}

func (m *module) cleanupAfterCrash(mgr *Manager) {
	utils.UncheckedError(m.sharedConn.Close())
	mgr.rMap.Range(func(r resource.Name, mod *module) bool {
		if mod == m {
			mgr.rMap.Delete(r)
		}
		return true
	})
	mgr.modules.Delete(m.cfg.Name)
}

func (m *module) getFullEnvironment(viamHomeDir string) map[string]string {
	return getFullEnvironment(m.cfg, m.dataDir, viamHomeDir)
}

func (m *module) getFTDCName() string {
	return fmt.Sprintf("proc.modules.%s", m.process.ID())
}

func (m *module) registerProcessWithFTDC() {
	if m.ftdc == nil {
		return
	}

	pid, err := m.process.UnixPid()
	if err != nil {
		m.logger.Warnw("Module process has no pid. Cannot start ftdc.", "err", err)
		return
	}

	statser, err := sys.NewPidSysUsageStatser(pid)
	if err != nil {
		m.logger.Warnw("Cannot find /proc files", "err", err)
		return
	}

	m.ftdc.Add(m.getFTDCName(), statser)
}

// Return an address string with an auto-assigned port.
// This gets closed and then passed down to the module child process.
func getAutomaticPort() (string, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		return "", err
	}
	return addr, nil
}
