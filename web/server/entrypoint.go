// Package server implements the entry point for running a robot web server.
package server

import (
	"cmp"
	"context"
	"encoding/json"
	"net"
	"os"
	"path"
	"runtime/pprof"
	"slices"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/perf"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/stacktrace"
	nc "go.viam.com/rdk/web/networkcheck"
)

// Arguments for the command.
type Arguments struct {
	AllowInsecureCreds         bool   `flag:"allow-insecure-creds,usage=allow connections to send credentials over plaintext"`
	ConfigFile                 string `flag:"config,usage=machine configuration file"`
	CPUProfile                 string `flag:"cpuprofile,usage=write cpu profile to file"`
	Debug                      bool   `flag:"debug"`
	SharedDir                  string `flag:"shareddir,usage=web resource directory"`
	Version                    bool   `flag:"version,usage=print version"`
	WebProfile                 bool   `flag:"webprofile,usage=include profiler in http server"`
	WebRTC                     bool   `flag:"webrtc,default=true,usage=force webrtc connections instead of direct"`
	RevealSensitiveConfigDiffs bool   `flag:"reveal-sensitive-config-diffs,usage=show config diffs"`
	UntrustedEnv               bool   `flag:"untrusted-env,usage=disable processes and shell from running in a untrusted environment"`
	OutputTelemetry            bool   `flag:"output-telemetry,usage=print out metrics data"`
	DisableMulticastDNS        bool   `flag:"disable-mdns,usage=disable server discovery through multicast DNS"`
	DumpResourcesPath          string `flag:"dump-resources,usage=dump all resource registrations as json to the provided file path"`
	EnableFTDC                 bool   `flag:"ftdc,default=true,usage=enable fulltime data capture for diagnostics"`
	OutputLogFile              string `flag:"log-file,usage=write logs to a file with log rotation"`
	NoTLS                      bool   `flag:"no-tls,usage=starts an insecure http server without TLS certificates even if one exists"`
	NetworkCheckOnly           bool   `flag:"network-check,usage=only runs normal network checks, logs results, and exits"`
}

type robotServer struct {
	args                                       Arguments
	rootLogger, configLogger, networkingLogger logging.Logger
	registry                                   *logging.Registry
	conn                                       rpc.ClientConn
	signalingConn                              rpc.ClientConn
}

func logViamEnvVariables(logger logging.Logger) {
	var viamEnvVariables []interface{}
	if value, exists := os.LookupEnv("VIAM_MODULE_ROOT"); exists {
		viamEnvVariables = append(viamEnvVariables, "VIAM_MODULE_ROOT", value)
	}
	if value, exists := os.LookupEnv("VIAM_RESOURCE_CONFIGURATION_TIMEOUT"); exists {
		viamEnvVariables = append(viamEnvVariables, "VIAM_RESOURCE_CONFIGURATION_TIMEOUT", value)
	}
	if value, exists := os.LookupEnv("VIAM_MODULE_STARTUP_TIMEOUT"); exists {
		viamEnvVariables = append(viamEnvVariables, "VIAM_MODULE_STARTUP_TIMEOUT", value)
	}
	if value, exists := os.LookupEnv("VIAM_CONFIG_READ_TIMEOUT"); exists {
		viamEnvVariables = append(viamEnvVariables, "VIAM_CONFIG_READ_TIMEOUT", value)
	}
	if value, exists := os.LookupEnv("CWD"); exists {
		viamEnvVariables = append(viamEnvVariables, "CWD", value)
	}
	if rutils.PlatformHomeDir() != "" {
		viamEnvVariables = append(viamEnvVariables, "HOME", rutils.PlatformHomeDir())
	}
	// Always attempt to overwrite VIAM_HOME because we do not currently support user-defined home directories.
	value, alreadySet := os.LookupEnv(rutils.HomeEnvVar)
	err := os.Setenv(rutils.HomeEnvVar, rutils.ViamDotDir)
	// if we successfully overwrite VIAM_HOME, log
	if err == nil && alreadySet {
		logger.Infof("Environment variable %v was overwritten from %v to %v", rutils.HomeEnvVar, value, rutils.ViamDotDir)
	} else if err != nil && !alreadySet {
		logger.Infof("Unable to set %v environment variable, continuing with startup", rutils.HomeEnvVar)
	}
	if value, exists := os.LookupEnv(rutils.HomeEnvVar); exists {
		viamEnvVariables = append(viamEnvVariables, rutils.HomeEnvVar, value)
	}
	if len(viamEnvVariables) != 0 {
		logger.Infow("Starting viam-server with following environment variables", viamEnvVariables...)
	}
}

func logVersion(logger logging.Logger) {
	var versionFields []interface{}
	if config.Version != "" {
		versionFields = append(versionFields, "version", config.Version)
	}
	if config.GitRevision != "" {
		versionFields = append(versionFields, "git_rev", config.GitRevision)
	}
	if len(versionFields) != 0 {
		logger.Infow("viam-server", versionFields...)
	} else {
		logger.Info("viam-server built from source; version unknown")
	}
}

func logStartupInfo(logger logging.Logger) {
	logVersion(logger)
	logViamEnvVariables(logger)
}

// RunServer is an entry point to starting the web server that can be called by main in a code
// sample or otherwise be used to initialize the server.
func RunServer(ctx context.Context, args []string, _ logging.Logger) (err error) {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	ctx, err = rutils.WithTrustedEnvironment(ctx, !argsParsed.UntrustedEnv)
	if err != nil {
		return err
	}

	if argsParsed.DumpResourcesPath != "" {
		return dumpResourceRegistrations(argsParsed.DumpResourcesPath)
	}

	// The root logger has the name "rdk" and represents the root of the logger tree. We use
	// the root logger to attach appenders like the net appender that should be on all of
	// its Subloggers. Some logger names, like "rdk.networking", will be treated as
	// diagnostic in app and some, like "rdk.modmanager", will be treated as user-facing.
	// Most users will only care about startup/shutdown/reconfiguration/module-output, but
	// NetCode will often need to see more log output when diagnosing an issue. `app` has
	// special logic to only render user-facing logs by default in the "Logs" tab.
	rootLogger, registry := logging.NewBlankLoggerWithRegistry("rdk")
	// Dan: We changed from a constructor that defaulted to INFO to `NewBlankLoggerWithRegistry`
	// which defaults to DEBUG. We pessimistically set the level to INFO to ensure parity. Though I
	// expect `InitLoggingSettings` will always put the logger into the right state without any
	// observable side-effects.
	rootLogger.SetLevel(logging.INFO)

	configLogger := rootLogger.Sublogger("config")
	networkingLogger := rootLogger.Sublogger("networking")

	if argsParsed.OutputLogFile != "" {
		logWriter, closer := logging.NewFileAppender(argsParsed.OutputLogFile)
		defer func() {
			utils.UncheckedError(closer.Close())
		}()
		registry.AddAppenderToAll(logWriter)
	} else {
		registry.AddAppenderToAll(logging.NewStdoutAppender())
	}

	logging.RegisterEventLogger(rootLogger, "viam-server")
	config.InitLoggingSettings(rootLogger, configLogger, argsParsed.Debug)

	if argsParsed.Version {
		// log startup info here and return if version flag.
		logStartupInfo(rootLogger)
		return
	} else if argsParsed.NetworkCheckOnly {
		// Run network checks synchronously and immediately exit if `--network-check` flag was
		// used. Otherwise run network checks asynchronously.
		nc.RunNetworkChecks(ctx, rootLogger, false /* !continueRunningTestDNS */)
		return
	}

	// log startup info locally if server fails and exits while attempting to start up
	var startupInfoLogged bool
	defer func() {
		if !startupInfoLogged {
			rootLogger.CInfo(ctx, "error starting viam-server, logging version and exiting")
			logStartupInfo(rootLogger)
		}
	}()

	if argsParsed.ConfigFile == "" {
		rootLogger.Error("please specify a config file through the -config parameter.")
		return
	}

	if argsParsed.CPUProfile != "" {
		f, err := os.Create(argsParsed.CPUProfile)
		if err != nil {
			return err
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}

	var appConn, signalingConn rpc.ClientConn

	// Read the config from disk and use it to initialize the remote logger.
	cfgFromDisk, err := config.ReadLocalConfig(argsParsed.ConfigFile, configLogger)
	if err != nil {
		return err
	}

	if argsParsed.OutputTelemetry {
		// Only handle printing metrics. Trace span exporting is now handled in the
		// robot config.
		exporter := perf.NewDevelopmentExporterWithOptions(perf.DevelopmentExporterOptions{
			ReportingInterval: time.Second * 10,
			TracesDisabled:    true,
		})
		if err := exporter.Start(); err != nil {
			return err
		}
		defer exporter.Stop()
	}

	// the underlying connection in `appConn` can be nil. In this case, a background Goroutine is kicked off to reattempt dials in a
	// serialized manner
	if cfgFromDisk.Cloud != nil {
		cloud := cfgFromDisk.Cloud

		cloudCreds := cfgFromDisk.Cloud.GetCloudCredsDialOpt()
		appConnLogger := networkingLogger.Sublogger("app_connection")
		appConn, err = grpc.NewAppConn(ctx, cloud.AppAddress, cloud.ID, cloudCreds, appConnLogger)
		if err != nil {
			return err
		}
		defer utils.UncheckedErrorFunc(appConn.Close)

		// if SignalingAddress is specified and different from AppAddress, create a new connection to it. Otherwise reuse appConn.
		if cloud.SignalingAddress != "" && cloud.SignalingAddress != cloud.AppAddress {
			signalingConnLogger := networkingLogger.Sublogger("signaling_connection")
			signalingConn, err = grpc.NewAppConn(
				ctx, cloud.SignalingAddress, cloud.ID, cloudCreds, signalingConnLogger)
			if err != nil {
				return err
			}
			defer utils.UncheckedErrorFunc(signalingConn.Close)
		} else {
			signalingConn = appConn
		}

		// Start remote logging with config from disk.
		// This is to ensure we make our best effort to write logs for failures loading the remote config.
		if cloud.AppAddress != "" {
			netAppender, err := logging.NewNetAppender(
				&logging.CloudConfig{
					AppAddress: cloud.AppAddress,
					ID:         cloud.ID,
					CloudCred:  cloudCreds,
				},
				appConn, false, logging.NewLogger("NetAppender-loggerWithoutNet"),
			)
			if err != nil {
				return err
			}
			defer netAppender.Close()

			registry.AddAppenderToAll(netAppender)
		}
	}
	// log startup info and run network checks after netlogger is initialized so it's captured in cloud machine logs.
	logStartupInfo(rootLogger)
	startupInfoLogged = true

	// The golog global logger is unused in rdk. But, goutils still makes infrequent use of
	// it for a hodgepodge of messages. Use the rootLogger ("rdk") here, as those goutils
	// messages are not of one specific category and are not noisy enough to be put under
	// a diagnostic logger.
	golog.ReplaceGloabl(rootLogger.AsZap())

	// RunNetworkChecks will create a (diagnostic) "rdk.network-checks" Sublogger.
	go nc.RunNetworkChecks(ctx, rootLogger, true /* continueRunningTestDNS */)

	server := robotServer{
		rootLogger:       rootLogger,
		configLogger:     configLogger,
		networkingLogger: networkingLogger,
		args:             argsParsed,
		registry:         registry,
		conn:             appConn,
		signalingConn:    signalingConn,
	}

	// Run the server with remote logging enabled.
	err = server.runServer(ctx)
	if err != nil {
		rootLogger.Error("Fatal error running server, exiting now:", err)
	}

	return err
}

// runServer is an entry point to starting the web server after the local config is read. Once the local config
// is read the logger may be initialized to remote log. This ensure we capture errors starting up the server and report to the cloud.
func (s *robotServer) runServer(ctx context.Context) error {
	if s.conn != nil {
		s.configLogger.CInfo(ctx, "Getting up-to-date config from cloud...")
	}
	// config.Read will add a timeout using contextutils.GetTimeoutCtx, so no need to add a separate timeout.
	cfg, err := config.Read(ctx, s.args.ConfigFile, s.configLogger, s.conn)
	if err != nil {
		return err
	}
	config.UpdateFileConfigDebug(cfg.Debug)

	err = s.serveWeb(ctx, cfg)
	if err != nil {
		s.rootLogger.Errorw("error serving web", "error", err)
	}

	return err
}

func (s *robotServer) createWebOptions(cfg *config.Config) (weboptions.Options, error) {
	options, err := weboptions.FromConfig(cfg)
	if err != nil {
		return weboptions.Options{}, err
	}
	options.Pprof = s.args.WebProfile || cfg.EnableWebProfile
	options.SharedDir = s.args.SharedDir
	options.Debug = s.args.Debug || cfg.Debug
	options.PreferWebRTC = s.args.WebRTC
	options.DisableMulticastDNS = s.args.DisableMulticastDNS
	options.NoTLS = s.args.NoTLS || cfg.Network.NoTLS
	if cfg.Cloud != nil && s.args.AllowInsecureCreds {
		options.SignalingDialOpts = append(options.SignalingDialOpts, rpc.WithAllowInsecureWithCredentialsDowngrade())
	}
	// options.SignalingAddress is set in config processing
	// signalingConn is a separate connection if SignalingAddress and AppAddress differ, otherwise it points to s.conn
	options.SignalingDialOpts = append(options.SignalingDialOpts, rpc.WithSignalingConn(s.signalingConn))

	if len(options.Auth.Handlers) == 0 {
		host, _, err := net.SplitHostPort(cfg.Network.BindAddress)
		if err != nil {
			return weboptions.Options{}, err
		}
		if host == "" || host == "0.0.0.0" || host == "::" {
			// Use rootLogger instead of networkingLogger here to be user-facing. This log has
			// important security implications that users have control over.
			s.rootLogger.Warn("binding to all interfaces without authentication")
		}
	}
	return options, nil
}

// A wrapper around actual config processing that also applies options from the
// robot server.
func (s *robotServer) processConfig(in *config.Config) (*config.Config, error) {
	out, err := config.ProcessConfig(in)
	if err != nil {
		return nil, err
	}
	out.Debug = s.args.Debug || in.Debug
	out.EnableWebProfile = s.args.WebProfile || in.EnableWebProfile
	out.FromCommand = true
	out.AllowInsecureCreds = s.args.AllowInsecureCreds
	out.UntrustedEnv = s.args.UntrustedEnv

	// Use ~/.viam/packages for package path if one was not specified.
	if in.PackagePath == "" {
		out.PackagePath = path.Join(rutils.ViamDotDir, "packages")
	}

	return out, nil
}

// A function to be started as a goroutine that watches for changes, either
// from disk or from cloud, to the robot's config. Starts comparisons based on
// `currCfg`. Reconfigures the robot when config changes are received from the
// watcher.
func (s *robotServer) configWatcher(ctx context.Context, currCfg *config.Config, r robot.LocalRobot,
	watcher config.Watcher,
) {
	// Reconfigure robot to have passed-in config before listening for any config
	// changes.
	startTime := time.Now()
	r.Reconfigure(ctx, currCfg)
	s.configLogger.CInfow(ctx, "Robot constructed with full config", "time_to_construct", time.Since(startTime).String())
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		select {
		case <-ctx.Done():
			return
		case cfg := <-watcher.Config():
			processedConfig, err := s.processConfig(cfg)
			if err != nil {
				s.configLogger.Errorw("reconfiguration aborted: error processing config", "error", err)
				continue
			}

			// Special case: the incoming config specifies the default BindAddress, but the current one in use is non-default.
			// Don't override the non-default BindAddress with the default one.
			// If this is the only difference, the next step, diff.NetworkEqual will be true.
			if processedConfig.Network.BindAddressDefaultSet && !currCfg.Network.BindAddressDefaultSet {
				processedConfig.Network.BindAddress = currCfg.Network.BindAddress
				processedConfig.Network.BindAddressDefaultSet = false
			}

			// flag to restart web service if necessary
			diff, err := config.DiffConfigs(*currCfg, *processedConfig, s.args.RevealSensitiveConfigDiffs)
			if err != nil {
				s.configLogger.Errorw("reconfiguration aborted: error diffing config", "error", err)
				continue
			}
			var options weboptions.Options

			if !diff.NetworkEqual {
				// TODO(RSDK-2694): use internal web service reconfiguration instead
				s.rootLogger.Info("network/auth config change detected, restarting web service")
				r.StopWeb()
				options, err = s.createWebOptions(processedConfig)
				if err != nil {
					s.configLogger.Errorw("reconfiguration aborted: error creating weboptions", "error", err)
					continue
				}
			}

			if currCfg.Network.BindAddress != processedConfig.Network.BindAddress {
				s.configLogger.Infof("Config watcher detected bind address change: updating %v -> %v",
					currCfg.Network.BindAddress,
					processedConfig.Network.BindAddress)
			}

			// Update logger registry if log patterns may have changed.
			//
			// This functionality is tested in `TestLogPropagation` in `local_robot_test.go`.
			if !diff.LogEqual || !diff.ResourcesEqual {
				// Only display the warning when the user attempted to change the `log` field of
				// the config.
				if !diff.LogEqual {
					s.configLogger.Debug("Detected potential changes to log patterns; updating logger levels")
					s.configLogger.Warn(
						"Changes to 'log' field may not affect modular logs. " +
							"Use 'log_level' in module config or 'log_configuration' in resource config instead",
					)
				}
				config.UpdateLoggerRegistryFromConfig(s.registry, processedConfig, s.rootLogger)
			}

			r.Reconfigure(ctx, processedConfig)

			if !diff.NetworkEqual {
				if err := r.StartWeb(ctx, options); err != nil {
					s.configLogger.Errorw("reconfiguration failed: error starting web service while reconfiguring", "error", err)
				}
				s.configLogger.Info("web service restart finished")
			}
			currCfg = processedConfig
		}
	}
}

func (s *robotServer) serveWeb(ctx context.Context, cfg *config.Config) (err error) {
	ctx, cancel := context.WithCancel(ctx)

	hungShutdownDeadline := 60 * time.Second
	// "rdk.stack_traces" is listed as a diagnostic logger in app; users will not see
	// viam-server stack traces by default on app.viam.com.
	stackTraceLogger := s.rootLogger.Sublogger("stack_traces")
	slowWatcher, slowWatcherCancel := utils.SlowGoroutineWatcherAfterContext(
		ctx, hungShutdownDeadline, "server is taking a while to shutdown", stackTraceLogger)

	// Set up SIGUSR1 handler to dump stack traces when the agent requests it before restart.
	stackTraceHandler, cleanupSignalHandler := stacktrace.NewSignalHandler(s.rootLogger)
	defer cleanupSignalHandler()

	doneServing := make(chan struct{})

	forceShutdown := make(chan struct{})
	defer func() { <-forceShutdown }()

	var (
		theRobot                  robot.LocalRobot
		theRobotLock              sync.Mutex
		cloudRestartCheckerActive chan struct{}
	)
	rpcDialer := rpc.NewCachedDialer()
	defer func() {
		if cloudRestartCheckerActive != nil {
			<-cloudRestartCheckerActive
		}
		err = multierr.Combine(err, rpcDialer.Close())
	}()
	defer cancel()
	ctx = rpc.ContextWithDialer(ctx, rpcDialer)

	utils.PanicCapturingGo(func() {
		defer close(forceShutdown)

		<-ctx.Done()
		shutdownStarted := time.Now()

		slowTicker := time.NewTicker(10 * time.Second)
		defer slowTicker.Stop()

		checkDone := func() bool {
			select {
			case <-slowWatcher:
				select {
				// the successful shutdown case has us close(doneServing), followed by slowWatcherCancel,
				// meaning both may be selected so we check to see if doneServing was also closed. If the
				// deadline truly elapses, there's a chance we shutdown cleanly at the exact same time which may
				// result in not catching this case.
				case <-doneServing:
					return true
				default:
					theRobotLock.Lock()
					robot := theRobot
					theRobotLock.Unlock()
					if robot != nil {
						robot.Kill()
					}
					s.rootLogger.Fatalw("server failed to cleanly shutdown after deadline", "deadline", hungShutdownDeadline)
					return true
				}
			case <-doneServing:
				return true
			default:
				return false
			}
		}

		for {
			select {
			case <-slowWatcher:
				if checkDone() {
					return
				}
			case <-doneServing:
				return
			case <-slowTicker.C:
				if checkDone() {
					return
				}
				s.rootLogger.Warnw("Waiting for clean shutdown", "time_elapsed",
					time.Since(shutdownStarted).String())
			}
		}
	})

	defer func() {
		close(doneServing)
		slowWatcherCancel()
		<-slowWatcher
	}()
	s.configLogger.CInfo(ctx, "Processing initial robot config...")
	fullProcessedConfig, err := s.processConfig(cfg)
	if err != nil {
		return err
	}

	// Update logger registry as soon as we have fully processed config. Further
	// updates to the registry will be handled by the config watcher goroutine.
	//
	// This functionality is tested in `TestLogPropagation` in `local_robot_test.go`.
	config.UpdateLoggerRegistryFromConfig(s.registry, fullProcessedConfig, s.configLogger)

	// Only start cloud restart checker if cloud config is non-nil, and viam-agent is not
	// handling restart checking for us (relevant environment variable is unset).
	if fullProcessedConfig.Cloud != nil && os.Getenv(rutils.ViamAgentHandlesNeedsRestartChecking) == "" {
		s.rootLogger.CInfo(ctx, "Agent does not handle checking needs restart functionality; will handle in server")
		cloudRestartCheckerActive = make(chan struct{})
		utils.PanicCapturingGo(func() {
			defer close(cloudRestartCheckerActive)
			restartCheck := newRestartChecker(cfg.Cloud, s.rootLogger, s.conn)
			restartInterval := defaultNeedsRestartCheckInterval

			for {
				if !utils.SelectContextOrWait(ctx, restartInterval) {
					return
				}

				mustRestart, newRestartInterval, err := restartCheck.needsRestart(ctx)
				if err != nil {
					s.networkingLogger.Infow("failed to check restart", "error", err)
					continue
				}

				restartInterval = newRestartInterval

				if mustRestart {
					stacktrace.LogStackTraceAndCancel(cancel, s.rootLogger)
				}
			}
		})
	}

	robotOptions := createRobotOptions()
	if s.args.RevealSensitiveConfigDiffs {
		robotOptions = append(robotOptions, robotimpl.WithRevealSensitiveConfigDiffs())
	}

	shutdownCallbackOpt := robotimpl.WithShutdownCallback(func() {
		stacktrace.LogStackTraceAndCancel(cancel, s.rootLogger)
	})
	robotOptions = append(robotOptions, shutdownCallbackOpt)

	if s.args.EnableFTDC {
		robotOptions = append(robotOptions, robotimpl.WithFTDC())
	}

	// Create `minimalProcessedConfig`, a copy of `fullProcessedConfig`. Remove
	// all components, services, remotes, modules, processes, packages, and jobs from
	// `minimalProcessedConfig`. Create new robot with `minimalProcessedConfig`
	// and immediately start web service. We need the machine to be reachable
	// through the web service ASAP, even if some resources take a long time to
	// initially configure.
	minimalProcessedConfig := *fullProcessedConfig
	minimalProcessedConfig.Components = nil
	minimalProcessedConfig.Services = nil
	minimalProcessedConfig.Remotes = nil
	minimalProcessedConfig.Modules = nil
	minimalProcessedConfig.Processes = nil
	minimalProcessedConfig.Packages = nil
	minimalProcessedConfig.Jobs = nil

	// Mark minimalProcessedConfig as an initial config, so robot reports a
	// state of initializing until reconfigured with full config.
	minimalProcessedConfig.Initial = true

	startTime := time.Now()
	myRobot, err := robotimpl.New(ctx, &minimalProcessedConfig, s.conn, s.rootLogger, robotOptions...)
	if err != nil {
		cancel()
		return err
	}
	s.configLogger.CInfow(ctx, "Robot created with minimal config", "time_to_create", time.Since(startTime).String())

	theRobotLock.Lock()
	theRobot = myRobot
	theRobotLock.Unlock()
	defer func() {
		err = multierr.Combine(err, theRobot.Close(context.Background()))
	}()

	// Register callback to forward SIGUSR1 to module processes for stack trace dumps, and
	// flush logs to cloud.
	stackTraceHandler.SetCallback(func() {
		theRobotLock.Lock()
		r := theRobot
		theRobotLock.Unlock()
		if r != nil {
			r.RequestModuleStackTraceDump()
		}
	})

	// watch for and deliver changes to the robot
	watcher, err := config.NewWatcher(ctx, cfg, s.configLogger, s.conn)
	if err != nil {
		cancel()
		return err
	}
	defer func() {
		err = multierr.Combine(err, watcher.Close())
	}()

	onWatchDone := make(chan struct{})
	go func() {
		defer close(onWatchDone)

		// Use `fullProcessedConfig` as the initial config for the config watcher
		// goroutine, as we want incoming config changes to be compared to the full
		// config.
		s.configWatcher(ctx, fullProcessedConfig, theRobot, watcher)
	}()
	// At end of this function, cancel context and wait for watcher goroutine
	// to complete.
	defer func() {
		cancel()
		<-onWatchDone
	}()
	s.configLogger.CInfo(ctx, "Config watcher started")

	// Create initial web options with `minimalProcessedConfig`.
	options, err := s.createWebOptions(&minimalProcessedConfig)
	if err != nil {
		return err
	}
	return web.RunWeb(ctx, theRobot, options, s.rootLogger)
}

// dumpResourceRegistrations prints all builtin resource registrations as a json array
// to the provided file. If you edit this function, ensure that etc/system_manifest/main.go is
// updated correspondingly.
func dumpResourceRegistrations(outputPath string) error {
	type resourceRegistration struct {
		API   string `json:"api"`
		Model string `json:"model"`
		// AttributeSchema is a serialization of the Go resource "Config" structures that components and services Reconfigure with.
		// Notably this includes the JSON tags that are used to parse these resource configs from the robot's JSON config.
		AttributeSchema *jsonschema.Schema `json:"attribute_schema,omitempty"`
	}

	// create the array of all resource registrations
	resources := make([]resourceRegistration, 0, len(resource.RegisteredResources()))
	for apimodel, reg := range resource.RegisteredResources() {
		var attributeSchema *jsonschema.Schema
		reflectType := reg.ConfigReflectType()
		if reflectType != nil {
			attributeSchema = jsonschema.ReflectFromType(reflectType)
		}
		resources = append(resources, resourceRegistration{
			API:             apimodel.API.String(),
			Model:           apimodel.Model.String(),
			AttributeSchema: attributeSchema,
		})
	}

	// sort the list alphabetically by API+Model
	slices.SortFunc(resources, func(a, b resourceRegistration) int {
		if a.API != b.API {
			return cmp.Compare(a.API, b.API)
		}
		return cmp.Compare(a.Model, b.Model)
	})

	// marshall and print the registrations to the provided file
	jsonResult, err := json.MarshalIndent(resources, "", "\t")
	if err != nil {
		return errors.Wrap(err, "unable to marshall resources")
	}

	if err := os.WriteFile(outputPath, jsonResult, 0o600); err != nil {
		return errors.Wrap(err, "unable to write resulting object to stdout")
	}
	return nil
}
