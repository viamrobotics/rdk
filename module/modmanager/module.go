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
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap/zapcore"
	pb "go.viam.com/api/module/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/ftdc"
	"go.viam.com/rdk/ftdc/sys"
	rdkgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	modlib "go.viam.com/rdk/module"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/packages"
	rutils "go.viam.com/rdk/utils"
)

type module struct {
	cfg     config.Module
	dataDir string
	process pexec.ManagedProcess
	// prevProcess, if not nil, will contain the previously running process. If the current process came from a restart by an
	// OnUnexpectedExitHandler, prevProcess will be the process that launched the OUE. If the process was started through
	// stopProcess and startProcess, it will simply be the previous (stopped) process.
	// This is used with wait to enforce clean shutdown without goroutine leaks.
	prevProcess pexec.ManagedProcess
	handles     modlib.HandlerMap
	sharedConn  rdkgrpc.SharedConn
	client      pb.ModuleServiceClient
	// robotClient supplements the ModuleServiceClient client to serve select robot level methods from the module server
	robotClient robotpb.RobotServiceClient
	addr        string
	resources   map[resource.Name]*addedResource
	// resourcesMu must be held if the `resources` field is accessed without
	// write-locking the module manager.
	resourcesMu sync.Mutex

	// pendingRemoval allows delaying module close until after resources within it are closed
	pendingRemoval bool
	restartCancel  context.CancelFunc

	logger logging.Logger
	ftdc   *ftdc.FTDC
}

// dial will Dial the module and replace the underlying connection (if it exists) in m.conn.
func (m *module) dial() error {
	// TODO(PRODUCT-343): session support probably means interceptors here
	var err error
	addrToDial := m.addr
	if !rutils.TCPRegex.MatchString(addrToDial) {
		addrToDial = "unix:" + addrToDial
	}

	otelStatsHandler := otelgrpc.NewClientHandler(
		otelgrpc.WithTracerProvider(trace.GetProvider()),
		otelgrpc.WithPropagators(propagation.TraceContext{}),
	)

	//nolint:staticcheck
	conn, err := grpc.Dial(
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
		grpc.WithStatsHandler(otelStatsHandler),
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
	parentCtxTimeout, parentCtxCancelFunc := context.WithTimeout(ctx, rutils.GetModuleStartupTimeout(m.logger))
	defer parentCtxCancelFunc()

	m.logger.CInfow(ctx, "Waiting for module to respond to ready request", "module", m.cfg.Name)

	req := &pb.ReadyRequest{ParentAddress: parentAddr}

	// Wait for gathering to complete. Pass the entire SDP as an offer to the `ReadyRequest`.
	var err error
	req.WebrtcOffer, err = m.sharedConn.GenerateEncodedOffer()
	if err != nil {
		m.logger.CWarnw(ctx, "Unable to generate offer for module PeerConnection. Ignoring.", "err", err)
	}

	for {
		var resp *pb.ReadyResponse
		// 5000 is an arbitrarily high number of attempts (context timeout should hit long before)
		for range 5000 {
			perCallCtx, perCallCtxCancelFunc := context.WithTimeout(parentCtxTimeout, 3*time.Second)
			resp, err = m.client.Ready(perCallCtx, req)
			perCallCtxCancelFunc()

			// if module is not ready yet, wait and try again
			code := status.Code(err)
			// context errors here are the perCallCtx
			if code == codes.Unavailable || code == codes.ResourceExhausted || code == codes.DeadlineExceeded || code == codes.Canceled {
				waitTimer := time.NewTimer(200 * time.Millisecond)
				select {
				case <-parentCtxTimeout.Done():
					waitTimer.Stop()
					return parentCtxTimeout.Err()
				case <-waitTimer.C:
				}
				// Short circuit this check if the process has already exited. We could get here if process exits before
				// module can set up the Ready server.
				// This is most likely in TCP mode, where the .sock presence check is skipped, but
				// also possible in UNIX mode if the process exits in between.
				// (OUE is waiting on the same modmanager lock, so it can't try to restart).
				if errors.Is(m.process.Status(), os.ErrProcessDone) {
					m.logger.Debug("Module process exited unexpectedly while waiting for ready.")
					parentCtxCancelFunc()
					return errors.New("module process exited unexpectedly")
				}
				continue
			} else if err != nil {
				return err
			}
			break
		}
		if resp == nil {
			// should not get here unless Module Startup Timeout has been overridden to very large value
			// and there is still no connection after 5000 retries
			parentCtxCancelFunc()
			return parentCtxTimeout.Err()
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

// returns true if this module should be run in TCP mode.
// (based on either global setting or per-module setting).
func (m *module) tcpMode() bool {
	return rutils.ViamTCPSockets() || m.cfg.TCPMode
}

// returns true if this module is running in TCP mode.
func (m *module) isRunningInTCPMode() bool {
	// addr is ip:port in TCP mode, or a path to .sock in unix socket mode.
	return rutils.TCPRegex.MatchString(m.addr)
}

func (m *module) startProcess(
	ctx context.Context,
	parentAddr string,
	oue pexec.UnexpectedExitHandler,
	viamHomeDir string,
	packagesDir string,
) error {
	var err error

	tcpMode := m.tcpMode()
	if tcpMode {
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
		m.addr, err = rutils.CleanWindowsSocketPath(runtime.GOOS, m.addr)
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
	moduleEnvironment := m.getFullEnvironment(viamHomeDir, packagesDir)
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
	// the latter. Module output through STDERR in particular may contain data like stack
	// traces from Golang and Python, which are repetitive but are not actually "noisy."
	stdoutLogger := m.logger.Sublogger("StdOut")
	stderrLogger := m.logger.Sublogger("StdErr")
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

	if tcpMode {
		pconf.Args = append(pconf.Args, "--tcp-mode")
	}

	m.prevProcess = m.process
	m.process = pexec.NewManagedProcess(pconf, m.logger)

	if err := m.process.Start(context.Background()); err != nil {
		return errors.WithMessage(err, "module startup failed")
	}

	// Turn on process cpu/memory diagnostics for the module process. If there's an error, we
	// continue normally, just without FTDC.
	m.registerProcessWithFTDC()

	checkTicker := time.NewTicker(100 * time.Millisecond)
	defer checkTicker.Stop()

	m.logger.CInfow(ctx, "Starting up module", "module", m.cfg.Name, "tcp_mode", tcpMode)
	rutils.LogViamEnvVariables("Starting module with following Viam environment variables", moduleEnvironment, m.logger)

	ctxTimeout, cancel := context.WithTimeout(ctx, rutils.GetModuleStartupTimeout(m.logger))
	defer cancel()
	for {
		select {
		case <-ctxTimeout.Done():
			if errors.Is(ctxTimeout.Err(), context.DeadlineExceeded) {
				return rutils.NewModuleStartUpTimeoutError(m.cfg.Name, m.logger)
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
		if !m.isRunningInTCPMode() {
			// Ensure that socket file has been created by the module. We don't do this check in
			// TCP mode because TCP addresses are not file paths and will fail check.
			_, err = os.Stat(m.addr)
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

	// Make sure the restart handler won't try to keep the process alive.
	if m.restartCancel != nil {
		m.restartCancel()
	}

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
		var processNotExistsErr *pexec.ProcessNotExistsError
		if strings.Contains(err.Error(), errMessageExitStatus143) || errors.As(err, &processNotExistsErr) {
			return nil
		}
		return err
	}

	return nil
}

// wait waits on a module's managedProcesses' goroutines to finish. stopProcess should be called before calling this,
// so it can call restartCancel to cancel the OUE.
// Can be used in testing to ensure that all of a module's associated goroutines complete before the test completes.
func (m *module) wait() {
	// if we are stuck in a restart loop, the OUE attempting the restarts is here
	if m.prevProcess != nil {
		m.prevProcess.Wait()
	}
	if m.process != nil {
		m.process.Wait()
	}
}

func (m *module) killProcessGroup() {
	if m.process == nil {
		return
	}
	m.logger.Infof("Killing module: %s process", m.cfg.Name)
	m.process.KillGroup()
}

func (m *module) registerResourceModels(mgr *Manager) {
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

func (m *module) deregisterResourceModels() {
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
	m.deregisterResourceModels()
	if err := m.sharedConn.Close(); err != nil {
		m.logger.Warnw("Error closing connection to crashed module", "error", err)
	}
	rutils.RemoveFileNoError(m.addr)
	if mgr.ftdc != nil {
		mgr.ftdc.Remove(m.getFTDCName())
	}
}

func (m *module) getFullEnvironment(viamHomeDir, packagesDir string) map[string]string {
	return getFullEnvironment(m.cfg, packagesDir, m.dataDir, viamHomeDir)
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

	statser, err := sys.NewSysUsageStatser(pid)
	if err != nil {
		m.logger.Warnw("Cannot start a system statser for module with pid", "pid", pid, "err", err)
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
